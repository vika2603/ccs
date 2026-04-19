package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupManifestName is the filename inside a full-backup archive that
// carries metadata. It is intentionally distinct from Manifest's filename
// ("manifest.json") so a caller can tell the two archive shapes apart
// without fully parsing either.
const BackupManifestName = "backup-manifest.json"

// BackupType is the value stored in BackupManifest.Type.
const BackupType = "backup"

type BackupManifest struct {
	Version        int       `json:"version"`
	Type           string    `json:"type"`
	ExportedAt     time.Time `json:"exported_at"`
	SourcePlatform string    `json:"source_platform"`
	Active         string    `json:"active,omitempty"`
	Profiles       []string  `json:"profiles"`
	Shared         []string  `json:"shared,omitempty"`
	Isolated       []string  `json:"isolated,omitempty"`
	Exclude        []string  `json:"exclude,omitempty"`
}

type BackupPackOptions struct {
	CCSRoot          string
	Profiles         []string
	PerProfileExclude []string
	ConfigPath       string
	EnvDir           string
	SharedDir        string
	Credentials      []byte
	Manifest         BackupManifest
}

// PackBackup writes a full backup archive to outPath. The archive preserves
// profile symlinks that point into the SharedDir as relative symlinks so the
// backup is portable across machines.
func PackBackup(outPath string, opts BackupPackOptions) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	opts.Manifest.ExportedAt = time.Now().UTC()
	if opts.Manifest.Type == "" {
		opts.Manifest.Type = BackupType
	}
	if opts.Manifest.Version == 0 {
		opts.Manifest.Version = 1
	}
	manifestJSON, err := json.MarshalIndent(opts.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := writeTarBytes(tw, BackupManifestName, manifestJSON); err != nil {
		return err
	}

	if opts.ConfigPath != "" {
		if _, err := os.Stat(opts.ConfigPath); err == nil {
			if err := packFileAt(tw, opts.ConfigPath, "config.toml"); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	if opts.SharedDir != "" {
		if _, err := os.Stat(opts.SharedDir); err == nil {
			if err := walkPreserveSymlinks(tw, opts.SharedDir, "shared", opts.CCSRoot, nil); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	excludeSet := map[string]struct{}{}
	for _, e := range opts.PerProfileExclude {
		excludeSet[e] = struct{}{}
	}
	for _, name := range opts.Profiles {
		profileDir := filepath.Join(opts.CCSRoot, "profiles", name)
		archivePrefix := filepath.ToSlash(filepath.Join("profiles", name))
		if err := walkPreserveSymlinks(tw, profileDir, archivePrefix, opts.CCSRoot, excludeSet); err != nil {
			return err
		}
	}

	if opts.EnvDir != "" {
		if _, err := os.Stat(opts.EnvDir); err == nil {
			entries, err := os.ReadDir(opts.EnvDir)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				src := filepath.Join(opts.EnvDir, e.Name())
				arc := filepath.ToSlash(filepath.Join("env", e.Name()))
				if err := packFileAt(tw, src, arc); err != nil {
					return err
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	if opts.Credentials != nil {
		if err := writeTarBytes(tw, "credentials.json.age", opts.Credentials); err != nil {
			return err
		}
	}
	return nil
}

// UnpackBackup extracts a backup archive to destDir and returns its manifest.
// Symlinks recorded in the archive are recreated with their stored relative
// targets, so the caller must extract into the target ~/.ccs root for the
// relative paths to resolve correctly.
func UnpackBackup(tarPath, destDir string) (BackupManifest, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return BackupManifest{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return BackupManifest{}, err
	}
	tr := tar.NewReader(gz)
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return BackupManifest{}, err
	}
	absDest = filepath.Clean(absDest)
	var m BackupManifest
	var manifestSeen bool
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return BackupManifest{}, err
		}
		out, err := safeJoin(absDest, h.Name)
		if err != nil {
			return BackupManifest{}, err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(out, os.FileMode(h.Mode)); err != nil {
				return BackupManifest{}, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return BackupManifest{}, err
			}
			wf, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return BackupManifest{}, err
			}
			if _, err := io.Copy(wf, tr); err != nil {
				wf.Close()
				return BackupManifest{}, err
			}
			wf.Close()
			if h.Name == BackupManifestName {
				b, _ := os.ReadFile(out)
				if err := json.Unmarshal(b, &m); err != nil {
					return BackupManifest{}, err
				}
				manifestSeen = true
			}
		case tar.TypeSymlink:
			if err := validateSymlinkTarget(absDest, out, h.Linkname); err != nil {
				return BackupManifest{}, err
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return BackupManifest{}, err
			}
			if err := os.Symlink(h.Linkname, out); err != nil {
				return BackupManifest{}, err
			}
		default:
			return BackupManifest{}, fmt.Errorf("unsupported tar entry %q", h.Name)
		}
	}
	if !manifestSeen {
		return BackupManifest{}, fmt.Errorf("%s missing from archive", BackupManifestName)
	}
	return m, nil
}

// walkPreserveSymlinks walks root and writes every entry under the given
// archive prefix. Symlinks whose targets resolve inside ccsRoot are stored as
// relative symlinks so the archive stays portable; symlinks outside ccsRoot
// are dereferenced. Directory entries listed in excludeTop (by basename) are
// skipped when they appear at the top level of root.
func walkPreserveSymlinks(tw *tar.Writer, root, prefix, ccsRoot string, excludeTop map[string]struct{}) error {
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			h := &tar.Header{Name: prefix + "/", Mode: int64(info.Mode().Perm()), Typeflag: tar.TypeDir, ModTime: info.ModTime()}
			return tw.WriteHeader(h)
		}
		if excludeTop != nil {
			top := rel
			if i := strings.IndexAny(rel, string(os.PathSeparator)+"/"); i >= 0 {
				top = rel[:i]
			}
			if _, skip := excludeTop[top]; skip {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			absTarget := target
			if !filepath.IsAbs(absTarget) {
				absTarget = filepath.Join(filepath.Dir(p), target)
			}
			absTarget = filepath.Clean(absTarget)
			inside, err := pathInside(ccsRoot, absTarget)
			if err != nil {
				return err
			}
			if inside {
				relLink, err := filepath.Rel(filepath.Dir(p), absTarget)
				if err != nil {
					return err
				}
				h := &tar.Header{
					Name:     name,
					Mode:     int64(info.Mode().Perm()),
					Typeflag: tar.TypeSymlink,
					Linkname: filepath.ToSlash(relLink),
					ModTime:  info.ModTime(),
				}
				return tw.WriteHeader(h)
			}
			ti, err := os.Stat(absTarget)
			if err != nil {
				return err
			}
			if ti.IsDir() {
				return walkPreserveSymlinks(tw, absTarget, name, ccsRoot, nil)
			}
			return writeTarFile(tw, name, absTarget, ti)
		}
		if info.IsDir() {
			h := &tar.Header{Name: name + "/", Mode: int64(info.Mode().Perm()), Typeflag: tar.TypeDir, ModTime: info.ModTime()}
			return tw.WriteHeader(h)
		}
		return writeTarFile(tw, name, p, info)
	})
}

// safeJoin joins name onto absRoot and rejects paths that escape absRoot
// (via absolute paths, `..`, or drive-relative shenanigans).
func safeJoin(absRoot, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty tar entry name")
	}
	clean := filepath.FromSlash(name)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute tar entry %q", name)
	}
	joined := filepath.Join(absRoot, clean)
	rel, err := filepath.Rel(absRoot, joined)
	if err != nil {
		return "", fmt.Errorf("tar entry %q escapes dest: %w", name, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("tar entry %q escapes dest", name)
	}
	return joined, nil
}

// validateSymlinkTarget ensures a symlink written at out pointing at linkname
// resolves within absRoot. Both absolute and relative linknames are checked.
func validateSymlinkTarget(absRoot, out, linkname string) error {
	if linkname == "" {
		return fmt.Errorf("empty symlink target")
	}
	target := filepath.FromSlash(linkname)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(out), target)
	}
	target = filepath.Clean(target)
	rel, err := filepath.Rel(absRoot, target)
	if err != nil {
		return fmt.Errorf("symlink %q escapes dest: %w", linkname, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("symlink %q escapes dest", linkname)
	}
	return nil
}

func pathInside(root, target string) (bool, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	absRoot = filepath.Clean(absRoot)
	absTarget = filepath.Clean(absTarget)
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false, nil
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)), nil
}

func packFileAt(tw *tar.Writer, srcPath, archivePath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	return writeTarFile(tw, archivePath, srcPath, info)
}
