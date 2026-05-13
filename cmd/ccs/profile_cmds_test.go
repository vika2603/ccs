package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	os.Setenv("HOME", home)
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestInitNewLs(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ccs", "shared")); err != nil {
		t.Fatalf("shared missing: %v", err)
	}
	if _, err := runCmd(t, home, "new", "work"); err != nil {
		t.Fatalf("new: %v", err)
	}
	out, err := runCmd(t, home, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "work") {
		t.Errorf("ls output missing work: %q", out)
	}
}

func TestPathPrintsProfileDir(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	out, err := runCmd(t, home, "path", "work")
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	want := filepath.Join(home, ".ccs", "profiles", "work")
	if !strings.Contains(out, want) {
		t.Errorf("path output %q does not contain %q", out, want)
	}
}

func TestCloneProfile(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "src")

	// Write an isolated file into the source profile.
	srcDir := filepath.Join(home, ".ccs", "profiles", "src")
	if err := os.WriteFile(filepath.Join(srcDir, ".claude.json"), []byte(`{"user":"test"}`), 0o644); err != nil {
		t.Fatalf("write isolated file: %v", err)
	}

	if _, err := runCmd(t, home, "clone", "src", "dst"); err != nil {
		t.Fatalf("clone: %v", err)
	}

	// Destination should exist and contain the cloned isolated file.
	dstFile := filepath.Join(home, ".ccs", "profiles", "dst", ".claude.json")
	b, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read cloned file: %v", err)
	}
	if string(b) != `{"user":"test"}` {
		t.Errorf("cloned content mismatch: %s", b)
	}

	// Shared fields should be symlinks.
	dstSkills := filepath.Join(home, ".ccs", "profiles", "dst", "skills")
	info, err := os.Lstat(dstSkills)
	if err != nil {
		t.Fatalf("lstat skills: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected skills to be a symlink")
	}

	// ls should show both profiles.
	out, err := runCmd(t, home, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(out, "dst") || !strings.Contains(out, "src") {
		t.Errorf("ls missing profiles: %q", out)
	}
}

func TestCloneNonexistentSource(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	_, err := runCmd(t, home, "clone", "nope", "dst")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitPreservesExistingConfig(t *testing.T) {
	home := t.TempDir()
	if _, err := runCmd(t, home, "init"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	cfg := filepath.Join(home, ".ccs", "config.toml")
	if err := os.WriteFile(cfg, []byte("version = 1\n[fields]\nshared = [\"skills\", \"custom\"]\n"), 0o644); err != nil {
		t.Fatalf("edit config: %v", err)
	}
	out, err := runCmd(t, home, "init")
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(b), "\"custom\"") {
		t.Fatalf("config edit was overwritten: %s", b)
	}
	if !strings.Contains(out, "config.toml exists; leaving it.") {
		t.Fatalf("missing idempotency note: %q", out)
	}
}
