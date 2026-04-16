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
	r := NewRegistry(config.Default().Fields)
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
