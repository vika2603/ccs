package profile

import (
	"os"
	"testing"
)

func TestRemoveDeletesProfileDir(t *testing.T) {
	m, p := setup(t)
	m.Init()
	m.New("work", false)
	if err := m.Remove("work"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(p.ProfilePath("work")); !os.IsNotExist(err) {
		t.Errorf("expected gone, got %v", err)
	}
}

func TestRemoveMissingIsError(t *testing.T) {
	m, _ := setup(t)
	m.Init()
	if err := m.Remove("nope"); err == nil {
		t.Error("expected error")
	}
}

type fakeStore struct {
	deleted map[string]bool
}

func (f *fakeStore) Read(p string) ([]byte, error)  { return nil, nil }
func (f *fakeStore) Write(p string, b []byte) error { return nil }
func (f *fakeStore) Exists(p string) (bool, error)  { return true, nil }
func (f *fakeStore) Delete(p string) error {
	if f.deleted == nil {
		f.deleted = map[string]bool{}
	}
	f.deleted[p] = true
	return nil
}

func TestRemoveAlwaysDeletesCredentials(t *testing.T) {
	m, p := setup(t)
	m.Init()
	m.New("work", false)
	f := &fakeStore{}
	m2 := m.WithCreds(f)
	if err := m2.Remove("work"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !f.deleted[p.ProfilePath("work")] {
		t.Error("expected creds.Delete called")
	}
}
