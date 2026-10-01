package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSymlinkCreates(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	os.Mkdir(target, 0o755)
	link := filepath.Join(dir, "link")
	if err := EnsureSymlink(target, link); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	got, _ := os.Readlink(link)
	if got != target {
		t.Errorf("got %q want %q", got, target)
	}
}

func TestEnsureSymlinkReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	target1 := filepath.Join(dir, "t1")
	target2 := filepath.Join(dir, "t2")
	os.Mkdir(target1, 0o755)
	os.Mkdir(target2, 0o755)
	link := filepath.Join(dir, "link")
	os.Symlink(target1, link)
	if err := EnsureSymlink(target2, link); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	got, _ := os.Readlink(link)
	if got != target2 {
		t.Errorf("got %q", got)
	}
}

func TestForceSymlinkReplacesRealCopy(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	os.MkdirAll(real, 0o755)
	os.WriteFile(filepath.Join(real, "a.txt"), []byte("hi"), 0o644)
	target := filepath.Join(dir, "target")
	os.Mkdir(target, 0o755)

	if err := ForceSymlink(target, real); err != nil {
		t.Fatalf("replace: %v", err)
	}
	info, err := os.Lstat(real)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink")
	}
	resolved, _ := os.Readlink(real)
	if resolved != target {
		t.Errorf("target mismatch: %q", resolved)
	}
}

func TestIsEmpty(t *testing.T) {
	dir := t.TempDir()
	emptyDir := filepath.Join(dir, "empty")
	os.Mkdir(emptyDir, 0o755)
	emptyFile := filepath.Join(dir, "empty.md")
	os.WriteFile(emptyFile, nil, 0o644)
	full := filepath.Join(dir, "full.md")
	os.WriteFile(full, []byte("x"), 0o644)
	for path, want := range map[string]bool{
		filepath.Join(dir, "missing"): true,
		emptyDir:                      true,
		emptyFile:                     true,
		full:                          false,
		dir:                           false,
	} {
		got, err := IsEmpty(path)
		if err != nil || got != want {
			t.Errorf("IsEmpty(%s) = %v, %v; want %v", path, got, err, want)
		}
	}
}
