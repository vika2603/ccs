package main

import (
	"strings"
	"testing"
)

func TestShellInitPrintsCompletion(t *testing.T) {
	home := t.TempDir()
	for _, kind := range []string{"zsh", "bash"} {
		out, err := runCmd(t, home, "shell-init", "--shell", kind)
		if err != nil {
			t.Fatalf("shell-init --shell %s: %v", kind, err)
		}
		if !strings.Contains(out, "__start_ccs") && !strings.Contains(out, "_ccs") {
			t.Errorf("%s output does not look like a completion script", kind)
		}
		if strings.Contains(out, "CLAUDE_CONFIG_DIR") {
			t.Errorf("%s output should not touch CLAUDE_CONFIG_DIR", kind)
		}
	}
}

func TestUseAndUnuseUpdateActiveProfile(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	if _, err := runCmd(t, home, "use", "work"); err != nil {
		t.Fatalf("use: %v", err)
	}
	out, _ := runCmd(t, home, "ls")
	if !strings.Contains(out, "* work") {
		t.Errorf("work should be active: %q", out)
	}
	if _, err := runCmd(t, home, "unuse"); err != nil {
		t.Fatalf("unuse: %v", err)
	}
	out, _ = runCmd(t, home, "ls")
	if strings.Contains(out, "* work") {
		t.Errorf("work should be inactive after unuse: %q", out)
	}
	if _, err := runCmd(t, home, "use", "ghost"); err == nil {
		t.Error("use should reject a missing profile")
	}
}

func TestShellInitZshGuardsCompdef(t *testing.T) {
	out, err := runCmd(t, t.TempDir(), "shell-init", "--shell", "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "if (( $+functions[compdef] )); then\n") {
		t.Errorf("zsh output must guard compdef for shells without compinit: %q", out[:min(80, len(out))])
	}
}
