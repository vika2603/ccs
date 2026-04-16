//go:build linux

package creds

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewFileStore()
	payload := []byte(`{"token":"abc"}`)
	if err := s.Write(dir, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := s.Read(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("got %q", got)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".credentials.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if string(b) != string(payload) {
		t.Errorf("file content mismatch")
	}
	exists, _ := s.Exists(dir)
	if !exists {
		t.Errorf("exists should be true")
	}
	if err := s.Delete(dir); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Read(dir); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
