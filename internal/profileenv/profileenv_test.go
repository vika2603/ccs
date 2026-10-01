package profileenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	f, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if len(f.Env) != 0 {
		t.Errorf("expected empty map, got %v", f.Env)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env", "work.toml")
	in := File{Env: map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-xyz",
		"HTTP_PROXY":        "http://localhost:7890",
		"WEIRD":             "has 'quote' and $dollar\nand newline",
	}}
	if err := Save(path, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(out.Env) != len(in.Env) {
		t.Fatalf("len mismatch: got %d want %d", len(out.Env), len(in.Env))
	}
	for k, v := range in.Env {
		if out.Env[k] != v {
			t.Errorf("key %q: got %q want %q", k, out.Env[k], v)
		}
	}
}

func TestSaveSetsMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are unix-only")
	}
	path := filepath.Join(t.TempDir(), "env", "w.toml")
	if err := Save(path, File{Env: map[string]string{"X": "y"}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 0600", info.Mode().Perm())
	}
}

func TestValidName(t *testing.T) {
	good := []string{"FOO", "_FOO", "FOO_BAR", "F", "_", "F1_2"}
	bad := []string{"", "1FOO", "FOO-BAR", "FOO BAR", "foo.bar", "FOO=BAR"}
	for _, k := range good {
		if err := ValidName(k); err != nil {
			t.Errorf("%q: unexpected error %v", k, err)
		}
	}
	for _, k := range bad {
		if err := ValidName(k); err == nil {
			t.Errorf("%q: expected error", k)
		}
	}
}

func TestLoadRejectsBadName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("[env]\n\"FOO-BAR\" = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected error for invalid key")
	}
}

func TestParseAssignment(t *testing.T) {
	k, v, err := ParseAssignment("FOO=bar=baz")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if k != "FOO" || v != "bar=baz" {
		t.Errorf("got (%q,%q)", k, v)
	}
	if _, _, err := ParseAssignment("no_equals"); err == nil {
		t.Errorf("expected error for missing '='")
	}
	if _, _, err := ParseAssignment("1BAD=v"); err == nil {
		t.Errorf("expected error for invalid name")
	}
}
