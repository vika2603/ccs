package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCopyTreeFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside.txt")
	writeFile(t, outside, "linked", 0o644)
	src := filepath.Join(root, "src")
	writeFile(t, filepath.Join(src, "a", "b.txt"), "b", 0o600)
	if err := os.Symlink(outside, filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(root, "dst")
	if err := CopyTree(src, dst); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
	if got := readFile(t, filepath.Join(dst, "a", "b.txt")); got != "b" {
		t.Errorf("a/b.txt = %q", got)
	}
	info, err := os.Lstat(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("link.txt should be a regular file after CopyTree")
	}
	if got := readFile(t, filepath.Join(dst, "link.txt")); got != "linked" {
		t.Errorf("link.txt = %q", got)
	}
	if info, err := os.Stat(filepath.Join(dst, "a", "b.txt")); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("a/b.txt perm = %v, err = %v", info.Mode().Perm(), err)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.toml")
	if err := WriteFileAtomic(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "two" {
		t.Errorf("content = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

func TestWriteFileAtomicKeepsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "config.toml")
	writeFile(t, target, "old", 0o644)
	link := filepath.Join(dir, "config.toml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced by regular file: %v", err)
	}
	if got := readFile(t, target); got != "new" {
		t.Errorf("target content = %q", got)
	}
}
