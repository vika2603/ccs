package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportProfileDefaultExcludesRuntimeAndTransient(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")

	profileDir := filepath.Join(home, ".ccs", "profiles", "work")
	os.WriteFile(filepath.Join(home, ".ccs", "shared", "skills", "a.md"), []byte("A"), 0o644)
	os.MkdirAll(filepath.Join(profileDir, "projects"), 0o755)
	os.WriteFile(filepath.Join(profileDir, "projects", "p.txt"), []byte("P"), 0o644)
	os.MkdirAll(filepath.Join(profileDir, "cache"), 0o755)
	os.WriteFile(filepath.Join(profileDir, "cache", "c.bin"), []byte("C"), 0o644)

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "work.tar.gz")
	if _, err := runCmd(t, home, "export", "work", "-o", outFile); err != nil {
		t.Fatalf("export: %v", err)
	}

	contents := listTarEntries(t, outFile)
	hasManifest := false
	hasSkill := false
	for _, n := range contents {
		if n == "manifest.json" {
			hasManifest = true
		}
		if strings.HasSuffix(n, "skills/a.md") {
			hasSkill = true
		}
		if strings.Contains(n, "profile/projects/") {
			t.Errorf("default export must not include isolated runtime data, found %q", n)
		}
		if strings.Contains(n, "profile/cache") || strings.Contains(n, "/cache/") {
			t.Errorf("default export must not include transient cache, found %q", n)
		}
	}
	if !hasManifest || !hasSkill {
		t.Errorf("archive contents wrong: %v", contents)
	}
}

func TestExportDefaultExcludesClaudeJSON(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")

	profileDir := filepath.Join(home, ".ccs", "profiles", "work")
	if err := os.WriteFile(filepath.Join(profileDir, ".claude.json"), []byte(`{"oauthAccount":{"email":"u@example.com"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "work.tar.gz")
	if _, err := runCmd(t, home, "export", "work", "-o", outFile); err != nil {
		t.Fatalf("export: %v", err)
	}

	for _, n := range listTarEntries(t, outFile) {
		if strings.HasSuffix(n, "/.claude.json") || n == "profile/.claude.json" {
			t.Fatalf("default export must not include .claude.json (account identity), found %q", n)
		}
	}
}

func TestExportFullIncludesClaudeJSON(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")

	profileDir := filepath.Join(home, ".ccs", "profiles", "work")
	want := []byte(`{"oauthAccount":{"email":"u@example.com"},"userID":"abc"}`)
	if err := os.WriteFile(filepath.Join(profileDir, ".claude.json"), want, 0o600); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "work-full.tar.gz")
	if _, err := runCmd(t, home, "export", "work", "--full", "-o", outFile); err != nil {
		t.Fatalf("export --full: %v", err)
	}

	got, found := readTarEntry(t, outFile, "profile/.claude.json")
	if !found {
		t.Fatalf("--full export must include profile/.claude.json")
	}
	if string(got) != string(want) {
		t.Fatalf(".claude.json bytes mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func readTarEntry(t *testing.T, p, name string) ([]byte, bool) {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, false
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Name == name {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return b, true
		}
	}
}

func TestExportProfileFullIncludesIsolatedRuntime(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")

	profileDir := filepath.Join(home, ".ccs", "profiles", "work")
	os.MkdirAll(filepath.Join(profileDir, "projects"), 0o755)
	os.WriteFile(filepath.Join(profileDir, "projects", "p.txt"), []byte("P"), 0o644)
	os.MkdirAll(filepath.Join(profileDir, "cache"), 0o755)
	os.WriteFile(filepath.Join(profileDir, "cache", "c.bin"), []byte("C"), 0o644)

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "work-full.tar.gz")
	if _, err := runCmd(t, home, "export", "work", "--full", "-o", outFile); err != nil {
		t.Fatalf("export --full: %v", err)
	}

	contents := listTarEntries(t, outFile)
	hasProjects := false
	for _, n := range contents {
		if strings.HasSuffix(n, "profile/projects/p.txt") {
			hasProjects = true
		}
		if strings.Contains(n, "/cache/") {
			t.Errorf("--full must still exclude transient cache, found %q", n)
		}
	}
	if !hasProjects {
		t.Errorf("--full export missing projects/: %v", contents)
	}
}

func TestExportInteractiveRejectsNonTTY(t *testing.T) {
	home := t.TempDir()
	runCmd(t, home, "init")
	runCmd(t, home, "new", "work")
	_, err := runCmd(t, home, "export", "work", "-i")
	if err == nil {
		t.Fatalf("expected -i on non-TTY to error")
	}
	if !strings.Contains(err.Error(), "TTY") {
		t.Fatalf("error should mention TTY; got %q", err.Error())
	}
}

func listTarEntries(t *testing.T, p string) []string {
	f, _ := os.Open(p)
	defer f.Close()
	gz, _ := gzip.NewReader(f)
	tr := tar.NewReader(gz)
	var out []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, h.Name)
		if h.Name == "manifest.json" {
			var m map[string]any
			b, _ := io.ReadAll(tr)
			json.Unmarshal(b, &m)
			if m["profile"] != "work" {
				t.Errorf("manifest profile wrong: %v", m["profile"])
			}
		}
	}
	return out
}
