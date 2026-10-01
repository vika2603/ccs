package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitInstallsShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", "/usr/bin:/bin")
	out, err := runCmd(t, home, "init")
	if err != nil {
		t.Fatalf("init: %v", err)
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
	if !strings.HasPrefix(content, "#!/bin/sh\n") || !strings.Contains(content, "__shim_exec") || !strings.Contains(content, "'claude'") {
		t.Errorf("unexpected shim content: %q", content)
	}
	if !strings.Contains(out, ".zprofile") {
		t.Errorf("init should print a PATH hint when ~/.ccs/bin is not on PATH: %q", out)
	}

	// Re-running init rewrites the shim instead of failing.
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("second init: %v", err)
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

// TestShimExecHonorsExistingCCD guards the `ccs <profile>` → shim loop: when
// an outer ccs already set CLAUDE_CONFIG_DIR (e.g. a launch.command wrapper
// like `caffeinate claude` routes `claude` through the PATH shim),
// __shim_exec must not re-derive CCD from state/active. state/active points
// at a missing profile, so consulting it would surface as a lookup error;
// passing through reaches binary resolution instead.
func TestShimExecHonorsExistingCCD(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	// state/active points at a profile that doesn't exist on disk.
	if err := os.WriteFile(filepath.Join(home, ".ccs", "state", "active"), []byte("ghost\n"), 0o644); err != nil {
		t.Fatalf("write active: %v", err)
	}
	// Simulate an outer `ccs vika` having set CCD before the shim ran.
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".ccs", "profiles", "vika"))
	_, err := runCmd(t, home, "__shim_exec", "ccs-nonexistent-binary-for-tests-zzz")
	if err == nil {
		t.Fatalf("expected error from missing binary")
	}
	if strings.Contains(err.Error(), "does not exist") {
		t.Errorf("shim should not have consulted state/active when CCD is set: %v", err)
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

func TestInitReplacesShimSymlinkWithoutTouchingTarget(t *testing.T) {
	home := t.TempDir()
	real := filepath.Join(home, "real-claude")
	if err := os.WriteFile(real, []byte("#!/bin/sh\necho real\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(home, ".ccs", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, shim); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if b, _ := os.ReadFile(real); string(b) != "#!/bin/sh\necho real\n" {
		t.Errorf("symlink target was overwritten: %q", b)
	}
	if info, err := os.Lstat(shim); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("shim should now be a regular file: %v", err)
	}
}
