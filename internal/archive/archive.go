package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type PackOptions struct {
	ProfileDir     string
	ProfileName    string
	ProfileEntries []string
	SharedPaths    map[string]string
	Manifest       Manifest
	Credentials    []byte
}

func Pack(outPath string, opts PackOptions) error {
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
	manifestJSON, err := json.MarshalIndent(opts.Manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := writeTarBytes(tw, "manifest.json", manifestJSON); err != nil {
		return err
	}

	if len(opts.ProfileEntries) == 0 {
		if err := walkDeref(tw, opts.ProfileDir, "profile"); err != nil {
			return err
		}
	} else {
		for _, name := range opts.ProfileEntries {
			entryPath := filepath.Join(opts.ProfileDir, name)
			archiveName := filepath.ToSlash(filepath.Join("profile", name))
			if err := packEntry(tw, entryPath, archiveName); err != nil {
				return err
			}
		}
	}

	for field, path := range opts.SharedPaths {
		archiveName := filepath.ToSlash(filepath.Join("shared", field))
		if err := packEntry(tw, path, archiveName); err != nil {
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

func packEntry(tw *tar.Writer, srcPath, archivePath string) error {
	info, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(srcPath)
		if err != nil {
			return err
		}
		ti, err := os.Stat(target)
		if err != nil {
			return err
		}
		if ti.IsDir() {
			return walkDeref(tw, target, archivePath)
		}
		return writeTarFile(tw, archivePath, target, ti)
	}
	if info.IsDir() {
		return walkDeref(tw, srcPath, archivePath)
	}
	return writeTarFile(tw, archivePath, srcPath, info)
}

func walkDeref(tw *tar.Writer, root, prefix string) error {
	return filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(filepath.Join(prefix, rel))

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(p)
			if err != nil {
				return err
			}
			ti, err := os.Stat(target)
			if err != nil {
				return err
			}
			if ti.IsDir() {
				return walkDeref(tw, target, name)
			}
			return writeTarFile(tw, name, target, ti)
		}
		if info.IsDir() {
			h := &tar.Header{Name: name + "/", Mode: int64(info.Mode().Perm()), Typeflag: tar.TypeDir, ModTime: info.ModTime()}
			return tw.WriteHeader(h)
		}
		return writeTarFile(tw, name, p, info)
	})
}

func writeTarFile(tw *tar.Writer, name, path string, info os.FileInfo) error {
	h := &tar.Header{
		Name:     name,
		Mode:     int64(info.Mode().Perm()),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

func writeTarBytes(tw *tar.Writer, name string, data []byte) error {
	h := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg, ModTime: time.Now()}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func WriteMinimalManifestTar(w io.Writer, m Manifest) error {
	if m.ExportedAt.IsZero() {
		m.ExportedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tw := tar.NewWriter(w)
	if err := writeTarBytes(tw, "manifest.json", data); err != nil {
		tw.Close()
		return err
	}
	return tw.Close()
}

func Unpack(tarPath, destDir string) (Manifest, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return Manifest{}, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return Manifest{}, err
	}
	tr := tar.NewReader(gz)
	var m Manifest
	var manifestSeen bool
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Manifest{}, err
		}
		out := filepath.Join(destDir, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(out, os.FileMode(h.Mode)); err != nil {
				return Manifest{}, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return Manifest{}, err
			}
			wf, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return Manifest{}, err
			}
			if _, err := io.Copy(wf, tr); err != nil {
				wf.Close()
				return Manifest{}, err
			}
			wf.Close()
			if h.Name == "manifest.json" {
				b, _ := os.ReadFile(out)
				if err := json.Unmarshal(b, &m); err != nil {
					return Manifest{}, err
				}
				manifestSeen = true
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return Manifest{}, err
			}
			if err := os.Symlink(h.Linkname, out); err != nil {
				return Manifest{}, err
			}
		default:
			return Manifest{}, fmt.Errorf("unsupported tar entry %q", h.Name)
		}
	}
	if !manifestSeen {
		return Manifest{}, fmt.Errorf("manifest.json missing from archive")
	}
	return m, nil
}
