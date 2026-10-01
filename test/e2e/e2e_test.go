package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vika2603/ccs/internal/config"
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

	run(t, bin, home, "new", "main", "--from", src)
	run(t, bin, home, "new", "work")

	run(t, bin, home, "use", "work")
	run(t, bin, home, "field", "fork", "skills", "work")

	out := run(t, bin, home, "status", "work")
	if !strings.Contains(out, "forked") {
		t.Errorf("status: %q", out)
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
	runEnvOrFail(nil, "field", "fork", "CLAUDE.md", "beta")
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

func TestCloneProfile(t *testing.T) {
	bin := buildBinary(t)
	home := t.TempDir()
	run(t, bin, home, "init")
	run(t, bin, home, "new", "src")

	// Write isolated data into source profile.
	if err := os.WriteFile(filepath.Join(home, ".ccs", "profiles", "src", ".claude.json"), []byte(`{"user":"alice"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write shared content.
	if err := os.WriteFile(filepath.Join(home, ".ccs", "shared", "CLAUDE.md"), []byte("shared-mem\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run(t, bin, home, "new", "dst", "--from", "src")

	// Isolated file should be a real copy.
	b, err := os.ReadFile(filepath.Join(home, ".ccs", "profiles", "dst", ".claude.json"))
	if err != nil || string(b) != `{"user":"alice"}` {
		t.Errorf("cloned .claude.json: %v / %q", err, b)
	}

	// Shared field should be a symlink resolving to shared content.
	dstCLAUDE := filepath.Join(home, ".ccs", "profiles", "dst", "CLAUDE.md")
	info, err := os.Lstat(dstCLAUDE)
	if err != nil {
		t.Fatalf("lstat dst CLAUDE.md: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("dst CLAUDE.md should be a symlink")
	}
	if b, err := os.ReadFile(dstCLAUDE); err != nil || string(b) != "shared-mem\n" {
		t.Errorf("dst CLAUDE.md content: %v / %q", err, b)
	}

	// Source should be unaffected.
	b, err = os.ReadFile(filepath.Join(home, ".ccs", "profiles", "src", ".claude.json"))
	if err != nil || string(b) != `{"user":"alice"}` {
		t.Errorf("source .claude.json altered: %v / %q", err, b)
	}

	// ls should show both.
	out := run(t, bin, home, "ls")
	if !strings.Contains(out, "src") || !strings.Contains(out, "dst") {
		t.Errorf("ls: %q", out)
	}
}

// TestLaunchWrapperRoutesThroughShimPreservesProfile covers a wrapping
// launch.command (e.g. `caffeinate claude`): `ccs b` execs the wrapper with
// CLAUDE_CONFIG_DIR set to b, the wrapper resolves `claude` via PATH back to
// ~/.ccs/bin/claude, and the shim must keep b instead of falling back to the
// active profile.
func TestLaunchWrapperRoutesThroughShimPreservesProfile(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	bin := buildBinary(t)
	home := t.TempDir()
	run(t, bin, home, "init")
	run(t, bin, home, "new", "a")
	run(t, bin, home, "new", "b")
	run(t, bin, home, "use", "a")

	cfgPath := filepath.Join(home, ".ccs", "config.toml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Launch.Command = []string{"sh", "-c", "claude"}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}

	fakeDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeDir, "claude"), []byte("#!/bin/sh\necho CCD=$CLAUDE_CONFIG_DIR\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The shim comes first on PATH, as in a real setup.
	shimDir := filepath.Join(home, ".ccs", "bin")
	env := []string{"PATH=" + shimDir + ":" + fakeDir + ":/usr/bin:/bin"}

	out, err := runEnv(t, bin, home, env, "b")
	if err != nil {
		t.Fatalf("ccs b: %v\n%s", err, out)
	}
	want := "CCD=" + filepath.Join(home, ".ccs", "profiles", "b")
	if strings.TrimSpace(out) != want {
		t.Errorf("got %q, want %q", strings.TrimSpace(out), want)
	}
}

// `ccs -- args` names no profile: it behaves like `claude args` through the
// shim, using the active profile or the default ~/.claude when none is set.
func TestDashPassesArgsToDefaultClaude(t *testing.T) {
	// The test may itself run under a ccs-managed claude.
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	bin := buildBinary(t)
	home := t.TempDir()
	run(t, bin, home, "init")
	run(t, bin, home, "new", "work")

	fakeDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho \"CCD=$CLAUDE_CONFIG_DIR ARGS=$*\"\n"
	if err := os.WriteFile(filepath.Join(fakeDir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + fakeDir + ":/usr/bin:/bin"}
	workDir := filepath.Join(home, ".ccs", "profiles", "work")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no active profile", []string{"--", "-v", "--model", "x"}, "CCD= ARGS=-v --model x"},
		{"active profile", []string{"--", "-v"}, "CCD=" + workDir + " ARGS=-v"},
		{"explicit profile", []string{"work", "--", "-v"}, "CCD=" + workDir + " ARGS=-v"},
	}
	for i, tc := range cases {
		if i == 1 {
			run(t, bin, home, "use", "work")
		}
		out, err := runEnv(t, bin, home, env, tc.args...)
		if err != nil {
			t.Fatalf("%s: %v\n%s", tc.name, err, out)
		}
		if got := strings.TrimSpace(out); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}

	// An explicit CLAUDE_CONFIG_DIR wins over the active profile, as in the shim.
	custom := filepath.Join(home, "custom")
	out, err := runEnv(t, bin, home, append(env, "CLAUDE_CONFIG_DIR="+custom), "--", "-c")
	if err != nil {
		t.Fatalf("explicit CCD: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(out); got != "CCD="+custom+" ARGS=-c" {
		t.Errorf("explicit CCD: got %q", got)
	}

	out, err = runEnv(t, bin, home, env, "-v")
	if err != nil || !strings.HasPrefix(out, "ccs ") {
		t.Errorf("`ccs -v` should still print the ccs version, got %q, %v", out, err)
	}
}
