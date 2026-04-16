package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vika2603/ccs/internal/archive"
)

func TestRestoreRoundTrip(t *testing.T) {
	src := t.TempDir()
	runCmd(t, src, "init")
	runCmd(t, src, "new", "work")
	os.WriteFile(filepath.Join(src, ".ccs", "shared", "skills", "a.md"), []byte("A"), 0o644)
	out := filepath.Join(t.TempDir(), "work.tar.gz")
	runCmd(t, src, "export", "work", "-o", out)

	dst := t.TempDir()
	runCmd(t, dst, "init")
	if _, err := runCmd(t, dst, "restore", out); err != nil {
		t.Fatalf("restore: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dst, ".ccs", "shared", "skills", "a.md"))
	if err != nil || string(b) != "A" {
		t.Errorf("restored content missing: %v / %q", err, b)
	}
}

func TestRestoreRefusesCrossPlatform(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "cross.tar.gz")
	other := "linux"
	if restorePlatformOverride == "linux" {
		other = "darwin"
	}
	m := archive.Manifest{Profile: "work", SourcePlatform: other}
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if err := archive.WriteMinimalManifestTar(gz, m); err != nil {
		t.Fatal(err)
	}
	gz.Close()
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	runCmd(t, home, "init")
	out, err := runCmd(t, home, "restore", archivePath)
	if err == nil {
		t.Fatalf("expected refusal, got output: %q", out)
	}
	wantFragment := "cross-platform"
	if !strings.Contains(err.Error(), wantFragment) {
		t.Errorf("error message should mention %q, got: %v", wantFragment, err)
	}
	if _, mErr := json.Marshal(m); mErr != nil {
		t.Fatal(mErr)
	}
}
