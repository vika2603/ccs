package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Version != 1 {
		t.Errorf("version: %d", c.Version)
	}
	if !containsString(c.Fields.Shared, "skills") {
		t.Errorf("shared missing skills: %v", c.Fields.Shared)
	}
	if !containsString(c.Fields.Isolated, "projects") {
		t.Errorf("isolated missing projects: %v", c.Fields.Isolated)
	}
	if !containsString(c.Fields.Transient, "cache") {
		t.Errorf("transient missing cache: %v", c.Fields.Transient)
	}
}

func TestLoadMissingReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	c, err := Load(filepath.Join(dir, "nope.toml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(c, Default()) {
		t.Errorf("expected default config")
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	in := Default()
	in.Fields.Shared = append(in.Fields.Shared, "custom")
	if err := Save(p, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch")
	}
}

func TestLoadRejectsFutureVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	os.WriteFile(p, []byte("version = 99\n"), 0o644)
	if _, err := Load(p); err == nil {
		t.Fatalf("expected error for future version")
	}
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
