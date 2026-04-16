package shell

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestRenderZsh(t *testing.T) {
	got := Render(Zsh)
	path := filepath.Join("testdata", "zsh.golden")
	if *update {
		os.WriteFile(path, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("zsh render mismatch.\n--- got ---\n%s\n--- want ---\n%s\n", got, want)
	}
}

func TestRenderBash(t *testing.T) {
	got := Render(Bash)
	path := filepath.Join("testdata", "bash.golden")
	if *update {
		os.WriteFile(path, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("bash render mismatch.\n--- got ---\n%s\n--- want ---\n%s\n", got, want)
	}
}

func TestDetectShell(t *testing.T) {
	cases := map[string]Shell{
		"/bin/zsh":            Zsh,
		"/usr/local/bin/bash": Bash,
	}
	for path, want := range cases {
		if got := Detect(path); got != want {
			t.Errorf("Detect(%q) = %v, want %v", path, got, want)
		}
	}
}
