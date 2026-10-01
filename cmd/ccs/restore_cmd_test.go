package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreWorksWithUnreadableConfig(t *testing.T) {
	t.Setenv("CCS_PASSPHRASE", "pw")
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	out := filepath.Join(t.TempDir(), "b.tar.gz")
	if _, err := runCmd(t, home, "backup", "-o", out); err != nil {
		t.Fatalf("backup: %v", err)
	}
	cfg := filepath.Join(home, ".ccs", "config.toml")
	if err := os.WriteFile(cfg, []byte("not = [valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCmd(t, home, "restore", "--force", out); err != nil {
		t.Fatalf("restore should recover from a broken config.toml: %v", err)
	}
	if _, err := runCmd(t, home, "ls"); err != nil {
		t.Errorf("config still unreadable after restore: %v", err)
	}
}
