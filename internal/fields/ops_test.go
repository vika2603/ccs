package fields

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/layout"
)

func setupOps(t *testing.T) (Ops, layout.Paths) {
	home := t.TempDir()
	p := layout.New(home)
	os.MkdirAll(p.SharedDir(), 0o755)
	os.MkdirAll(p.ProfilesDir(), 0o755)
	r := NewRegistry(config.Default())
	return NewOps(p, r), p
}

func TestForkBreaksSymlink(t *testing.T) {
	ops, p := setupOps(t)
	profile := p.ProfilePath("work")
	os.MkdirAll(profile, 0o755)
	os.MkdirAll(p.SharedField("skills"), 0o755)
	os.WriteFile(filepath.Join(p.SharedField("skills"), "a.md"), []byte("A"), 0o644)
	os.Symlink(p.SharedField("skills"), filepath.Join(profile, "skills"))

	if err := ops.Fork("work", "skills"); err != nil {
		t.Fatalf("fork: %v", err)
	}
}

func TestShareConflictUsesPrompt(t *testing.T) {
	ops, p := setupOps(t)
	profile := p.ProfilePath("work")
	os.MkdirAll(filepath.Join(profile, "skills"), 0o755)
	os.WriteFile(filepath.Join(profile, "skills", "new.md"), []byte("N"), 0o644)
	os.MkdirAll(p.SharedField("skills"), 0o755)
	os.WriteFile(filepath.Join(p.SharedField("skills"), "old.md"), []byte("O"), 0o644)

	var out bytes.Buffer
	// Wrap the scripted input in bufio.Reader so PromptConflict's
	// non-TTY fail-closed branch does not trip on strings.Reader.
	if err := ops.Share("work", "skills", &out, bufio.NewReader(strings.NewReader("o\n"))); err != nil {
		t.Fatalf("share: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.SharedField("skills"), "new.md")); err != nil {
		t.Fatalf("expected overwrite path to win: %v", err)
	}
}

func TestForkShareRoundTripFileField(t *testing.T) {
	home := t.TempDir()
	p := layout.New(home)
	os.MkdirAll(p.SharedDir(), 0o755)
	os.MkdirAll(p.ProfilesDir(), 0o755)
	reg := NewRegistry(config.Config{Shared: []string{"statusline.sh"}})
	ops := NewOps(p, reg)

	profile := p.ProfilePath("work")
	os.MkdirAll(profile, 0o755)
	sharedFile := p.SharedField("statusline.sh")
	if err := os.WriteFile(sharedFile, []byte("#!/bin/sh\necho original\n"), 0o755); err != nil {
		t.Fatalf("write shared: %v", err)
	}
	linkPath := filepath.Join(profile, "statusline.sh")
	if err := os.Symlink(sharedFile, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := ops.Fork("work", "statusline.sh"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat after fork: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("after fork, profile copy should be a regular file, got %v", info.Mode())
	}
	got, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("read after fork: %v", err)
	}
	if string(got) != "#!/bin/sh\necho original\n" {
		t.Fatalf("fork did not preserve content: %q", got)
	}

	if err := os.WriteFile(linkPath, []byte("#!/bin/sh\necho mutated\n"), 0o755); err != nil {
		t.Fatalf("mutate: %v", err)
	}

	var out bytes.Buffer
	if err := ops.Share("work", "statusline.sh", &out, bufio.NewReader(strings.NewReader("o\n"))); err != nil {
		t.Fatalf("share: %v", err)
	}
	info, err = os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat after share: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("after share, profile entry should be a symlink, got %v", info.Mode())
	}
	sharedContent, err := os.ReadFile(sharedFile)
	if err != nil {
		t.Fatalf("read shared after share: %v", err)
	}
	if string(sharedContent) != "#!/bin/sh\necho mutated\n" {
		t.Fatalf("share did not push mutated content: %q", sharedContent)
	}
}

func TestStatus(t *testing.T) {
	ops, p := setupOps(t)
	profile := p.ProfilePath("work")
	os.MkdirAll(profile, 0o755)
	os.MkdirAll(p.SharedField("skills"), 0o755)
	os.Symlink(p.SharedField("skills"), filepath.Join(profile, "skills"))
	os.MkdirAll(filepath.Join(profile, "commands"), 0o755)

	st, err := ops.Status("work")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st["skills"] != Linked || st["commands"] != Forked {
		t.Fatalf("unexpected status map: %#v", st)
	}
}
