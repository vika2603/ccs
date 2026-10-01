package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmdWithInput(t *testing.T, home, stdin string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("HOME", home)
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func runCmd(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	return runCmdWithInput(t, home, "", args...)
}

func mustRun(t *testing.T, home string, args ...string) string {
	t.Helper()
	out, err := runCmd(t, home, args...)
	if err != nil {
		t.Fatalf("ccs %v: %v\n%s", args, err, out)
	}
	return out
}

func TestInitInstallsShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", "/usr/bin:/bin")
	out := mustRun(t, home, "init")
	shim := filepath.Join(home, ".ccs", "bin", "claude")
	info, err := os.Stat(shim)
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("shim missing or not executable: %v", err)
	}
	if b, _ := os.ReadFile(shim); !strings.Contains(string(b), "__shim_exec claude") {
		t.Errorf("unexpected shim: %s", b)
	}
	if !strings.Contains(out, ".zprofile") {
		t.Errorf("expected PATH hint: %q", out)
	}
	mustRun(t, home, "init")
}

func TestInitReplacesShimSymlinkWithoutTouchingTarget(t *testing.T) {
	home := t.TempDir()
	real := filepath.Join(home, "real-claude")
	if err := os.WriteFile(real, []byte("real"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(home, ".ccs", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, shim); err != nil {
		t.Fatal(err)
	}
	mustRun(t, home, "init")
	if b, _ := os.ReadFile(real); string(b) != "real" {
		t.Errorf("symlink target overwritten: %q", b)
	}
}

func TestShimScriptQuotesPath(t *testing.T) {
	if got := shimScript("/weird/it's dir/ccs"); !strings.Contains(got, `'/weird/it'\''s dir/ccs'`) {
		t.Errorf("path not quoted: %q", got)
	}
}

func TestNewLsUseRm(t *testing.T) {
	home := t.TempDir()
	mustRun(t, home, "new", "gw")
	mustRun(t, home, "new", "work", "--login")
	if _, err := runCmd(t, home, "new", "default"); err == nil {
		t.Error("default is reserved")
	}
	if _, err := runCmd(t, home, "new", "use"); err == nil {
		t.Error("subcommand names are reserved")
	}

	gw := filepath.Join(home, ".ccs", "profiles", "gw.toml")
	if err := os.WriteFile(gw, []byte("[settings.env]\nANTHROPIC_BASE_URL = \"https://gw.example.com\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := mustRun(t, home, "ls")
	for _, want := range []string{"* default", "gw", "api: gw.example.com", "work", "login"} {
		if !strings.Contains(out, want) {
			t.Errorf("ls missing %q:\n%s", want, out)
		}
	}

	mustRun(t, home, "use", "work")
	if out := mustRun(t, home, "ls"); !strings.Contains(out, "* work") {
		t.Errorf("work should be active:\n%s", out)
	}
	if _, err := runCmd(t, home, "use", "ghost"); err == nil {
		t.Error("use should reject a missing profile")
	}

	if _, err := runCmdWithInput(t, home, "n\n", "rm", "work"); err == nil {
		t.Error("declined rm should abort")
	}
	mustRun(t, home, "rm", "work", "-y")
	if _, err := os.Stat(filepath.Join(home, ".ccs", "accounts", "work")); !os.IsNotExist(err) {
		t.Errorf("account directory should be removed: %v", err)
	}
	if out := mustRun(t, home, "ls"); !strings.Contains(out, "* default") {
		t.Errorf("removing the active profile should fall back to default:\n%s", out)
	}
	mustRun(t, home, "use", "default")
	if _, err := runCmd(t, home, "rm", "default", "-y"); err == nil {
		t.Error("default must not be removable")
	}
}

func TestEditRestoresInvalidProfile(t *testing.T) {
	home := t.TempDir()
	mustRun(t, home, "new", "gw")
	path := filepath.Join(home, ".ccs", "profiles", "gw.toml")
	before, _ := os.ReadFile(path)

	t.Setenv("EDITOR", `sh -c 'printf "logn = true\n" > "$1"' sh`)
	if _, err := runCmd(t, home, "edit", "gw"); err == nil {
		t.Fatal("edit should reject an invalid profile")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("previous content not restored:\n%s", after)
	}

	t.Setenv("EDITOR", `sh -c 'printf "login = true\n" > "$1"' sh`)
	mustRun(t, home, "edit", "gw")
	if out := mustRun(t, home, "ls"); !strings.Contains(out, "login") {
		t.Errorf("edited profile not applied:\n%s", out)
	}
	if _, err := runCmd(t, home, "edit", "default"); err == nil {
		t.Error("the default profile has no file to edit")
	}
}

func TestVersionFlag(t *testing.T) {
	if out := mustRun(t, t.TempDir(), "--version"); !strings.HasPrefix(out, "ccs ") {
		t.Errorf("unexpected version output %q", out)
	}
}
