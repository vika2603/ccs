//go:build darwin

package creds

import (
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
)

func keychainAccount() string {
	u, _ := user.Current()
	return u.Username
}

func keychainEntryExists(service, account string) bool {
	err := exec.Command("/usr/bin/security", "find-generic-password",
		"-s", service, "-a", account).Run()
	return err == nil
}

func TestKeychainStoreRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "profile")
	s := NewKeychainStore()
	payload := []byte(`{"token":"abc"}`)

	defaultPath := filepath.Join(t.TempDir(), ".claude")
	service, _ := ServiceName(dir, defaultPath)
	account := keychainAccount()
	t.Cleanup(func() {
		exec.Command("/usr/bin/security", "delete-generic-password",
			"-s", service, "-a", account).Run()
	})

	if err := s.Write(dir, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !keychainEntryExists(service, account) {
		t.Fatalf("entry not present after write")
	}
	got, err := s.Read(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("got %q", got)
	}
	if err := s.Delete(dir); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if keychainEntryExists(service, account) {
		t.Errorf("entry should be gone")
	}
}

func TestKeychainReadNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nope")
	if _, err := NewKeychainStore().Read(dir); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
