package profile

import (
	"os"
	"testing"
)

func TestRenameMovesDir(t *testing.T) {
	m, p := setup(t)
	m.Init()
	m.New("work")
	if err := m.Rename("work", "office"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := os.Stat(p.ProfilePath("work")); !os.IsNotExist(err) {
		t.Errorf("old dir still exists")
	}
	if _, err := os.Stat(p.ProfilePath("office")); err != nil {
		t.Errorf("new dir missing: %v", err)
	}
}

func TestRenameFailsOnConflict(t *testing.T) {
	m, _ := setup(t)
	m.Init()
	m.New("a")
	m.New("b")
	if err := m.Rename("a", "b"); err == nil {
		t.Error("expected conflict error")
	}
}

type migrateFake struct {
	from, to string
}

func (f *migrateFake) Read(p string) ([]byte, error) {
	if f.from == "" {
		f.from = p
	}
	return []byte("data"), nil
}
func (f *migrateFake) Write(p string, b []byte) error { f.to = p; return nil }
func (f *migrateFake) Delete(p string) error          { return nil }
func (f *migrateFake) Exists(p string) (bool, error)  { return true, nil }

func TestRenameMigratesCreds(t *testing.T) {
	m, p := setup(t)
	m.Init()
	m.New("work")
	f := &migrateFake{}
	m2 := m.WithCreds(f)
	if err := m2.Rename("work", "office"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if f.from != p.ProfilePath("work") || f.to != p.ProfilePath("office") {
		t.Errorf("migrate called with %q -> %q", f.from, f.to)
	}
}
