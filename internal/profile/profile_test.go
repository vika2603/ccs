package profile

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
)

func setup(t *testing.T) (Manager, layout.Paths) {
	t.Helper()
	home := t.TempDir()
	p := layout.New(home)
	os.MkdirAll(p.Root(), 0o755)
	reg := fields.NewRegistry(config.Default().Fields)
	return NewManager(p, reg), p
}

func TestNewCreatesProfileAndSymlinks(t *testing.T) {
	m, p := setup(t)
	if err := m.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := m.New("work"); err != nil {
		t.Fatalf("new: %v", err)
	}
	for _, f := range []string{"skills", "commands", "CLAUDE.md"} {
		path := filepath.Join(p.ProfilePath("work"), f)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("lstat %s: %v", f, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s: expected symlink", f)
		}
	}
	sharedInfo, err := os.Lstat(filepath.Join(p.SharedDir(), "CLAUDE.md"))
	if err != nil {
		t.Fatalf("lstat shared/CLAUDE.md: %v", err)
	}
	if !sharedInfo.Mode().IsRegular() {
		t.Fatalf("shared/CLAUDE.md should be a regular file, got %v", sharedInfo.Mode())
	}
}

func TestNewRejectsExisting(t *testing.T) {
	m, _ := setup(t)
	m.Init()
	if err := m.New("work"); err != nil {
		t.Fatal(err)
	}
	if err := m.New("work"); err == nil {
		t.Errorf("expected duplicate error")
	}
}

func TestList(t *testing.T) {
	m, _ := setup(t)
	m.Init()
	m.New("a")
	m.New("b")
	names, err := m.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("got %v", names)
	}
}

func TestPathReturnsAbsolute(t *testing.T) {
	m, p := setup(t)
	m.Init()
	m.New("work")
	got, err := m.Path("work")
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if got != p.ProfilePath("work") {
		t.Errorf("got %q", got)
	}
	if _, err := m.Path("missing"); err == nil {
		t.Errorf("expected error for missing profile")
	}
}

func TestInitIsIdempotent(t *testing.T) {
	m, _ := setup(t)
	if err := m.Init(); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := m.Init(); err != nil {
		t.Fatalf("second: %v", err)
	}
}
