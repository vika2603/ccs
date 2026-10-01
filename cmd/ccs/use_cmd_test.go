package main

import (
	"strings"
	"testing"
)

func TestUseSetsAndClearsActiveProfile(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	if _, err := runCmd(t, home, "use", "work"); err != nil {
		t.Fatalf("use: %v", err)
	}
	if out, _ := runCmd(t, home, "ls"); !strings.Contains(out, "* work") {
		t.Errorf("work should be active: %q", out)
	}
	if _, err := runCmd(t, home, "use", "--none"); err != nil {
		t.Fatalf("use --none: %v", err)
	}
	if out, _ := runCmd(t, home, "ls"); strings.Contains(out, "* work") {
		t.Errorf("no profile should be active after use --none: %q", out)
	}
	if _, err := runCmd(t, home, "use", "ghost"); err == nil {
		t.Error("use should reject a missing profile")
	}
	if _, err := runCmd(t, home, "use", "--none", "work"); err == nil || !strings.Contains(err.Error(), "--none takes no profile name") {
		t.Errorf("use --none should reject a profile name with a clear error, got %v", err)
	}
}
