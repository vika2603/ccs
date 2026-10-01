package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/fsutil"
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
	MissingSharedLink
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
	case MissingSharedLink:
		return "missing-shared-link"
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

// KeychainLister returns the service names of stored keychain items. A nil
// lister skips the keychain check on platforms without a keychain.
type KeychainLister func() ([]string, error)

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

// profileDirs returns the entries of profiles/ that resolve to directories,
// following symlinks so a symlinked profile still counts.
func (c Checker) profileDirs() []os.DirEntry {
	entries, _ := os.ReadDir(c.paths.ProfilesDir())
	out := entries[:0]
	for _, e := range entries {
		if info, err := os.Stat(c.paths.ProfilePath(e.Name())); err == nil && info.IsDir() {
			out = append(out, e)
		}
	}
	return out
}

func (c Checker) Check() ([]Finding, error) {
	var out []Finding
	profiles := c.profileDirs()
	usedShared := map[string]bool{}
	for _, pe := range profiles {
		profileDir := c.paths.ProfilePath(pe.Name())
		entries, _ := os.ReadDir(profileDir)
		out = append(out, c.missingSharedLinks(pe.Name(), profileDir, entries)...)
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
					out = append(out, Finding{Kind: BrokenSymlink, Profile: pe.Name(), Detail: e.Name(), Path: linkPath})
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
			empty, err := fsutil.IsEmpty(path)
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

// missingSharedLinks reports configured shared fields absent from a profile
// that links at least one shared field. Profiles without any shared link,
// such as those created with `ccs new --blank`, are skipped on purpose.
func (c Checker) missingSharedLinks(profile, profileDir string, entries []os.DirEntry) []Finding {
	present := map[string]bool{}
	linked := false
	for _, e := range entries {
		present[e.Name()] = true
		if e.Type()&os.ModeSymlink != 0 && c.configured.Classify(e.Name()) == fields.Shared {
			linked = true
		}
	}
	if !linked {
		return nil
	}
	var out []Finding
	for _, s := range c.configured.Shared() {
		if !present[s.Name] {
			out = append(out, Finding{Kind: MissingSharedLink, Profile: profile, Detail: s.Name, Path: filepath.Join(profileDir, s.Name)})
		}
	}
	return out
}

func (c Checker) envOrphans(profiles []os.DirEntry) []Finding {
	entries, err := os.ReadDir(c.paths.EnvDir())
	if err != nil {
		return nil
	}
	have := map[string]bool{}
	for _, pe := range profiles {
		have[pe.Name()] = true
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
	out, _ := c.listKeychainOrphans(profiles)
	return out
}

func (c Checker) listKeychainOrphans(profiles []os.DirEntry) ([]Finding, error) {
	if c.keychain == nil {
		return nil, nil
	}
	services, err := c.keychain()
	if err != nil {
		return nil, fmt.Errorf("list keychain: %w", err)
	}
	defaultService, err := creds.ServiceName(c.defaultCCD, c.defaultCCD)
	if err != nil {
		return nil, err
	}
	expected := map[string]bool{defaultService: true}
	for _, pe := range profiles {
		if svc, err := creds.ServiceName(c.paths.ProfilePath(pe.Name()), c.defaultCCD); err == nil {
			expected[svc] = true
		}
	}
	var out []Finding
	for _, svc := range services {
		if !strings.HasPrefix(svc, defaultService) {
			continue
		}
		if expected[svc] {
			continue
		}
		out = append(out, Finding{Kind: OrphanKeychainEntry, Detail: svc, Path: svc})
	}
	return out, nil
}
