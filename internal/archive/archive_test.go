package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackDereferencesSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	os.MkdirAll(real, 0o755)
	os.WriteFile(filepath.Join(real, "a.md"), []byte("A"), 0o644)
	profile := filepath.Join(dir, "profile")
	os.MkdirAll(profile, 0o755)
	os.Symlink(real, filepath.Join(profile, "skills"))

	tarPath := filepath.Join(dir, "out.tar.gz")
	p := PackOptions{
		ProfileDir:  profile,
		ProfileName: "work",
		Manifest:    Manifest{Version: 1, Profile: "work"},
	}
	if err := Pack(tarPath, p); err != nil {
		t.Fatalf("pack: %v", err)
	}
	files := listTar(t, tarPath)
	var found bool
	for _, f := range files {
		if strings.HasSuffix(f, "profile/skills/a.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("symlink contents not dereferenced: %v", files)
	}
	found = false
	for _, f := range files {
		if f == "manifest.json" {
			found = true
		}
	}
	if !found {
		t.Errorf("manifest.json missing")
	}
}

func TestUnpackRestoresTree(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "src")
	os.MkdirAll(filepath.Join(profile, "skills"), 0o755)
	os.WriteFile(filepath.Join(profile, "skills", "a.md"), []byte("A"), 0o644)
	tarPath := filepath.Join(dir, "out.tar.gz")
	if err := Pack(tarPath, PackOptions{
		ProfileDir:  profile,
		ProfileName: "work",
		Manifest:    Manifest{Version: 1, Profile: "work"},
	}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "dest")
	os.Mkdir(dest, 0o755)
	m, err := Unpack(tarPath, dest)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if m.Profile != "work" {
		t.Errorf("manifest profile: %q", m.Profile)
	}
	b, _ := os.ReadFile(filepath.Join(dest, "profile", "skills", "a.md"))
	if string(b) != "A" {
		t.Errorf("content: %q", b)
	}
}

func TestPackIncludesFileShapedSharedEntry(t *testing.T) {
	dir := t.TempDir()
	sharedRoot := filepath.Join(dir, "shared")
	os.MkdirAll(sharedRoot, 0o755)
	claudePath := filepath.Join(sharedRoot, "CLAUDE.md")
	os.WriteFile(claudePath, []byte("memory"), 0o644)
	profile := filepath.Join(dir, "profile")
	os.MkdirAll(profile, 0o755)
	os.Symlink(claudePath, filepath.Join(profile, "CLAUDE.md"))

	tarPath := filepath.Join(dir, "out.tar.gz")
	if err := Pack(tarPath, PackOptions{
		ProfileDir:     profile,
		ProfileName:    "work",
		ProfileEntries: []string{"CLAUDE.md"},
		SharedPaths:    map[string]string{"CLAUDE.md": claudePath},
		Manifest:       Manifest{Version: 1, Profile: "work"},
	}); err != nil {
		t.Fatalf("pack: %v", err)
	}
	names := listTar(t, tarPath)
	var profileEntry, sharedEntry bool
	for _, n := range names {
		if n == "profile/CLAUDE.md" {
			profileEntry = true
		}
		if n == "shared/CLAUDE.md" {
			sharedEntry = true
		}
	}
	if !profileEntry {
		t.Errorf("profile/CLAUDE.md missing from archive: %v", names)
	}
	if !sharedEntry {
		t.Errorf("shared/CLAUDE.md missing from archive: %v", names)
	}
}

func TestWriteMinimalManifestTarRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMinimalManifestTar(&buf, Manifest{Version: 1, Profile: "work", SourcePlatform: "linux"}); err != nil {
		t.Fatalf("WriteMinimalManifestTar: %v", err)
	}
	tr := tar.NewReader(&buf)
	h, err := tr.Next()
	if err != nil {
		t.Fatalf("tar next: %v", err)
	}
	if h.Name != "manifest.json" {
		t.Fatalf("first entry %q, want manifest.json", h.Name)
	}
}

func listTar(t *testing.T, p string) []string {
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
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
	return names
}
