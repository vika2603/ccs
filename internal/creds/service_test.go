package creds

import (
	"path/filepath"
	"testing"
)

func TestServiceNameDefaultPathUsesBareServiceName(t *testing.T) {
	defaultPath := filepath.Clean("/Users/a/.claude")
	got, err := ServiceName(defaultPath, defaultPath)
	if err != nil {
		t.Fatalf("ServiceName: %v", err)
	}
	if got != "Claude Code-credentials" {
		t.Fatalf("got %q", got)
	}
}

func TestServiceNameNamedProfileUsesSuffix(t *testing.T) {
	defaultPath := filepath.Clean("/Users/a/.claude")
	got, err := ServiceName("/Users/a/.ccs/profiles/work", defaultPath)
	if err != nil {
		t.Fatalf("ServiceName: %v", err)
	}
	want := "Claude Code-credentials-" + sha8("/Users/a/.ccs/profiles/work")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestServiceNameCanonicalizesRelativeInput(t *testing.T) {
	home := t.TempDir()
	defaultPath := filepath.Join(home, ".claude")
	in := filepath.Join(home, ".ccs", "profiles", "..", "profiles", "work")
	got, err := ServiceName(in, defaultPath)
	if err != nil {
		t.Fatalf("ServiceName: %v", err)
	}
	want := "Claude Code-credentials-" + sha8(filepath.Join(home, ".ccs", "profiles", "work"))
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestServiceNameTrailingSlashHashesLikeCleanPath(t *testing.T) {
	defaultPath := filepath.Clean("/Users/a/.claude")
	a, _ := ServiceName("/Users/a/.ccs/profiles/work", defaultPath)
	b, _ := ServiceName("/Users/a/.ccs/profiles/work/", defaultPath)
	if a != b {
		t.Fatalf("expected same service name, got %q and %q", a, b)
	}
}
