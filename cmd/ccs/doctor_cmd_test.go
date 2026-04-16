package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsBrokenSymlink(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	os.RemoveAll(filepath.Join(home, ".ccs", "shared", "skills"))
	out, err := runCmd(t, home, "doctor")
	if err == nil {
		t.Fatalf("expected non-zero exit when findings exist; output: %q", out)
	}
	if !strings.Contains(out, "broken-symlink") {
		t.Errorf("doctor output: %q", out)
	}
}

func TestDoctorExitsZeroWhenClean(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	if _, err := runCmd(t, home, "doctor"); err != nil {
		t.Errorf("clean tree should exit zero: %v", err)
	}
}
