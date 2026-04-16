package fields

import (
	"os"
	"path/filepath"
	"sort"
)

type ProfileEntry struct {
	Name      string
	Category  Category
	Kind      Kind
	Size      int64
	FileCount int
	IsUnknown bool
}

func ScanProfile(profileDir string, reg *Registry) ([]ProfileEntry, error) {
	dirEntries, err := os.ReadDir(profileDir)
	if err != nil {
		return nil, err
	}
	var out []ProfileEntry
	for _, de := range dirEntries {
		name := de.Name()
		if name == ".credentials.json" {
			continue
		}
		class := reg.Describe(name)
		path := filepath.Join(profileDir, name)
		kind, err := detectKind(path)
		if err != nil {
			return nil, err
		}
		size := int64(0)
		count := 0
		if kind == KindDir {
			size, count = measureDir(path)
		} else {
			info, err := de.Info()
			if err != nil {
				return nil, err
			}
			size = info.Size()
		}
		out = append(out, ProfileEntry{
			Name:      name,
			Category:  class.Category,
			Kind:      kind,
			Size:      size,
			FileCount: count,
			IsUnknown: reg.IsUnknown(name),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func measureDir(root string) (int64, int) {
	var size int64
	var count int
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
			count++
		}
		return nil
	})
	return size, count
}
