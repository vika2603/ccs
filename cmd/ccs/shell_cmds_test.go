package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellInitZsh(t *testing.T) {
	home := t.TempDir()
	out, err := runCmd(t, home, "shell-init")
	if err != nil {
		t.Fatalf("shell-init: %v", err)
	}
	if !strings.Contains(out, "_CCS_SHELL_INIT_DONE") {
		t.Errorf("missing idempotent guard in output: %q", out[:minInt(200, len(out))])
	}
}

func TestUseEmitsStateChange(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	runCmd(t, home, "env", "set", "work", "ANTHROPIC_API_KEY=sk-secret")
	out, err := runCmd(t, home, "__shell_use", "work")
	if err != nil {
		t.Fatalf("use: %v", err)
	}
	if !strings.Contains(out, "export CLAUDE_CONFIG_DIR=") {
		t.Errorf("missing export line: %q", out)
	}
	if !strings.Contains(out, "export CCS_MANAGED_CCD=1") {
		t.Errorf("missing sentinel: %q", out)
	}
	if !strings.Contains(out, "export CCS_ENV_SIG=") {
		t.Errorf("missing CCS_ENV_SIG: %q", out)
	}
	wantPath := filepath.Join(home, ".ccs", "profiles", "work")
	if !strings.Contains(out, wantPath) {
		t.Errorf("path not present: %q", out)
	}
	// Profile env vars must not reach the shell.
	if strings.Contains(out, "ANTHROPIC_API_KEY") || strings.Contains(out, "sk-secret") {
		t.Errorf("profile env leaked into shell eval: %q", out)
	}
	if strings.Contains(out, "export CCS_MANAGED_VARS=") {
		t.Errorf("CCS_MANAGED_VARS no longer expected in output: %q", out)
	}
}

func TestUseEmptyUnsets(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	out, err := runCmd(t, home, "__shell_use")
	if err != nil {
		t.Fatalf("use empty: %v", err)
	}
	if !strings.Contains(out, "unset CCS_MANAGED_VARS CCS_ENV_SIG CLAUDE_CONFIG_DIR CCS_MANAGED_CCD") {
		t.Errorf("missing unset line: %q", out)
	}
}

func TestShellHookFastPathWhenSigMatches(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	useOut, err := runCmd(t, home, "__shell_use", "work")
	if err != nil {
		t.Fatalf("__shell_use work: %v", err)
	}
	// Extract CCS_ENV_SIG value from the emitted output: export CCS_ENV_SIG='work:NNNN'
	sig := extractExport(useOut, "CCS_ENV_SIG")
	if sig == "" {
		t.Fatalf("could not find CCS_ENV_SIG in output: %q", useOut)
	}
	// Simulate the shell having already eval'd the block: set CCS_ENV_SIG in the env.
	os.Setenv("CCS_ENV_SIG", sig)
	defer os.Unsetenv("CCS_ENV_SIG")
	hookOut, err := runCmd(t, home, "__shell_hook")
	if err != nil {
		t.Fatalf("__shell_hook: %v", err)
	}
	if strings.TrimSpace(hookOut) != "" {
		t.Errorf("hook should be silent when sig matches, got: %q", hookOut)
	}
}

func TestShellHookEmitsWhenSigMissing(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	runCmd(t, home, "__shell_use", "work")
	os.Unsetenv("CCS_ENV_SIG")
	os.Unsetenv("CLAUDE_CONFIG_DIR")
	os.Unsetenv("CCS_MANAGED_CCD")
	hookOut, err := runCmd(t, home, "__shell_hook")
	if err != nil {
		t.Fatalf("__shell_hook: %v", err)
	}
	if !strings.Contains(hookOut, "export CLAUDE_CONFIG_DIR=") {
		t.Errorf("hook should emit full block when sig missing, got: %q", hookOut)
	}
	if !strings.Contains(hookOut, "export CCS_ENV_SIG=") {
		t.Errorf("hook should set CCS_ENV_SIG, got: %q", hookOut)
	}
}

func TestShellHookBailsIfUserOwnsCCD(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	runCmd(t, home, "__shell_use", "work")
	os.Setenv("CLAUDE_CONFIG_DIR", "/user/owned")
	os.Unsetenv("CCS_MANAGED_CCD")
	defer os.Unsetenv("CLAUDE_CONFIG_DIR")
	hookOut, err := runCmd(t, home, "__shell_hook")
	if err != nil {
		t.Fatalf("__shell_hook: %v", err)
	}
	if strings.TrimSpace(hookOut) != "" {
		t.Errorf("hook should be silent when user owns CCD, got: %q", hookOut)
	}
}

func extractExport(s, name string) string {
	_, rest, ok := strings.Cut(s, "export "+name+"='")
	if !ok {
		return ""
	}
	val, _, ok := strings.Cut(rest, "'")
	if !ok {
		return ""
	}
	return val
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
