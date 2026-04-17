package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
)

type Kind int

const (
	BrokenSymlink Kind = iota
	OrphanSharedField
	UnclassifiedEntry
	OrphanKeychainEntry
	ClassificationDrift
	OrphanEnvFile
)

func (k Kind) String() string {
	switch k {
	case BrokenSymlink:
		return "broken-symlink"
	case OrphanSharedField:
		return "orphan-shared-field"
	case UnclassifiedEntry:
		return "unclassified-entry"
	case OrphanKeychainEntry:
		return "orphan-keychain-entry"
	case ClassificationDrift:
		return "classification-drift"
	case OrphanEnvFile:
		return "orphan-env-file"
	default:
		return "unknown"
	}
}

type Finding struct {
	Kind    Kind
	Profile string
	Detail  string
	Path    string
}

type KeychainLister interface {
	List() ([]string, error)
}

type Checker struct {
	paths      layout.Paths
	configured *fields.Registry
	defaults   *fields.Registry
	keychain   KeychainLister
	defaultCCD string
}

func NewChecker(p layout.Paths, configured, defaults *fields.Registry, kc KeychainLister, defaultCCD string) Checker {
	return Checker{paths: p, configured: configured, defaults: defaults, keychain: kc, defaultCCD: defaultCCD}
}

func (c Checker) Check() ([]Finding, error) {
	var out []Finding
	profiles, _ := os.ReadDir(c.paths.ProfilesDir())
	usedShared := map[string]bool{}
	for _, pe := range profiles {
		if !pe.IsDir() {
			continue
		}
		profileDir := c.paths.ProfilePath(pe.Name())
		entries, _ := os.ReadDir(profileDir)
		for _, e := range entries {
			linkPath := filepath.Join(profileDir, e.Name())
			info, err := os.Lstat(linkPath)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(linkPath)
				if err != nil {
					continue
				}
				if _, err := os.Stat(target); err != nil {
					out = append(out, Finding{Kind: BrokenSymlink, Profile: pe.Name(), Path: linkPath})
					continue
				}
				usedShared[filepath.Base(target)] = true
			}
			if c.configured.IsUnknown(e.Name()) {
				out = append(out, Finding{Kind: UnclassifiedEntry, Profile: pe.Name(), Detail: e.Name(), Path: linkPath})
			}
		}
	}
	sharedEntries, _ := os.ReadDir(c.paths.SharedDir())
	knownShared := map[string]bool{}
	for _, s := range c.configured.Shared() {
		knownShared[s.Name] = true
	}
	for _, e := range sharedEntries {
		if usedShared[e.Name()] {
			continue
		}
		path := filepath.Join(c.paths.SharedDir(), e.Name())
		if knownShared[e.Name()] {
			empty, err := sharedEntryEmpty(path)
			if err == nil && empty {
				continue
			}
		}
		out = append(out, Finding{Kind: OrphanSharedField, Detail: e.Name(), Path: path})
	}
	out = append(out, c.classificationDrift()...)
	out = append(out, c.keychainOrphans(profiles)...)
	out = append(out, c.envOrphans(profiles)...)
	return out, nil
}

func (c Checker) envOrphans(profiles []os.DirEntry) []Finding {
	entries, err := os.ReadDir(c.paths.EnvDir())
	if err != nil {
		return nil
	}
	have := map[string]bool{}
	for _, pe := range profiles {
		if pe.IsDir() {
			have[pe.Name()] = true
		}
	}
	var out []Finding
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".toml") {
			continue
		}
		profile := strings.TrimSuffix(name, ".toml")
		if have[profile] {
			continue
		}
		out = append(out, Finding{
			Kind:   OrphanEnvFile,
			Detail: profile,
			Path:   filepath.Join(c.paths.EnvDir(), name),
		})
	}
	return out
}

func (c Checker) classificationDrift() []Finding {
	var out []Finding
	configured := c.configured.All()
	defaults := c.defaults.All()
	for name, want := range defaults {
		got, ok := configured[name]
		if !ok {
			continue
		}
		if got.Category != want.Category {
			out = append(out, Finding{
				Kind:   ClassificationDrift,
				Detail: name,
				Path:   c.paths.ConfigFile(),
			})
		}
	}
	return out
}

func (c Checker) keychainOrphans(profiles []os.DirEntry) []Finding {
	if c.keychain == nil {
		return nil
	}
	services, err := c.keychain.List()
	if err != nil {
		return nil
	}
	expected := map[string]bool{"Claude Code-credentials": true}
	for _, pe := range profiles {
		if !pe.IsDir() {
			continue
		}
		expected[expectedServiceName(c.paths.ProfilePath(pe.Name()))] = true
	}
	var out []Finding
	for _, svc := range services {
		if !strings.HasPrefix(svc, "Claude Code-credentials") {
			continue
		}
		if expected[svc] {
			continue
		}
		out = append(out, Finding{Kind: OrphanKeychainEntry, Detail: svc, Path: svc})
	}
	return out
}

func sharedEntryEmpty(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return info.Size() == 0, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func expectedServiceName(path string) string {
	abs, _ := filepath.Abs(path)
	abs = filepath.Clean(abs)
	sum := sha256.Sum256([]byte(abs))
	return "Claude Code-credentials-" + hex.EncodeToString(sum[:])[:8]
}
