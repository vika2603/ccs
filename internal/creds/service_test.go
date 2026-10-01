package creds

import (
	"path/filepath"
	"testing"
)

// The expected value is the service Claude Code itself uses for this
// CLAUDE_CONFIG_DIR, so a change in hashing would orphan stored logins.
func TestServiceNameMatchesClaudeCode(t *testing.T) {
	got, err := serviceName("/Users/a/.ccs/accounts/work")
	if err != nil {
		t.Fatal(err)
	}
	// printf %s /Users/a/.ccs/accounts/work | shasum -a 256 | cut -c1-8
	if want := "Claude Code-credentials-da61693a"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestServiceNameCleansPath(t *testing.T) {
	a, _ := serviceName("/Users/a/.ccs/accounts/work")
	b, _ := serviceName("/Users/a/.ccs/accounts/../accounts/work/")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
	rel, _ := serviceName(filepath.Join(".", "x"))
	abs, _ := filepath.Abs("x")
	if want, _ := serviceName(abs); rel != want {
		t.Fatalf("relative path not made absolute: %q != %q", rel, want)
	}
}
