package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportExistingDir(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(home, "src.claude")
	os.MkdirAll(filepath.Join(src, "skills"), 0o755)
	os.WriteFile(filepath.Join(src, "skills", "a.md"), []byte("A"), 0o644)
	os.MkdirAll(filepath.Join(src, "projects"), 0o755)
	os.WriteFile(filepath.Join(src, "projects", "notes.txt"), []byte("notes"), 0o644)
	os.WriteFile(filepath.Join(src, "CLAUDE.md"), []byte("memory"), 0o644)

	runCmd(t, home, "init")
	_, err := runCmd(t, home, "import", src, "main")
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	info, _ := os.Lstat(filepath.Join(home, ".ccs", "profiles", "main", "skills"))
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink for skills")
	}
	b, err := os.ReadFile(filepath.Join(home, ".ccs", "shared", "skills", "a.md"))
	if err != nil || string(b) != "A" {
		t.Errorf("shared content missing")
	}

	b, err = os.ReadFile(filepath.Join(home, ".ccs", "profiles", "main", "projects", "notes.txt"))
	if err != nil || string(b) != "notes" {
		t.Errorf("isolated runtime content missing after import: %v / %q", err, b)
	}

	info, _ = os.Lstat(filepath.Join(home, ".ccs", "profiles", "main", "CLAUDE.md"))
	if info == nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink for file-shaped CLAUDE.md")
	}
	b, err = os.ReadFile(filepath.Join(home, ".ccs", "shared", "CLAUDE.md"))
	if err != nil || string(b) != "memory" {
		t.Errorf("shared CLAUDE.md content missing: %v / %q", err, b)
	}
}
