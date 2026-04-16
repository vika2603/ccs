package creds

import (
	"path/filepath"
	"testing"
)

func TestMigrateMissingIsNoop(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	defaultPath := filepath.Join(tmp, ".claude")
	s := New()
	if err := Migrate(s, a, b, defaultPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

type migrateStoreFake struct {
	reads   map[string][]byte
	writes  map[string][]byte
	deleted []string
}

func (f *migrateStoreFake) Read(profile string) ([]byte, error) {
	if b, ok := f.reads[profile]; ok {
		return b, nil
	}
	if b, ok := f.writes[profile]; ok {
		return b, nil
	}
	return nil, ErrNotFound
}

func (f *migrateStoreFake) Write(profile string, data []byte) error {
	if f.writes == nil {
		f.writes = map[string][]byte{}
	}
	f.writes[profile] = append([]byte(nil), data...)
	return nil
}

func (f *migrateStoreFake) Delete(profile string) error {
	f.deleted = append(f.deleted, profile)
	return nil
}

func (f *migrateStoreFake) Exists(profile string) (bool, error) { return true, nil }

type migrateReadMismatchFake struct{ migrateStoreFake }

func (f *migrateReadMismatchFake) Read(profile string) ([]byte, error) {
	if _, ok := f.writes[profile]; ok {
		return nil, nil
	}
	return f.migrateStoreFake.Read(profile)
}

func TestMigrateVerifiesReadableBeforeDelete(t *testing.T) {
	oldProfile := filepath.Join(t.TempDir(), "old")
	newProfile := filepath.Join(t.TempDir(), "new")
	s := &migrateReadMismatchFake{
		migrateStoreFake: migrateStoreFake{
			reads: map[string][]byte{oldProfile: []byte("data")},
		},
	}
	err := Migrate(s, oldProfile, newProfile, filepath.Join(t.TempDir(), ".claude"))
	if err == nil {
		t.Fatalf("expected verification error")
	}
	if len(s.deleted) != 0 {
		t.Fatalf("old entry should still be present")
	}
}
