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
		"skills":            {Name: "skills", Category: Shared, Kind: KindDir},
		"CLAUDE.md":         {Name: "CLAUDE.md", Category: Shared, Kind: KindFile},
		"settings.json":     {Name: "settings.json", Category: Shared, Kind: KindFile},
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

func TestSelectExportMaterialFiltersByMode(t *testing.T) {
	profile := t.TempDir()
	os.MkdirAll(filepath.Join(profile, "projects"), 0o755)
	os.WriteFile(filepath.Join(profile, "history.jsonl"), []byte("history"), 0o644)
	os.WriteFile(filepath.Join(profile, "CLAUDE.md"), []byte("shared note"), 0o644)
	os.WriteFile(filepath.Join(profile, ".credentials.json"), []byte("secret"), 0o600)
	os.WriteFile(filepath.Join(profile, ".claude.json"), []byte(`{"oauthAccount":{"email":"x"}}`), 0o600)
	os.MkdirAll(filepath.Join(profile, "cache"), 0o755)

	reg := NewRegistry(config.Default())
	defEntries, err := SelectExportMaterial(profile, reg, ExportDefault)
	if err != nil {
		t.Fatalf("SelectExportMaterial(default): %v", err)
	}
	if containsEntry(defEntries, "cache") {
		t.Fatalf("transient cache should not be exported in any mode")
	}
	if containsEntry(defEntries, "projects") {
		t.Fatalf("isolated runtime projects/ must not appear in ExportDefault")
	}
	if containsEntry(defEntries, ".credentials.json") {
		t.Fatalf(".credentials.json must never be selected by SelectExportMaterial; credentials are handled separately")
	}
	if !containsEntry(defEntries, "CLAUDE.md") {
		t.Fatalf("CLAUDE.md (shared) should be selected in ExportDefault")
	}
	if containsEntry(defEntries, ".claude.json") {
		t.Fatalf(".claude.json must never be selected by SelectExportMaterial; it is appended by export_cmd under --with-credentials/--full")
	}

	fullEntries, err := SelectExportMaterial(profile, reg, ExportFull)
	if err != nil {
		t.Fatalf("SelectExportMaterial(full): %v", err)
	}
	if !containsEntry(fullEntries, "projects") {
		t.Fatalf("ExportFull must include isolated runtime entries")
	}
	if containsEntry(fullEntries, "cache") {
		t.Fatalf("transient cache must stay excluded even in ExportFull")
	}
	if containsEntry(fullEntries, ".credentials.json") {
		t.Fatalf(".credentials.json must never be selected by SelectExportMaterial; credentials are handled separately")
	}
	if containsEntry(fullEntries, ".claude.json") {
		t.Fatalf(".claude.json must never be selected by SelectExportMaterial even in ExportFull; it is appended by export_cmd")
	}

	credEntries, err := SelectExportMaterial(profile, reg, ExportWithCredentials)
	if err != nil {
		t.Fatalf("SelectExportMaterial(with-credentials): %v", err)
	}
	if containsEntry(credEntries, ".claude.json") {
		t.Fatalf(".claude.json must never be selected by SelectExportMaterial even in ExportWithCredentials; it is appended by export_cmd")
	}
	if containsEntry(credEntries, ".credentials.json") {
		t.Fatalf(".credentials.json must never be selected by SelectExportMaterial")
	}
}

func containsEntry(entries []Entry, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}
