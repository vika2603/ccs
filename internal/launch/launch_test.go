package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestEnvSetsOrRemovesConfigDir(t *testing.T) {
	in := []string{"PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/old"}
	if got := Env(in, "/new"); !slices.Equal(got, []string{"PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/new"}) {
		t.Errorf("Env(/new) = %v", got)
	}
	if got := Env(in, ""); !slices.Equal(got, []string{"PATH=/usr/bin"}) {
		t.Errorf("Env(\"\") = %v", got)
	}
}

func TestResolveSkipsShimDir(t *testing.T) {
	shimDir, realDir := t.TempDir(), t.TempDir()
	for _, dir := range []string{shimDir, realDir} {
		if err := os.WriteFile(filepath.Join(dir, "widget"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+realDir)
	got, err := Resolve("widget", shimDir)
	if err != nil || got != filepath.Join(realDir, "widget") {
		t.Errorf("got %q %v, want the binary in %s", got, err, realDir)
	}
	if _, err := Resolve("widget", realDir); err != nil {
		t.Errorf("an unskipped dir should still be searched: %v", err)
	}
	t.Setenv("PATH", shimDir)
	if _, err := Resolve("widget", shimDir); !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("only the shim on PATH should be not found: %v", err)
	}
}
