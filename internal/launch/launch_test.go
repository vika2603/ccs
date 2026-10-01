package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestEnv(t *testing.T) {
	in := []string{
		"PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/old", "CCS_PROFILE=outer",
		"ANTHROPIC_AUTH_TOKEN=t", "ANTHROPIC_BASE_URL=https://gw", "ANTHROPIC_MODEL=m",
		"CLAUDE_CODE_OAUTH_TOKEN=o", "CLAUDE_CODE_EFFORT_LEVEL=high",
	}
	kept := []string{"PATH=/usr/bin", "CLAUDE_CODE_EFFORT_LEVEL=high"}
	if got, want := Env(in, "work", "/new"), append(slices.Clone(kept), "CLAUDE_CONFIG_DIR=/new", "CCS_PROFILE=work"); !slices.Equal(got, want) {
		t.Errorf("login profile:\n got %v\nwant %v", got, want)
	}
	if got, want := Env(in, "gw", ""), append(slices.Clone(kept), "CCS_PROFILE=gw"); !slices.Equal(got, want) {
		t.Errorf("profile on ~/.claude:\n got %v\nwant %v", got, want)
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
