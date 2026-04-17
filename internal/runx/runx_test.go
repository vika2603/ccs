package runx

import (
	"os"
	"strings"
	"testing"
)

func TestBuildEnvInsertsConfigDir(t *testing.T) {
	env := BuildEnv([]string{"PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/old"}, "/new", nil)
	var got string
	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			got = e
		}
	}
	if got != "CLAUDE_CONFIG_DIR=/new" {
		t.Errorf("got %q", got)
	}
}

func TestBuildEnvAddsWhenAbsent(t *testing.T) {
	env := BuildEnv([]string{"PATH=/usr/bin"}, "/new", nil)
	found := false
	for _, e := range env {
		if e == "CLAUDE_CONFIG_DIR=/new" {
			found = true
		}
	}
	if !found {
		t.Errorf("CLAUDE_CONFIG_DIR not added: %v", env)
	}
}

func TestBuildEnvOverlaysProfileEnv(t *testing.T) {
	in := []string{"PATH=/usr/bin", "FOO=old", "KEEP=ok"}
	env := BuildEnv(in, "/new", map[string]string{
		"FOO":                "new",
		"ANTHROPIC_BASE_URL": "https://example.com",
	})
	want := map[string]string{
		"PATH":               "/usr/bin",
		"FOO":                "new",
		"KEEP":               "ok",
		"ANTHROPIC_BASE_URL": "https://example.com",
		"CLAUDE_CONFIG_DIR":  "/new",
	}
	got := map[string]string{}
	for _, e := range env {
		name, val, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		got[name] = val
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %q: got %q want %q", k, got[k], v)
		}
	}
	count := 0
	for _, e := range env {
		if strings.HasPrefix(e, "FOO=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("FOO appeared %d times, want 1 (overlay should replace, not duplicate)", count)
	}
}

func TestResolveExecutable(t *testing.T) {
	path, err := os.Executable()
	if err != nil {
		t.Skip("no executable info")
	}
	got, err := Resolve([]string{path})
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
	got, err := Resolve([]string{"widget"})
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
