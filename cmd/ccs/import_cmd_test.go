package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vika2603/ccs/internal/creds"
)

type fakeCredStore struct {
	entries map[string][]byte
	deleted map[string]bool
}

func newFakeCredStore() *fakeCredStore {
	return &fakeCredStore{entries: map[string][]byte{}, deleted: map[string]bool{}}
}

func (f *fakeCredStore) Read(p string) ([]byte, error) {
	if b, ok := f.entries[p]; ok {
		return b, nil
	}
	return nil, creds.ErrNotFound
}

func (f *fakeCredStore) Write(p string, b []byte) error {
	f.entries[p] = append([]byte(nil), b...)
	return nil
}

func (f *fakeCredStore) Delete(p string) error {
	delete(f.entries, p)
	f.deleted[p] = true
	return nil
}

func (f *fakeCredStore) Exists(p string) (bool, error) {
	_, ok := f.entries[p]
	return ok, nil
}

func TestMaybeImportCredsPromptsAndWritesOnYes(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	store := newFakeCredStore()
	srcAbs, _ := filepath.Abs(srcDir)
	dstAbs, _ := filepath.Abs(dstDir)
	store.entries[srcAbs] = []byte("TOKEN")

	in := strings.NewReader("y\n")
	var out, errOut bytes.Buffer
	if err := maybeImportCreds(srcDir, dstDir, "work", false, store, in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportCreds: %v", err)
	}
	if got := string(store.entries[dstAbs]); got != "TOKEN" {
		t.Errorf("expected TOKEN written at dst, got %q", got)
	}
	if store.deleted[srcAbs] {
		t.Errorf("did not expect source delete when move=false")
	}
	if !strings.Contains(out.String(), "found credentials") {
		t.Errorf("expected prompt, got %q", out.String())
	}
}

func TestMaybeImportCredsSkipsOnNo(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	store := newFakeCredStore()
	srcAbs, _ := filepath.Abs(srcDir)
	dstAbs, _ := filepath.Abs(dstDir)
	store.entries[srcAbs] = []byte("TOKEN")

	in := strings.NewReader("n\n")
	var out, errOut bytes.Buffer
	if err := maybeImportCreds(srcDir, dstDir, "work", false, store, in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportCreds: %v", err)
	}
	if _, ok := store.entries[dstAbs]; ok {
		t.Errorf("expected dst untouched on no")
	}
	if !strings.Contains(out.String(), "skipped") {
		t.Errorf("expected skip message, got %q", out.String())
	}
}

func TestMaybeImportCredsSilentWhenSourceMissing(t *testing.T) {
	store := newFakeCredStore()
	in := strings.NewReader("")
	var out, errOut bytes.Buffer
	if err := maybeImportCreds(t.TempDir(), t.TempDir(), "work", false, store, in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportCreds: %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("expected silent, got out=%q err=%q", out.String(), errOut.String())
	}
}

func TestMaybeImportCredsMoveDeletesSource(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	store := newFakeCredStore()
	srcAbs, _ := filepath.Abs(srcDir)
	store.entries[srcAbs] = []byte("TOKEN")

	in := strings.NewReader("y\n")
	var out, errOut bytes.Buffer
	if err := maybeImportCreds(srcDir, dstDir, "work", true, store, in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportCreds: %v", err)
	}
	if !store.deleted[srcAbs] {
		t.Errorf("expected source deletion when move=true")
	}
}

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
