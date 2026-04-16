package main

import (
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
	wantPath := filepath.Join(home, ".ccs", "profiles", "work")
	if !strings.Contains(out, wantPath) {
		t.Errorf("path not present: %q", out)
	}
}

func TestUseEmptyUnsets(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	out, err := runCmd(t, home, "__shell_use")
	if err != nil {
		t.Fatalf("use empty: %v", err)
	}
	if !strings.Contains(out, "unset CLAUDE_CONFIG_DIR CCS_MANAGED_CCD") {
		t.Errorf("missing unset line: %q", out)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
