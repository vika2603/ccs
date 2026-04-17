package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallShimWritesFile(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	out, err := runCmd(t, home, "install-shim")
	if err != nil {
		t.Fatalf("install-shim: %v", err)
	}

	shim := filepath.Join(home, ".ccs", "bin", "claude")
	info, err := os.Stat(shim)
	if err != nil {
		t.Fatalf("stat shim: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("shim not executable: mode %v", info.Mode())
	}

	body, err := os.ReadFile(shim)
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if !strings.HasPrefix(content, "#!/bin/sh\n") {
		t.Errorf("shim missing shebang: %q", content)
	}
	if !strings.Contains(content, "__shim_exec") {
		t.Errorf("shim should call __shim_exec: %q", content)
	}
	if !strings.Contains(content, "'claude'") {
		t.Errorf("shim should pass target 'claude': %q", content)
	}

	// Output tells the user where the shim lives + PATH guidance.
	if !strings.Contains(out, shim) {
		t.Errorf("install output missing shim path: %q", out)
	}
	if !strings.Contains(out, ".zprofile") {
		t.Errorf("install output missing .zprofile hint: %q", out)
	}
}

func TestInstallShimRefusesOverwriteWithoutForce(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runCmd(t, home, "install-shim"); err != nil {
		t.Fatalf("install-shim: %v", err)
	}
	if _, err := runCmd(t, home, "install-shim"); err == nil {
		t.Errorf("expected error when shim already exists")
	}
	if _, err := runCmd(t, home, "install-shim", "--force"); err != nil {
		t.Errorf("--force should overwrite: %v", err)
	}
}

func TestShimScriptQuotesPathsWithSpaces(t *testing.T) {
	got := shimScript("/home/user dir/ccs", "claude")
	if !strings.Contains(got, "'/home/user dir/ccs'") {
		t.Errorf("ccs path not quoted: %q", got)
	}
}

func TestShimScriptEscapesApostrophe(t *testing.T) {
	got := shimScript("/weird/it's/ccs", "claude")
	// Inside single quotes, an apostrophe becomes '\''.
	if !strings.Contains(got, `'/weird/it'\''s/ccs'`) {
		t.Errorf("apostrophe not escaped: %q", got)
	}
}

// TestShimExecPassthroughOnNoActive: with no active profile, __shim_exec
// should resolve the target against PATH (minus ~/.ccs/bin) and try to exec.
// We pass a bogus binary name so the call fails at resolution and never
// actually replaces the test process.
func TestShimExecPassthroughOnNoActive(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	// No active profile.
	_, err := runCmd(t, home, "__shim_exec", "ccs-nonexistent-binary-for-tests-zzz")
	if err == nil {
		t.Fatalf("expected error from missing binary")
	}
	if strings.Contains(err.Error(), "no active profile") {
		t.Errorf("passthrough should not complain about profile: %v", err)
	}
}

// TestShimExecWithActiveProfileReachesResolve: with an active profile and a
// bogus command name, error comes from resolution (profile loading succeeded).
func TestShimExecWithActiveProfileReachesResolve(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := runCmd(t, home, "new", "work"); err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := runCmd(t, home, "use", "work"); err != nil {
		t.Fatalf("use: %v", err)
	}
	_, err := runCmd(t, home, "__shim_exec", "ccs-nonexistent-binary-for-tests-zzz")
	if err == nil {
		t.Fatalf("expected error from missing binary")
	}
}

// TestShimExecDoesNotConsumeTargetFlags verifies --help after the target is
// forwarded to the target, not swallowed by cobra. The bogus binary fails at
// resolution; the test asserts we got the resolve error, not cobra's help
// output.
func TestShimExecDoesNotConsumeTargetFlags(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	_, err := runCmd(t, home, "__shim_exec", "ccs-nonexistent-bin-xyz", "--help", "--version")
	if err == nil {
		t.Fatalf("expected error from missing binary")
	}
	if !strings.Contains(err.Error(), "ccs-nonexistent-bin-xyz") {
		t.Errorf("expected resolution error about target binary, got: %v", err)
	}
}
