package profileenv

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestSignatureStableOnUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.toml")
	if err := Save(path, File{Env: map[string]string{"A": "1"}}); err != nil {
		t.Fatal(err)
	}
	s1 := Signature("work", path)
	s2 := Signature("work", path)
	if s1 != s2 {
		t.Errorf("signature changed without edit: %q vs %q", s1, s2)
	}
}

func TestSignatureChangesOnEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.toml")
	if err := Save(path, File{Env: map[string]string{"A": "1"}}); err != nil {
		t.Fatal(err)
	}
	s1 := Signature("work", path)
	time.Sleep(10 * time.Millisecond)
	if err := Save(path, File{Env: map[string]string{"A": "2"}}); err != nil {
		t.Fatal(err)
	}
	s2 := Signature("work", path)
	if s1 == s2 {
		t.Errorf("signature did not change after edit: %q", s1)
	}
}

func TestSignatureEmptyProfile(t *testing.T) {
	if got := Signature("", ""); got != "!none" {
		t.Errorf("got %q, want !none", got)
	}
}

func TestRenderContainsCCDAndSig(t *testing.T) {
	out := Render(Action{
		Set:       map[string]string{"FOO": "bar", "BAZ": "qux"},
		ConfigDir: "/tmp/p",
		Sig:       "work:123",
	})
	for _, want := range []string{
		"if [ -n \"${CCS_MANAGED_VARS-}\" ]",
		"export CCS_ENV_SIG='work:123'",
		"export CLAUDE_CONFIG_DIR='/tmp/p'",
		"export CCS_MANAGED_CCD=1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q\n---\n%s", want, out)
		}
	}
}

// Profile env vars must not reach the shell. They're injected into the claude
// process via the ~/.ccs/bin/claude shim instead.
func TestRenderDoesNotExportProfileEnv(t *testing.T) {
	out := Render(Action{
		Set:       map[string]string{"FOO": "bar", "ANTHROPIC_API_KEY": "sk-secret"},
		ConfigDir: "/tmp/p",
		Sig:       "s",
	})
	for _, unwanted := range []string{
		"export FOO=",
		"export ANTHROPIC_API_KEY=",
		"export CCS_MANAGED_VARS=",
		"sk-secret",
		"'bar'",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("render unexpectedly contains %q\n---\n%s", unwanted, out)
		}
	}
}

func TestRenderClearAll(t *testing.T) {
	out := RenderClearAll()
	for _, want := range []string{
		"if [ -n \"${CCS_MANAGED_VARS-}\" ]",
		"unset CCS_MANAGED_VARS CCS_ENV_SIG CLAUDE_CONFIG_DIR CCS_MANAGED_CCD",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("clearall missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderClearManaged(t *testing.T) {
	out := RenderClearManaged()
	if !strings.Contains(out, "unset CCS_MANAGED_VARS CCS_ENV_SIG") {
		t.Errorf("expected basic unset line, got: %s", out)
	}
	if !strings.Contains(out, "[ -n \"${CCS_MANAGED_CCD-}\" ] && unset CLAUDE_CONFIG_DIR CCS_MANAGED_CCD") {
		t.Errorf("expected conditional CCD unset, got: %s", out)
	}
}
