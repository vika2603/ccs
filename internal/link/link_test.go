package link

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

func TestIsSymlinkTo(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	os.Mkdir(target, 0o755)
	link := filepath.Join(dir, "link")
	os.Symlink(target, link)
	ok, err := IsSymlinkTo(link, target)
	if err != nil || !ok {
		t.Errorf("expected symlink to match: %v", err)
	}
}

func TestReplaceSymlinkWithCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	os.MkdirAll(filepath.Join(src, "sub"), 0o755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o644)
	link := filepath.Join(dir, "link")
	os.Symlink(src, link)

	if err := ReplaceSymlinkWithCopy(link); err != nil {
		t.Fatalf("replace: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("link should no longer be a symlink")
	}
	b, _ := os.ReadFile(filepath.Join(link, "sub", "b.txt"))
	if string(b) != "world" {
		t.Errorf("copy contents mismatch: %q", b)
	}
}

func TestReplaceCopyWithSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	os.MkdirAll(real, 0o755)
	os.WriteFile(filepath.Join(real, "a.txt"), []byte("hi"), 0o644)
	target := filepath.Join(dir, "target")
	os.Mkdir(target, 0o755)

	if err := ReplaceCopyWithSymlink(real, target); err != nil {
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
