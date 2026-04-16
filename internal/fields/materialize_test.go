package fields

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vika2603/ccs/internal/config"
)

type stubPrompter struct {
	overwrite    bool
	err          error
	unknownSeen  []string
	conflictSeen []string
}

func (s *stubPrompter) OnSharedConflict(name, existingPath, incomingPath string) (bool, error) {
	s.conflictSeen = append(s.conflictSeen, name)
	return s.overwrite, s.err
}

func (s *stubPrompter) OnUnknownEntry(name string) {
	s.unknownSeen = append(s.unknownSeen, name)
}

func TestImportEntriesSharedSymlinksIsolatedCopies(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	shared := filepath.Join(tmp, "shared")
	if err := os.MkdirAll(filepath.Join(src, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "skills", "a.md"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "CLAUDE.md"), []byte("note"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "projects", "p.txt"), []byte("P"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry(config.Default().Fields)
	prompter := &stubPrompter{}
	if err := ImportEntries(src, dst, shared, reg, prompter, false); err != nil {
		t.Fatalf("ImportEntries: %v", err)
	}

	info, err := os.Lstat(filepath.Join(dst, "skills"))
	if err != nil {
		t.Fatalf("lstat skills: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at dst/skills")
	}
	if b, _ := os.ReadFile(filepath.Join(shared, "skills", "a.md")); string(b) != "A" {
		t.Fatalf("shared content missing")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "projects", "p.txt")); string(b) != "P" {
		t.Fatalf("isolated content should be copied into dst profile")
	}
	if len(prompter.conflictSeen) != 0 {
		t.Fatalf("expected no conflict prompt on clean shared dir")
	}
}

func TestImportEntriesSharedConflictRoutesThroughPrompter(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	shared := filepath.Join(tmp, "shared")
	os.MkdirAll(filepath.Join(src, "skills"), 0o755)
	os.WriteFile(filepath.Join(src, "skills", "new.md"), []byte("NEW"), 0o644)
	os.MkdirAll(dst, 0o755)
	os.MkdirAll(filepath.Join(shared, "skills"), 0o755)
	os.WriteFile(filepath.Join(shared, "skills", "existing.md"), []byte("OLD"), 0o644)

	reg := NewRegistry(config.Default().Fields)

	aborter := &stubPrompter{overwrite: false}
	if err := ImportEntries(src, dst, shared, reg, aborter, false); err == nil {
		t.Fatalf("expected abort when prompter denies overwrite")
	}
	if got := aborter.conflictSeen; len(got) != 1 || got[0] != "skills" {
		t.Fatalf("expected one conflict prompt on skills, got %v", got)
	}
	if _, err := os.Stat(filepath.Join(shared, "skills", "existing.md")); err != nil {
		t.Fatalf("shared content should be preserved on abort: %v", err)
	}

	overwriter := &stubPrompter{overwrite: true}
	if err := ImportEntries(src, dst, shared, reg, overwriter, false); err != nil {
		t.Fatalf("ImportEntries(overwrite): %v", err)
	}
	if _, err := os.Stat(filepath.Join(shared, "skills", "existing.md")); err == nil {
		t.Fatalf("overwrite should have replaced existing shared content")
	}
	if b, _ := os.ReadFile(filepath.Join(shared, "skills", "new.md")); string(b) != "NEW" {
		t.Fatalf("overwrite should have installed new content")
	}
}

func TestImportEntriesUnknownReportedViaPrompter(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")
	shared := filepath.Join(tmp, "shared")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "novel-entry"), []byte("x"), 0o644)
	os.MkdirAll(dst, 0o755)
	os.MkdirAll(shared, 0o755)

	reg := NewRegistry(config.Default().Fields)
	prompter := &stubPrompter{}
	if err := ImportEntries(src, dst, shared, reg, prompter, false); err != nil {
		t.Fatalf("ImportEntries: %v", err)
	}
	if len(prompter.unknownSeen) != 1 || prompter.unknownSeen[0] != "novel-entry" {
		t.Fatalf("expected OnUnknownEntry(%q), got %v", "novel-entry", prompter.unknownSeen)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "novel-entry")); string(b) != "x" {
		t.Fatalf("unknown entry should have been copied as isolated")
	}
}
