package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
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

// Pack writes a single-profile export archive. Only the entries named in
// ProfileEntries are packed from ProfileDir.
func Pack(outPath string, opts PackOptions) error {
	return writeArchive(outPath, func(tw *tar.Writer) error {
		opts.Manifest.ExportedAt = time.Now().UTC()
		manifestJSON, err := json.MarshalIndent(opts.Manifest, "", "  ")
		if err != nil {
			return err
		}
		if err := writeTarBytes(tw, "manifest.json", manifestJSON); err != nil {
			return err
		}
		for _, name := range opts.ProfileEntries {
			entryPath := filepath.Join(opts.ProfileDir, name)
			if err := packEntry(tw, entryPath, path.Join("profile", name)); err != nil {
				return err
			}
		}
		for _, field := range slices.Sorted(maps.Keys(opts.SharedPaths)) {
			if err := packEntry(tw, opts.SharedPaths[field], path.Join("shared", field)); err != nil {
				return err
			}
		}
		if opts.Credentials != nil {
			return writeTarBytes(tw, "credentials.json.age", opts.Credentials)
		}
		return nil
	})
}

// writeArchive creates outPath as a gzipped tar, lets fill write its entries,
// and closes every layer so flush errors are reported. A partially written
// file is removed on failure.
func writeArchive(outPath string, fill func(*tar.Writer) error) (err error) {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(outPath)
		}
	}()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	err = fill(tw)
	err = errors.Join(err, tw.Close(), gz.Close(), f.Close())
	return err
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
	return errors.Join(writeTarBytes(tw, "manifest.json", data), tw.Close())
}

// Unpack extracts a single-profile export archive into destDir and returns
// its manifest.
func Unpack(tarPath, destDir string) (Manifest, error) {
	var m Manifest
	if err := extract(tarPath, destDir, "manifest.json", &m, false); err != nil {
		return Manifest{}, err
	}
	return m, nil
}
