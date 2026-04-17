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

func TestImportStandardLayoutPicksUpHomeClaudeJSON(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(home, ".claude")
	os.MkdirAll(filepath.Join(src, "skills"), 0o755)
	os.WriteFile(filepath.Join(src, "CLAUDE.md"), []byte("memory"), 0o644)

	siblingBytes := []byte(`{"oauthAccount":{"email":"x@example.com"},"userID":"u1"}`)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), siblingBytes, 0o600); err != nil {
		t.Fatalf("write sibling: %v", err)
	}

	runCmd(t, home, "init")
	out, err := runCmd(t, home, "import", src, "main")
	if err != nil {
		t.Fatalf("import: %v\noutput: %s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(home, ".ccs", "profiles", "main", ".claude.json"))
	if err != nil {
		t.Fatalf("read imported .claude.json: %v", err)
	}
	if string(got) != string(siblingBytes) {
		t.Errorf(".claude.json bytes mismatch:\n got=%q\nwant=%q", got, siblingBytes)
	}

	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err != nil {
		t.Errorf("sibling $HOME/.claude.json should be preserved, got: %v", err)
	}
}

func TestMaybeImportClaudeJSONCopiesOnEmptyInput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(home, ".claude")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	want := []byte(`{"oauthAccount":{"email":"x@example.com"}}`)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), want, 0o600); err != nil {
		t.Fatal(err)
	}

	in := strings.NewReader("\n")
	var out, errOut bytes.Buffer
	if err := maybeImportClaudeJSON(src, dst, "main", in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportClaudeJSON: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, ".claude.json"))
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("bytes mismatch: got=%q want=%q", got, want)
	}
	info, err := os.Stat(filepath.Join(dst, ".claude.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("expected mode 0o600, got %v (err=%v)", info.Mode().Perm(), err)
	}
	if !strings.Contains(out.String(), "found .claude.json sibling") {
		t.Errorf("expected prompt, got %q", out.String())
	}
}

func TestMaybeImportClaudeJSONSkipsOnNo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(home, ".claude")
	os.MkdirAll(src, 0o755)
	dst := t.TempDir()
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600)

	in := strings.NewReader("n\n")
	var out, errOut bytes.Buffer
	if err := maybeImportClaudeJSON(src, dst, "main", in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportClaudeJSON: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("expected dst untouched on no, stat err=%v", err)
	}
	if !strings.Contains(out.String(), "skipped .claude.json") {
		t.Errorf("expected skip message, got %q", out.String())
	}
}

func TestMaybeImportClaudeJSONSkipsWhenInDirJSONExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(home, ".claude")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, ".claude.json"), []byte(`{"in":"dir"}`), 0o600)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"sibling":true}`), 0o600)
	dst := t.TempDir()

	in := strings.NewReader("")
	var out, errOut bytes.Buffer
	if err := maybeImportClaudeJSON(src, dst, "main", in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportClaudeJSON: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no prompt when in-dir .claude.json exists, got %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(dst, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("helper must not write dst when in-dir json exists; stat err=%v", err)
	}
}

func TestMaybeImportClaudeJSONSilentWhenSrcNotHomeClaude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600)
	src := t.TempDir()
	dst := t.TempDir()

	in := strings.NewReader("")
	var out, errOut bytes.Buffer
	if err := maybeImportClaudeJSON(src, dst, "main", in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportClaudeJSON: %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("expected silent for non-$HOME/.claude src, got out=%q err=%q", out.String(), errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dst, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("must not import when src is not $HOME/.claude; stat err=%v", err)
	}
}

func TestMaybeImportClaudeJSONSilentWhenSiblingMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(home, ".claude")
	os.MkdirAll(src, 0o755)
	dst := t.TempDir()

	in := strings.NewReader("")
	var out, errOut bytes.Buffer
	if err := maybeImportClaudeJSON(src, dst, "main", in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportClaudeJSON: %v", err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("expected silent when sibling missing, got out=%q err=%q", out.String(), errOut.String())
	}
}

func TestMaybeImportClaudeJSONDoesNotDeleteSibling(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(home, ".claude")
	os.MkdirAll(src, 0o755)
	dst := t.TempDir()
	siblingBytes := []byte(`{"keep":"me"}`)
	siblingPath := filepath.Join(home, ".claude.json")
	os.WriteFile(siblingPath, siblingBytes, 0o600)

	in := strings.NewReader("y\n")
	var out, errOut bytes.Buffer
	if err := maybeImportClaudeJSON(src, dst, "main", in, &out, &errOut); err != nil {
		t.Fatalf("maybeImportClaudeJSON: %v", err)
	}
	got, err := os.ReadFile(siblingPath)
	if err != nil {
		t.Fatalf("sibling must remain readable, got err=%v", err)
	}
	if string(got) != string(siblingBytes) {
		t.Errorf("sibling bytes changed: got=%q want=%q", got, siblingBytes)
	}
}
