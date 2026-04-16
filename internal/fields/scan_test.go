package fields

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/vika2603/ccs/internal/config"
)

func TestScanProfileClassifiesAndExcludesCredentialsFile(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "skills"))
	mustMkdir(t, filepath.Join(dir, "projects"))
	mustMkdir(t, filepath.Join(dir, "cache"))
	mustWrite(t, filepath.Join(dir, "CLAUDE.md"), "shared note")
	mustWrite(t, filepath.Join(dir, ".credentials.json"), "secret")
	mustWrite(t, filepath.Join(dir, ".claude.json"), "{}")
	mustWrite(t, filepath.Join(dir, "my-notes.md"), "user content")
	mustMkdir(t, filepath.Join(dir, "scratch"))

	reg := NewRegistry(config.Default().Fields)
	entries, err := ScanProfile(dir, reg)
	if err != nil {
		t.Fatalf("ScanProfile: %v", err)
	}

	got := map[string]ProfileEntry{}
	for _, e := range entries {
		got[e.Name] = e
	}
	if _, present := got[".credentials.json"]; present {
		t.Fatalf("ScanProfile must exclude .credentials.json; entries=%v", keys(got))
	}

	cases := map[string]struct {
		category  Category
		kind      Kind
		isUnknown bool
	}{
		"skills":       {Shared, KindDir, false},
		"CLAUDE.md":    {Shared, KindFile, false},
		"projects":     {Isolated, KindDir, false},
		".claude.json": {Isolated, KindFile, false},
		"cache":        {Transient, KindDir, false},
		"my-notes.md":  {Isolated, KindFile, true},
		"scratch":      {Isolated, KindDir, true},
	}
	for name, want := range cases {
		e, ok := got[name]
		if !ok {
			t.Errorf("missing entry %q; scanned=%v", name, sorted(got))
			continue
		}
		if e.Category != want.category || e.Kind != want.kind || e.IsUnknown != want.isUnknown {
			t.Errorf("%s: category=%v kind=%v unknown=%v; want %v/%v/%v",
				name, e.Category, e.Kind, e.IsUnknown,
				want.category, want.kind, want.isUnknown)
		}
	}
}

func TestScanProfileComputesSizeAndCount(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "CLAUDE.md"), "1234567890")
	mustMkdir(t, filepath.Join(dir, "skills"))
	mustWrite(t, filepath.Join(dir, "skills", "a.md"), "A")
	mustWrite(t, filepath.Join(dir, "skills", "b.md"), "BB")
	mustMkdir(t, filepath.Join(dir, "skills", "nested"))
	mustWrite(t, filepath.Join(dir, "skills", "nested", "c.md"), "CCC")

	reg := NewRegistry(config.Default().Fields)
	entries, err := ScanProfile(dir, reg)
	if err != nil {
		t.Fatalf("ScanProfile: %v", err)
	}
	got := map[string]ProfileEntry{}
	for _, e := range entries {
		got[e.Name] = e
	}
	if got["CLAUDE.md"].Size != 10 {
		t.Errorf("CLAUDE.md size = %d; want 10", got["CLAUDE.md"].Size)
	}
	if got["skills"].FileCount != 3 {
		t.Errorf("skills file count = %d; want 3", got["skills"].FileCount)
	}
	if got["skills"].Size != 6 {
		t.Errorf("skills size = %d; want 6", got["skills"].Size)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func keys(m map[string]ProfileEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sorted(m map[string]ProfileEntry) []string { return keys(m) }
