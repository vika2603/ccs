package fields

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vika2603/ccs/internal/config"
)

func TestDescribeIncludesCategoryAndKind(t *testing.T) {
	r := NewRegistry(config.Default())
	cases := map[string]Classification{
		"skills":                    {Name: "skills", Category: Shared, Kind: KindDir},
		"CLAUDE.md":                 {Name: "CLAUDE.md", Category: Shared, Kind: KindFile},
		"settings.json":             {Name: "settings.json", Category: Shared, Kind: KindFile},
		"projects":                  {Name: "projects", Category: Isolated, Kind: KindDir},
		".credentials.json":         {Name: ".credentials.json", Category: Isolated, Kind: KindFile},
		".claude.json":              {Name: ".claude.json", Category: Isolated, Kind: KindFile},
		"mcp-needs-auth-cache.json": {Name: "mcp-needs-auth-cache.json", Category: Isolated, Kind: KindFile},
		"policy-limits.json":        {Name: "policy-limits.json", Category: Isolated, Kind: KindFile},
		"backups":                   {Name: "backups", Category: Isolated, Kind: KindDir},
		"sessions":                  {Name: "sessions", Category: Isolated, Kind: KindDir},
		"cache":                     {Name: "cache", Category: Isolated, Kind: KindDir},
		"unknown":                   {Name: "unknown", Category: Isolated, Kind: KindDir},
	}
	for name, want := range cases {
		got := r.Describe(name)
		if got.Category != want.Category || got.Kind != want.Kind {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestInferKindExtensionHeuristic(t *testing.T) {
	cases := map[string]Kind{
		"statusline.sh": KindFile,
		"config.yaml":   KindFile,
		"notes.md":      KindFile,
		"skills":        KindDir,
		"backups":       KindDir,
		"unknown":       KindDir,
	}
	for name, want := range cases {
		if got := inferKind(name); got != want {
			t.Errorf("inferKind(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestCreateSharedTargetsCreatesRegularFileForExtensionName(t *testing.T) {
	dir := t.TempDir()
	entries := []Classification{
		{Name: "statusline.sh", Category: Shared, Kind: inferKind("statusline.sh")},
	}
	if err := CreateSharedTargets(dir, entries); err != nil {
		t.Fatalf("CreateSharedTargets: %v", err)
	}
	info, err := os.Lstat(filepath.Join(dir, "statusline.sh"))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("statusline.sh should be a regular file, got %v", info.Mode())
	}
}

func TestDescribeReportsUnknown(t *testing.T) {
	r := NewRegistry(config.Default())
	if !r.IsUnknown("novel-entry") {
		t.Fatalf("expected novel-entry to be flagged unknown")
	}
	if r.IsUnknown("skills") {
		t.Fatalf("skills should not be unknown")
	}
}

func TestListShared(t *testing.T) {
	r := NewRegistry(config.Default())
	got := r.Shared()
	if len(got) == 0 {
		t.Fatalf("expected non-empty shared list")
	}
	for _, entry := range got {
		if entry.Name == "CLAUDE.md" && entry.Kind != KindFile {
			t.Fatalf("CLAUDE.md should be KindFile")
		}
	}
}

func TestCreateSharedTargetsCreatesRegularFilesForKindFile(t *testing.T) {
	dir := t.TempDir()
	entries := []Classification{
		{Name: "skills", Category: Shared, Kind: KindDir},
		{Name: "CLAUDE.md", Category: Shared, Kind: KindFile},
	}
	if err := CreateSharedTargets(dir, entries); err != nil {
		t.Fatalf("CreateSharedTargets: %v", err)
	}
	info, err := os.Lstat(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("CLAUDE.md should be a regular file, got %v", info.Mode())
	}
}
