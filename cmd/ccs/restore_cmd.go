package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/archive"
	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/state"
)

var restorePlatformOverride = runtime.GOOS

func newRestoreCmd() *cobra.Command {
	var force bool
	var noActive bool
	cmd := &cobra.Command{
		Use:   "restore <file>",
		Short: "Restore the entire ccs directory from a backup archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}

			tmp, err := os.MkdirTemp("", "ccs-restore-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)

			m, err := archive.UnpackBackup(src, tmp)
			if err != nil {
				return err
			}
			if m.Type != archive.BackupType {
				return fmt.Errorf("archive is not a full backup (type=%q); use `ccs import` for single-profile archives", m.Type)
			}
			if m.SourcePlatform != "" && m.SourcePlatform != restorePlatformOverride {
				return fmt.Errorf("archive platform %q does not match current platform %q; cross-platform restore is not supported yet", m.SourcePlatform, restorePlatformOverride)
			}

			if err := os.MkdirAll(p.Root(), 0o755); err != nil {
				return err
			}

			slots := []restoreSlot{}
			if _, err := os.Stat(filepath.Join(tmp, "config.toml")); err == nil {
				slots = append(slots, restoreSlot{filepath.Join(tmp, "config.toml"), p.ConfigFile()})
			}

			if err := collectChildrenAsSlots(filepath.Join(tmp, "shared"), p.SharedDir(), &slots); err != nil {
				return err
			}
			if err := collectChildrenAsSlots(filepath.Join(tmp, "profiles"), p.ProfilesDir(), &slots); err != nil {
				return err
			}
			if err := collectChildrenAsSlots(filepath.Join(tmp, "env"), p.EnvDir(), &slots); err != nil {
				return err
			}

			if !force {
				for _, s := range slots {
					if _, err := os.Lstat(s.dst); err == nil {
						return fmt.Errorf("%s already exists (use --force to overwrite)", s.dst)
					} else if !errors.Is(err, os.ErrNotExist) {
						return err
					}
				}
			}

			for _, s := range slots {
				if err := os.MkdirAll(filepath.Dir(s.dst), 0o755); err != nil {
					return err
				}
				if err := os.RemoveAll(s.dst); err != nil {
					return err
				}
				if err := copyPathTree(s.src, s.dst); err != nil {
					return err
				}
			}

			encPath := filepath.Join(tmp, "credentials.json.age")
			if _, err := os.Stat(encPath); err == nil {
				data, err := os.ReadFile(encPath)
				if err != nil {
					return err
				}
				pass, err := readPassphrase("Passphrase to decrypt credentials: ", false)
				if err != nil {
					return err
				}
				plain, err := archive.DecryptPassphrase(data, pass)
				if err != nil {
					return err
				}
				var bundle map[string]string
				if err := json.Unmarshal(plain, &bundle); err != nil {
					return fmt.Errorf("parse credentials bundle: %w", err)
				}
				store := creds.New()
				for name, b64 := range bundle {
					blob, err := base64.StdEncoding.DecodeString(b64)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: decode credentials for %q: %v\n", name, err)
						continue
					}
					if err := store.Write(p.ProfilePath(name), blob); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: write credentials for %q: %v\n", name, err)
					}
				}
			}

			if !noActive && m.Active != "" {
				if err := state.Write(p.ActiveFile(), m.Active); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: set active profile %q: %v\n", m.Active, err)
				}
			}

			cmd.Printf("restored %d profile(s) from %s\n", len(m.Profiles), src)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing entries in ~/.ccs")
	cmd.Flags().BoolVar(&noActive, "no-active", false, "do not restore the active profile pointer")
	return cmd
}

type restoreSlot struct {
	src string
	dst string
}

// collectChildrenAsSlots lists srcDir and appends each entry as a pending
// install into dstDir. When srcDir does not exist it is a no-op.
func collectChildrenAsSlots(srcDir, dstDir string, out *[]restoreSlot) error {
	entries, err := os.ReadDir(srcDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		*out = append(*out, restoreSlot{
			src: filepath.Join(srcDir, e.Name()),
			dst: filepath.Join(dstDir, e.Name()),
		})
	}
	return nil
}

// copyPathTree copies src to dst, preserving symlinks (not dereferenced) and
// file modes. Intended for moving a freshly unpacked tree into ~/.ccs.
func copyPathTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyPathTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
