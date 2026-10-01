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

type env struct {
	t    *testing.T
	bin  string
	home string
	path string
}

// setup builds ccs, runs `ccs init` in a fresh HOME, and puts the shim and a
// fake claude on PATH. The fake prints its config dir, the profile ccs
// recorded, whether ANTHROPIC_AUTH_TOKEN is set, and its arguments; see out.
func setup(t *testing.T) env {
	t.Helper()
	// The test may itself run under a ccs-managed claude, whose profile and
	// credentials must not leak into the runs.
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if key == "CLAUDE_CONFIG_DIR" || key == "CCS_PROFILE" || key == "CLAUDE_CODE_OAUTH_TOKEN" || strings.HasPrefix(key, "ANTHROPIC_") {
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}
	home := t.TempDir()
	fakeDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho \"CCD=$CLAUDE_CONFIG_DIR PROFILE=$CCS_PROFILE TOKEN=${ANTHROPIC_AUTH_TOKEN:+set} ARGS=$*\"\n"
	if err := os.WriteFile(filepath.Join(fakeDir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	e := env{
		t:    t,
		bin:  buildBinary(t),
		home: home,
		path: filepath.Join(home, ".ccs", "bin") + ":" + fakeDir + ":/usr/bin:/bin",
	}
	e.run(nil, "init")
	return e
}

// out is what the fake claude prints for the given environment and args.
func out(ccd, profile, token, args string) string {
	return "CCD=" + ccd + " PROFILE=" + profile + " TOKEN=" + token + " ARGS=" + args
}

func (e env) exec(name string, extraEnv []string, args ...string) string {
	e.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "HOME="+e.home, "PATH="+e.path)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		e.t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (e env) run(extraEnv []string, args ...string) string {
	e.t.Helper()
	return e.exec(e.bin, extraEnv, args...)
}

// claude runs `claude args...` through the shim.
func (e env) claude(extraEnv []string, args ...string) string {
	e.t.Helper()
	return e.exec(filepath.Join(e.home, ".ccs", "bin", "claude"), extraEnv, args...)
}

func (e env) write(rel, content string) {
	e.t.Helper()
	path := filepath.Join(e.home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		e.t.Fatal(err)
	}
}

func TestDefaultProfileRunsOnClaudeDir(t *testing.T) {
	e := setup(t)
	if got := e.run(nil, "--", "-c"); got != out("", "default", "", "-c") {
		t.Errorf("ccs -- -c: %q", got)
	}
	if got := e.claude(nil, "-p", "hi"); got != out("", "default", "", "-p hi") {
		t.Errorf("claude via shim: %q", got)
	}
}

func TestGatewayProfilePassesSettings(t *testing.T) {
	e := setup(t)
	e.run(nil, "new", "gw")
	e.write(".ccs/profiles/gw.toml", "[settings.env]\nANTHROPIC_BASE_URL = \"https://gw.example.com\"\nANTHROPIC_AUTH_TOKEN = \"secret\"\n")
	settings := filepath.Join(e.home, ".ccs", "run", "gw.settings.json")

	want := out("", "gw", "", "--settings "+settings+" -c")
	got := e.run(nil, "gw", "--", "-c")
	if got != want {
		t.Errorf("ccs gw -- -c: got %q, want %q", got, want)
	}
	if strings.Contains(got, "secret") {
		t.Error("token must not appear on the command line")
	}
	b, err := os.ReadFile(settings)
	if err != nil || !strings.Contains(string(b), "https://gw.example.com") {
		t.Errorf("settings file: %v %s", err, b)
	}

	e.run(nil, "use", "gw")
	if got := e.claude(nil, "-c"); got != want {
		t.Errorf("claude via shim with gw active: %q", got)
	}
}

func TestLoginProfileLinksClaudeDir(t *testing.T) {
	e := setup(t)
	e.write(".claude/skills/a/SKILL.md", "a")
	e.write(".claude/settings.json", "{}")
	e.write(".claude/projects/p.txt", "shared")
	e.run(nil, "new", "work", "--login")
	e.write(".ccs/profiles/work.toml", "login = true\nisolate = [\"projects\"]\n")
	acct := filepath.Join(e.home, ".ccs", "accounts", "work")

	if got := e.run(nil, "work"); got != out(acct, "work", "", "") {
		t.Errorf("ccs work: %q", got)
	}
	for _, name := range []string{"skills", "settings.json"} {
		if target, err := os.Readlink(filepath.Join(acct, name)); err != nil || target != filepath.Join(e.home, ".claude", name) {
			t.Errorf("%s should link to ~/.claude: %q %v", name, target, err)
		}
	}
	info, err := os.Lstat(filepath.Join(acct, "projects"))
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("isolated projects should be a real copy: %v", err)
	}
}

// A CLAUDE_CONFIG_DIR set by the caller wins over the active profile, so a
// wrapper that resolves `claude` back to the shim keeps its profile.
func TestShimHonorsCallerConfigDir(t *testing.T) {
	e := setup(t)
	e.run(nil, "new", "work", "--login")
	e.run(nil, "use", "work")
	custom := filepath.Join(e.home, "custom")
	extra := []string{"CLAUDE_CONFIG_DIR=" + custom}
	if got := e.claude(extra, "-c"); got != out(custom, "", "", "-c") {
		t.Errorf("claude via shim: %q", got)
	}
	if got := e.run(extra, "--", "-c"); got != out(custom, "", "", "-c") {
		t.Errorf("ccs -- -c: %q", got)
	}
	acct := filepath.Join(e.home, ".ccs", "accounts", "work")
	if got := e.run(extra, "work"); got != out(acct, "work", "", "") {
		t.Errorf("an explicit profile wins over the caller's dir: %q", got)
	}
}

// A claude started inside a ccs session, for example by a hook or the Bash
// tool, keeps the session's profile rather than switching to the active one.
// The outer session's environment is what such a child inherits.
func TestNestedClaudeKeepsSessionProfile(t *testing.T) {
	e := setup(t)
	e.run(nil, "new", "gw")
	e.write(".ccs/profiles/gw.toml", "[settings.env]\nANTHROPIC_AUTH_TOKEN = \"gw-token\"\n")
	e.run(nil, "new", "work", "--login")
	e.run(nil, "use", "work")
	settings := filepath.Join(e.home, ".ccs", "run", "gw.settings.json")
	acct := filepath.Join(e.home, ".ccs", "accounts", "work")

	inGw := []string{"CCS_PROFILE=gw", "ANTHROPIC_AUTH_TOKEN=gw-token"}
	if got, want := e.claude(inGw, "-c"), out("", "gw", "", "--settings "+settings+" -c"); got != want {
		t.Errorf("claude inside gw: got %q, want %q", got, want)
	}
	inWork := []string{"CCS_PROFILE=work", "CLAUDE_CONFIG_DIR=" + acct}
	if got, want := e.claude(inWork, "-c"), out(acct, "work", "", "-c"); got != want {
		t.Errorf("claude inside work: got %q, want %q", got, want)
	}

	// An explicit profile started from inside gw must not inherit gw's token;
	// it would override work's own login.
	if got, want := e.run(inGw, "work"), out(acct, "work", "", ""); got != want {
		t.Errorf("ccs work inside gw: got %q, want %q", got, want)
	}
}
