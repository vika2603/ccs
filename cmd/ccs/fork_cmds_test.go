package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmdWithInput(t *testing.T, home, stdin string, args ...string) (string, error) {
	t.Helper()
	os.Setenv("HOME", home)
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Wrap in bufio.Reader so PromptConflict takes the scripted-input
	// fast path instead of failing closed for non-TTY input.
	cmd.SetIn(bufio.NewReader(strings.NewReader(stdin)))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestForkAndStatus(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	os.WriteFile(filepath.Join(home, ".ccs", "shared", "skills", "a.md"), []byte("A"), 0o644)
	if _, err := runCmd(t, home, "fork", "skills", "work"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	out, _ := runCmd(t, home, "status", "work")
	if !strings.Contains(out, "forked") {
		t.Errorf("status output missing forked: %q", out)
	}
	if !strings.Contains(out, "shared/:") {
		t.Errorf("status output missing shared/ summary: %q", out)
	}
	if !strings.Contains(out, "skills/") {
		t.Errorf("status output missing shared/skills entry: %q", out)
	}
}

func TestShareConflictPrompts(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	os.Remove(filepath.Join(home, ".ccs", "profiles", "work", "skills"))
	os.MkdirAll(filepath.Join(home, ".ccs", "profiles", "work", "skills"), 0o755)
	os.WriteFile(filepath.Join(home, ".ccs", "profiles", "work", "skills", "local.md"), []byte("L"), 0o644)
	os.WriteFile(filepath.Join(home, ".ccs", "shared", "skills", "shared.md"), []byte("S"), 0o644)

	out, err := runCmdWithInput(t, home, "o\n", "share", "skills", "work")
	if err != nil {
		t.Fatalf("share: %v", err)
	}
	if !strings.Contains(out, "overwrite/abort/diff") {
		t.Fatalf("missing conflict prompt: %q", out)
	}
}
