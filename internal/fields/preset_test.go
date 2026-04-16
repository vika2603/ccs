package fields

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vika2603/ccs/internal/config"
)

func TestPresetSelection(t *testing.T) {
	items := []ProfileEntry{
		{Name: "skills", Category: Shared, Kind: KindDir},
		{Name: "CLAUDE.md", Category: Shared, Kind: KindFile},
		{Name: ".claude.json", Category: Isolated, Kind: KindFile},
		{Name: "projects", Category: Isolated, Kind: KindDir},
		{Name: "scratch", Category: Isolated, Kind: KindDir, IsUnknown: true},
	}

	cases := map[Preset]PresetSeed{
		PresetDefault: {
			Entries:     map[string]bool{"skills": true, "CLAUDE.md": true},
			Credentials: false,
		},
		PresetWithCreds: {
			Entries: map[string]bool{
				"skills": true, "CLAUDE.md": true, ".claude.json": true,
			},
			Credentials: true,
		},
		PresetFull: {
			Entries: map[string]bool{
				"skills": true, "CLAUDE.md": true,
				".claude.json": true, "projects": true, "scratch": true,
			},
			Credentials: true,
		},
	}

	for p, want := range cases {
		got := PresetSelection(items, p)
		if !reflect.DeepEqual(got.Entries, want.Entries) {
			t.Errorf("%s entries = %v; want %v", p, got.Entries, want.Entries)
		}
		if got.Credentials != want.Credentials {
			t.Errorf("%s credentials = %v; want %v", p, got.Credentials, want.Credentials)
		}
	}
}

func TestPresetSelectionMatchesSelectExportMaterial(t *testing.T) {
	profile := t.TempDir()
	mustMkdir(t, filepath.Join(profile, "skills"))
	mustWrite(t, filepath.Join(profile, "CLAUDE.md"), "shared")
	mustWrite(t, filepath.Join(profile, ".claude.json"), `{}`)
	mustMkdir(t, filepath.Join(profile, "projects"))
	mustMkdir(t, filepath.Join(profile, "scratch"))
	mustWrite(t, filepath.Join(profile, ".credentials.json"), "secret")
	mustMkdir(t, filepath.Join(profile, "cache"))

	reg := NewRegistry(config.Default())
	rawItems, err := ScanProfile(profile, reg)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	items := rawItems[:0]
	for _, it := range rawItems {
		if reg.IsExcludedFromExport(it.Name) {
			continue
		}
		items = append(items, it)
	}

	cases := []struct {
		preset Preset
		mode   ExportMode
		extra  []string
	}{
		{PresetDefault, ExportDefault, nil},
		{PresetWithCreds, ExportWithCredentials, []string{".claude.json"}},
		{PresetFull, ExportFull, []string{".claude.json"}},
	}

	for _, c := range cases {
		t.Run(c.preset.String(), func(t *testing.T) {
			seed := PresetSelection(items, c.preset)
			gotPicker := map[string]bool{}
			for name, ok := range seed.Entries {
				if ok {
					gotPicker[name] = true
				}
			}
			selected, err := SelectExportMaterial(profile, reg, c.mode)
			if err != nil {
				t.Fatalf("SelectExportMaterial: %v", err)
			}
			gotNonInteractive := map[string]bool{}
			for _, e := range selected {
				gotNonInteractive[e.Name] = true
			}
			for _, n := range c.extra {
				if _, ok := gotPicker[n]; ok {
					gotNonInteractive[n] = true
				}
			}
			if !reflect.DeepEqual(gotPicker, gotNonInteractive) {
				t.Fatalf("preset %s mismatch:\n picker:          %v\n non-interactive: %v",
					c.preset, gotPicker, gotNonInteractive)
			}
		})
	}
}
