package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "ccs")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/ccs")
	cmd.Dir = filepath.Join("..", "..")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	return out
}

func run(t *testing.T, ccs, home string, args ...string) string {
	t.Helper()
	cmd := exec.Command(ccs, args...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", ccs, args, err, out)
	}
	return string(out)
}

func runEnv(t *testing.T, ccs, home string, extraEnv []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(ccs, args...)
	cmd.Env = append(append(os.Environ(), "HOME="+home), extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestFullFlow(t *testing.T) {
	bin := buildBinary(t)
	home := t.TempDir()
	run(t, bin, home, "init")

	src := filepath.Join(home, "src.claude")
	os.MkdirAll(filepath.Join(src, "skills", "hello"), 0o755)
	os.WriteFile(filepath.Join(src, "skills", "hello", "SKILL.md"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(src, "CLAUDE.md"), []byte("mem"), 0o644)
	os.MkdirAll(filepath.Join(src, "projects"), 0o755)
	os.WriteFile(filepath.Join(src, "projects", "p.txt"), []byte("p"), 0o644)

	run(t, bin, home, "adopt", src, "main")
	run(t, bin, home, "new", "work")

	run(t, bin, home, "use", "work")
	run(t, bin, home, "fork", "skills", "work")

	out := run(t, bin, home, "status", "work")
	if !strings.Contains(out, "forked") {
		t.Errorf("status: %q", out)
	}

	exportFile := filepath.Join(t.TempDir(), "main.tar.gz")
	run(t, bin, home, "export", "main", "-o", exportFile)
	dst := t.TempDir()
	run(t, bin, dst, "init")
	run(t, bin, dst, "import", exportFile, "--as", "main2")

	b, err := os.ReadFile(filepath.Join(dst, ".ccs", "shared", "skills", "hello", "SKILL.md"))
	if err != nil || string(b) != "hi" {
		t.Errorf("restore content: %v / %q", err, b)
	}

	if got := run(t, bin, home, "doctor"); !strings.Contains(got, "clean") && !strings.Contains(got, "orphan-shared-field") {
		t.Errorf("doctor: %q", got)
	}
}

func TestBackupRestore(t *testing.T) {
	bin := buildBinary(t)
	home := t.TempDir()
	runEnvOrFail := func(extraEnv []string, args ...string) string {
		t.Helper()
		out, err := runEnv(t, bin, home, extraEnv, args...)
		if err != nil {
			t.Fatalf("ccs %v: %v\n%s", args, err, out)
		}
		return out
	}

	runEnvOrFail(nil, "init")
	runEnvOrFail(nil, "new", "alpha")
	runEnvOrFail(nil, "new", "beta")
	runEnvOrFail(nil, "env", "set", "alpha", "FOO=bar")
	runEnvOrFail(nil, "use", "alpha")
	runEnvOrFail(nil, "fork", "CLAUDE.md", "beta")
	if err := os.WriteFile(filepath.Join(home, ".ccs", "profiles", "beta", "CLAUDE.md"), []byte("beta-local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".ccs", "shared", "CLAUDE.md"), []byte("shared-mem\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupFile := filepath.Join(t.TempDir(), "backup.tar.gz")
	runEnvOrFail([]string{"CCS_PASSPHRASE=test"}, "backup", "-o", backupFile)

	dst := t.TempDir()
	out, err := runEnv(t, bin, dst, []string{"CCS_PASSPHRASE=test"}, "restore", backupFile)
	if err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}

	for _, name := range []string{"alpha", "beta"} {
		if _, err := os.Stat(filepath.Join(dst, ".ccs", "profiles", name)); err != nil {
			t.Errorf("profile %s missing after restore: %v", name, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dst, ".ccs", "shared", "CLAUDE.md")); err != nil || string(b) != "shared-mem\n" {
		t.Errorf("restored shared CLAUDE.md: %v / %q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dst, ".ccs", "profiles", "beta", "CLAUDE.md")); err != nil || string(b) != "beta-local\n" {
		t.Errorf("restored beta fork: %v / %q", err, b)
	}
	// alpha CLAUDE.md should be a symlink that resolves to shared.
	alphaCLAUDE := filepath.Join(dst, ".ccs", "profiles", "alpha", "CLAUDE.md")
	info, err := os.Lstat(alphaCLAUDE)
	if err != nil {
		t.Fatalf("lstat alpha CLAUDE.md: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("alpha CLAUDE.md should be a symlink after restore")
	}
	if b, err := os.ReadFile(alphaCLAUDE); err != nil || string(b) != "shared-mem\n" {
		t.Errorf("follow alpha CLAUDE.md: %v / %q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dst, ".ccs", "env", "alpha.toml")); err != nil || !strings.Contains(string(b), "FOO") {
		t.Errorf("restored env alpha.toml: %v / %q", err, b)
	}
	// active profile should be alpha
	if b, err := os.ReadFile(filepath.Join(dst, ".ccs", "state", "active")); err != nil || strings.TrimSpace(string(b)) != "alpha" {
		t.Errorf("active after restore: %v / %q", err, b)
	}
}

// TestRunRoutesThroughShimPreservesProfile reproduces the bug where
// `ccs run <profile>` execs a wrapper (e.g. `caffeinate claude`) whose
// child resolves `claude` via PATH back through ~/.ccs/bin/claude. The
// shim must honor the CCD the outer `ccs run` already set and not fall
// back to state/active, which would silently swap to the wrong profile.
func TestRunRoutesThroughShimPreservesProfile(t *testing.T) {
	bin := buildBinary(t)
	home := t.TempDir()
	run(t, bin, home, "init")
	run(t, bin, home, "new", "a")
	run(t, bin, home, "new", "b")
	run(t, bin, home, "use", "a") // state/active = a
	run(t, bin, home, "install-shim")

	// Fake `claude` that prints its CCD so we can observe which profile
	// actually reached the final process.
	fakeDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeClaude := filepath.Join(fakeDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho CCD=$CLAUDE_CONFIG_DIR\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// PATH: ~/.ccs/bin (shim) first, then fakebin. Matches the real setup
	// where `claude` without absolute path hits the shim, and the shim's
	// passthrough (ResolveSkipping ~/.ccs/bin) then finds fake claude.
	shimDir := filepath.Join(home, ".ccs", "bin")
	extraEnv := []string{"PATH=" + shimDir + ":" + fakeDir + ":/usr/bin:/bin"}

	// `sh -c claude` simulates a launch wrapper like `caffeinate claude`:
	// ccs run b execs sh with CCD=b; sh resolves `claude` via PATH (ccs
	// can't skip BinDir for a child process) -> hits the shim ->
	// __shim_exec must honor the already-set CCD=b rather than fall back
	// to state/active=a.
	out, err := runEnv(t, bin, home, extraEnv, "run", "b", "--", "sh", "-c", "claude")
	if err != nil {
		t.Fatalf("run b: %v\n%s", err, out)
	}
	wantCCD := filepath.Join(home, ".ccs", "profiles", "b")
	if !strings.Contains(out, "CCD="+wantCCD) {
		t.Errorf("expected CCD=%s, got: %q", wantCCD, out)
	}
	gotCCDa := filepath.Join(home, ".ccs", "profiles", "a")
	if strings.Contains(out, "CCD="+gotCCDa) {
		t.Errorf("shim clobbered CCD back to active profile a: %q", out)
	}
}
