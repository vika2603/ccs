package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vika2603/ccs/internal/config"
)

func TestClassifyAppendsToCategory(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	if _, err := runCmd(t, home, "classify", "plans", "isolated"); err != nil {
		t.Fatalf("classify: %v", err)
	}
	cfg, err := config.Load(filepath.Join(home, ".ccs", "config.toml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	found := false
	for _, v := range cfg.Isolated {
		if v == "plans" {
			found = true
		}
	}
	if !found {
		t.Fatalf("plans not appended to isolated: %+v", cfg.Isolated)
	}
}

func TestClassifyRejectsDuplicate(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	_, err := runCmd(t, home, "classify", "skills", "shared")
	if err == nil {
		t.Fatalf("expected error on duplicate classification")
	}
	if !strings.Contains(err.Error(), "already classified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClassifyRejectsInvalidCategory(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	_, err := runCmd(t, home, "classify", "something", "bogus")
	if err == nil {
		t.Fatalf("expected error for invalid category")
	}
}
