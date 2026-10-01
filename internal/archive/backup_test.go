package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestBackupRoundTrip builds a fake ~/.ccs tree with one profile linked into
// shared and one profile that has forked some content, runs PackBackup on it,
// unpacks into a different root, and verifies contents and symlink targets.
func TestBackupRoundTrip(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.toml"), []byte("version = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sharedCLAUDE := filepath.Join(src, "shared", "CLAUDE.md")
	if err := os.WriteFile(sharedCLAUDE, []byte("shared-claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillsDir := filepath.Join(src, "shared", "skills", "hello")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	profA := filepath.Join(src, "profiles", "a")
	profB := filepath.Join(src, "profiles", "b")
	for _, d := range []string{profA, profB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// profile a: CLAUDE.md symlinked to shared/, its own .claude.json, and a
	// cache dir that must be excluded.
	if err := os.Symlink(filepath.Join(src, "shared", "CLAUDE.md"), filepath.Join(profA, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profA, ".claude.json"), []byte(`{"p":"a"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(profA, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profA, "cache", "c.txt"), []byte("drop me"), 0o644); err != nil {
		t.Fatal(err)
	}
	// profile b: a fork of CLAUDE.md with local content.
	if err := os.WriteFile(filepath.Join(profB, "CLAUDE.md"), []byte("forked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	envDir := filepath.Join(src, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "a.toml"), []byte("[env]\nFOO=\"bar\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "backup.tar.gz")
	manifest := BackupManifest{
		Version:        1,
		Type:           BackupType,
		SourcePlatform: runtime.GOOS,
		Active:         "a",
		Profiles:       []string{"a", "b"},
	}
	opts := BackupPackOptions{
		CCSRoot:           src,
		Profiles:          []string{"a", "b"},
		PerProfileExclude: []string{"cache"},
		ConfigPath:        filepath.Join(src, "config.toml"),
		EnvDir:            envDir,
		SharedDir:         filepath.Join(src, "shared"),
		Credentials:       []byte("dummy-encrypted"),
		Manifest:          manifest,
	}
	if err := PackBackup(outPath, opts); err != nil {
		t.Fatalf("PackBackup: %v", err)
	}

	dst := t.TempDir()
	got, err := UnpackBackup(outPath, dst)
	if err != nil {
		t.Fatalf("UnpackBackup: %v", err)
	}
	if got.Type != BackupType {
		t.Errorf("manifest type = %q want %q", got.Type, BackupType)
	}
	if got.Active != "a" {
		t.Errorf("active = %q want a", got.Active)
	}
	if len(got.Profiles) != 2 {
		t.Errorf("profiles = %v", got.Profiles)
	}

	// config.toml round-trip
	if b, err := os.ReadFile(filepath.Join(dst, "config.toml")); err != nil || string(b) != "version = 2\n" {
		t.Errorf("config.toml: %v / %q", err, b)
	}

	// shared/CLAUDE.md real file
	if b, err := os.ReadFile(filepath.Join(dst, "shared", "CLAUDE.md")); err != nil || string(b) != "shared-claude\n" {
		t.Errorf("shared/CLAUDE.md: %v / %q", err, b)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "shared", "skills", "hello", "SKILL.md")); err != nil || string(b) != "hi" {
		t.Errorf("shared skills: %v / %q", err, b)
	}

	// profile a CLAUDE.md must be a symlink with relative target
	profACLAUDE := filepath.Join(dst, "profiles", "a", "CLAUDE.md")
	info, err := os.Lstat(profACLAUDE)
	if err != nil {
		t.Fatalf("lstat profile a CLAUDE.md: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("profile a CLAUDE.md is not a symlink")
	}
	target, err := os.Readlink(profACLAUDE)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "../../shared/CLAUDE.md" {
		t.Errorf("symlink target = %q want ../../shared/CLAUDE.md", target)
	}
	// And following the symlink resolves (dst is a valid ccs root)
	if b, err := os.ReadFile(profACLAUDE); err != nil || string(b) != "shared-claude\n" {
		t.Errorf("follow profile a CLAUDE.md: %v / %q", err, b)
	}

	// profile a .claude.json round-trip
	if b, err := os.ReadFile(filepath.Join(dst, "profiles", "a", ".claude.json")); err != nil || string(b) != `{"p":"a"}` {
		t.Errorf("profile a .claude.json: %v / %q", err, b)
	}

	// cache must be excluded
	if _, err := os.Lstat(filepath.Join(dst, "profiles", "a", "cache")); !os.IsNotExist(err) {
		t.Errorf("cache should be excluded; err=%v", err)
	}

	// profile b fork round-trip (not a symlink)
	profBCLAUDE := filepath.Join(dst, "profiles", "b", "CLAUDE.md")
	binfo, err := os.Lstat(profBCLAUDE)
	if err != nil {
		t.Fatal(err)
	}
	if binfo.Mode()&os.ModeSymlink != 0 {
		t.Errorf("profile b CLAUDE.md unexpectedly a symlink")
	}
	if b, err := os.ReadFile(profBCLAUDE); err != nil || string(b) != "forked\n" {
		t.Errorf("profile b CLAUDE.md: %v / %q", err, b)
	}

	// env file round-trip
	if b, err := os.ReadFile(filepath.Join(dst, "env", "a.toml")); err != nil || string(b) != "[env]\nFOO=\"bar\"\n" {
		t.Errorf("env a.toml: %v / %q", err, b)
	}

	// credentials.json.age present as bytes
	if b, err := os.ReadFile(filepath.Join(dst, "credentials.json.age")); err != nil || string(b) != "dummy-encrypted" {
		t.Errorf("credentials.json.age: %v / %q", err, b)
	}
}

// writeMaliciousArchive crafts a gzipped tar whose first entry carries the
// provided header and optional body. A manifest entry is appended so the
// extractor has a reason to keep reading until the malicious header is
// processed.
func writeMaliciousArchive(t *testing.T, path, manifestName string, manifest any, h tar.Header, body []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	h.ModTime = time.Unix(0, 0)
	if err := tw.WriteHeader(&h); err != nil {
		t.Fatal(err)
	}
	if h.Typeflag == tar.TypeReg && len(body) > 0 {
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTarBytes(tw, manifestName, b); err != nil {
		t.Fatal(err)
	}
}

var traversalCases = []struct {
	name   string
	header tar.Header
	body   []byte
}{
	{
		name: "dotdot-regular",
		header: tar.Header{
			Name:     "../escape.txt",
			Mode:     0o644,
			Size:     5,
			Typeflag: tar.TypeReg,
		},
		body: []byte("pwned"),
	},
	{
		name: "absolute-regular",
		header: tar.Header{
			Name:     "/tmp/escape.txt",
			Mode:     0o644,
			Size:     5,
			Typeflag: tar.TypeReg,
		},
		body: []byte("pwned"),
	},
	{
		name: "symlink-relative-escape",
		header: tar.Header{
			Name:     "link",
			Typeflag: tar.TypeSymlink,
			Linkname: "../../outside",
		},
	},
	{
		name: "symlink-absolute-escape",
		header: tar.Header{
			Name:     "link",
			Typeflag: tar.TypeSymlink,
			Linkname: "/etc/passwd",
		},
	},
}

func TestUnpackBackupRejectsTraversal(t *testing.T) {
	for _, tc := range traversalCases {
		t.Run(tc.name, func(t *testing.T) {
			tarPath := filepath.Join(t.TempDir(), "bad.tar.gz")
			writeMaliciousArchive(t, tarPath, BackupManifestName, BackupManifest{Version: 1, Type: BackupType, SourcePlatform: runtime.GOOS}, tc.header, tc.body)
			dst := t.TempDir()
			_, err := UnpackBackup(tarPath, dst)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "escape") {
				t.Errorf("error %q does not look like a traversal rejection", err)
			}
			// Verify nothing was written outside destDir (best-effort check on
			// the sibling dir of dst).
			outside := filepath.Join(filepath.Dir(dst), "escape.txt")
			if _, err := os.Lstat(outside); err == nil {
				t.Errorf("file written outside dest: %s", outside)
			}
		})
	}
}

// A link entry placed beneath an earlier symlink is created somewhere other
// than its archive path says, so its target must not be checked lexically.
func TestUnpackBackupRejectsEntriesBeneathSymlink(t *testing.T) {
	tarPath := filepath.Join(t.TempDir(), "nested.tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, h := range []tar.Header{
		{Name: "profile/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "d1/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "d1/d2", Typeflag: tar.TypeSymlink, Linkname: "../profile"},
		{Name: "d1/d2/leak", Typeflag: tar.TypeSymlink, Linkname: "../../secret.txt"},
	} {
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := json.Marshal(BackupManifest{Version: 1, Type: BackupType})
	if err := writeTarBytes(tw, BackupManifestName, b); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(tw.Close(), gz.Close(), f.Close()); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if _, err := UnpackBackup(tarPath, dst); err == nil {
		t.Fatal("expected entry beneath a symlink to be rejected")
	}
	if _, err := os.Lstat(filepath.Join(dst, "profile", "leak")); err == nil {
		t.Error("symlink pointing outside dest was created")
	}
}

// A symlink chain can pass the per-link lexical check yet resolve outside the
// destination; writes through it must still be refused.
func TestUnpackRefusesWritesThroughEscapingSymlinkChain(t *testing.T) {
	tarPath := filepath.Join(t.TempDir(), "chain.tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	entries := []tar.Header{
		{Name: "sub/q", Typeflag: tar.TypeSymlink, Linkname: "."},
		{Name: "sub/p", Typeflag: tar.TypeSymlink, Linkname: "q/../.."},
		{Name: "sub/p/escape.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5},
	}
	for _, h := range entries {
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte("pwned")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writeTarBytes(tw, BackupManifestName, []byte(`{"type":"backup"}`)); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(tw.Close(), gz.Close(), f.Close()); err != nil {
		t.Fatal(err)
	}

	parent := t.TempDir()
	dst := filepath.Join(parent, "dest")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := UnpackBackup(tarPath, dst); err == nil {
		t.Fatal("expected write through escaping symlink chain to fail")
	}
	if _, err := os.Lstat(filepath.Join(parent, "escape.txt")); err == nil {
		t.Error("file written outside dest through symlink chain")
	}
}
