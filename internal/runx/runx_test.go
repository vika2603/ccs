package runx

import (
	"os"
	"strings"
	"testing"
)

func TestBuildEnvInsertsConfigDir(t *testing.T) {
	env := BuildEnv([]string{"PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/old"}, "/new")
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
	env := BuildEnv([]string{"PATH=/usr/bin"}, "/new")
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
