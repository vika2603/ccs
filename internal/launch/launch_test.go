package launch

import (
	"os"
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

func TestResolveExecutable(t *testing.T) {
	path, err := os.Executable()
	if err != nil {
		t.Skip("no executable info")
	}
	got, err := ResolveSkipping([]string{path}, nil)
	if err != nil || got != path {
		t.Errorf("got %q err %v", got, err)
	}
}

// TestResolveSkippingSkipsShimDir ensures the shim dir (first in PATH) is
// ignored and the real binary in a later entry is picked up. Models the
// ~/.ccs/bin scenario: shim named "claude" must not resolve to itself.
func TestResolveSkippingSkipsShimDir(t *testing.T) {
	shimDir := t.TempDir()
	realDir := t.TempDir()
	// Write executables to both directories. Content is arbitrary; mode 0755
	// is what ResolveSkipping looks for.
	for _, dir := range []string{shimDir, realDir} {
		if err := os.WriteFile(dir+"/widget", []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	os.Setenv("PATH", shimDir+":"+realDir)

	// Without skip: resolves to shim (first hit).
	got, err := ResolveSkipping([]string{"widget"}, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != shimDir+"/widget" {
		t.Errorf("without skip: got %q, want shim", got)
	}

	// With skip: resolves to real dir.
	got, err = ResolveSkipping([]string{"widget"}, []string{shimDir})
	if err != nil {
		t.Fatalf("resolve skipping: %v", err)
	}
	if got != realDir+"/widget" {
		t.Errorf("with skip: got %q, want %q", got, realDir+"/widget")
	}
}

// TestResolveSkippingAbsolutePath passes through absolute paths unchanged so
// callers that already know the binary location aren't re-resolved.
func TestResolveSkippingAbsolutePath(t *testing.T) {
	got, err := ResolveSkipping([]string{"/bin/sh"}, []string{"/anything"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != "/bin/sh" {
		t.Errorf("got %q, want /bin/sh", got)
	}
}
