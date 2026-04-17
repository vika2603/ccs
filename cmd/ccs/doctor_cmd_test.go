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

func TestDoctorReportsOrphanEnvFile(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	// Drop an env file for a profile that doesn't exist.
	ghost := filepath.Join(home, ".ccs", "env", "ghost.toml")
	if err := os.WriteFile(ghost, []byte("[env]\nFOO = \"bar\"\n"), 0o600); err != nil {
		t.Fatalf("write orphan: %v", err)
	}
	out, err := runCmd(t, home, "doctor")
	if err == nil {
		t.Fatalf("expected non-zero when orphan present; output: %q", out)
	}
	if !strings.Contains(out, "orphan-env-file") {
		t.Errorf("expected orphan-env-file in output: %q", out)
	}
	if !strings.Contains(out, "ghost") {
		t.Errorf("expected ghost in output: %q", out)
	}
}

func TestRmSyncsEnvFile(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	runCmd(t, home, "env", "set", "work", "FOO=bar")
	envFile := filepath.Join(home, ".ccs", "env", "work.toml")
	if _, err := os.Stat(envFile); err != nil {
		t.Fatalf("env file should exist: %v", err)
	}
	if _, err := runCmd(t, home, "rm", "work", "-y"); err != nil {
		t.Fatalf("rm work: %v", err)
	}
	if _, err := os.Stat(envFile); !os.IsNotExist(err) {
		t.Errorf("env file should be removed after rm; err=%v", err)
	}
}

func TestEnvEditSuccess(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	runCmd(t, home, "env", "set", "work", "KEEP=original")
	// EDITOR that overwrites the file with a valid TOML appending a new key.
	os.Setenv("EDITOR", `sh -c 'printf "[env]\nKEEP = \"original\"\nADDED = \"yes\"\n" > "$1"' sh`)
	defer os.Unsetenv("EDITOR")
	if _, err := runCmd(t, home, "env", "edit", "work"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	out, err := runCmd(t, home, "env", "ls", "work", "--show-values")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "ADDED=yes") {
		t.Errorf("ADDED key missing after edit: %q", out)
	}
	if !strings.Contains(out, "KEEP=original") {
		t.Errorf("KEEP key missing after edit: %q", out)
	}
}

func TestEnvEditRestoresOnInvalidEdit(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	runCmd(t, home, "env", "set", "work", "KEEP=original")
	// EDITOR that writes garbage TOML.
	os.Setenv("EDITOR", `sh -c 'printf "this is not valid toml [[[\n" > "$1"' sh`)
	defer os.Unsetenv("EDITOR")
	if _, err := runCmd(t, home, "env", "edit", "work"); err == nil {
		t.Fatalf("edit should have failed validation")
	}
	// Original content must be restored.
	out, err := runCmd(t, home, "env", "ls", "work", "--show-values")
	if err != nil {
		t.Fatalf("ls after restore: %v", err)
	}
	if !strings.Contains(out, "KEEP=original") {
		t.Errorf("original KEEP not restored after bad edit: %q", out)
	}
}

func TestEnvEditRemovesFileIfItDidNotExist(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	// Never ran env set, so the file doesn't exist yet.
	os.Setenv("EDITOR", `sh -c 'printf "garbage [[\n" > "$1"' sh`)
	defer os.Unsetenv("EDITOR")
	if _, err := runCmd(t, home, "env", "edit", "work"); err == nil {
		t.Fatalf("edit should have failed validation")
	}
	path := filepath.Join(home, ".ccs", "env", "work.toml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("env file should be removed when edit fails and file didn't exist; err=%v", err)
	}
}

func TestMvSyncsEnvFile(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	runCmd(t, home, "env", "set", "work", "FOO=bar")
	if _, err := runCmd(t, home, "mv", "work", "job"); err != nil {
		t.Fatalf("mv: %v", err)
	}
	oldPath := filepath.Join(home, ".ccs", "env", "work.toml")
	newPath := filepath.Join(home, ".ccs", "env", "job.toml")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("old env file should be gone; err=%v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("new env file should exist: %v", err)
	}
}
