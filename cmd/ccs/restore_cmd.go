package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/archive"
	"github.com/vika2603/ccs/internal/fsutil"
	"github.com/vika2603/ccs/internal/layout"
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
			// Only paths and the credential store are needed; skipping
			// config.toml keeps recovery possible when it is unreadable.
			a, err := loadPaths()
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

			if err := os.MkdirAll(a.Root(), 0o755); err != nil {
				return err
			}

			slots := []restoreSlot{}
			if _, err := os.Stat(filepath.Join(tmp, "config.toml")); err == nil {
				slots = append(slots, restoreSlot{filepath.Join(tmp, "config.toml"), a.ConfigFile()})
			}

			if err := collectChildrenAsSlots(filepath.Join(tmp, "shared"), a.SharedDir(), &slots); err != nil {
				return err
			}
			if err := collectChildrenAsSlots(filepath.Join(tmp, "profiles"), a.ProfilesDir(), &slots); err != nil {
				return err
			}
			if err := collectChildrenAsSlots(filepath.Join(tmp, "env"), a.EnvDir(), &slots); err != nil {
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
				if err := fsutil.CopyTreeNoFollow(s.src, s.dst); err != nil {
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
				for name, b64 := range bundle {
					if err := layout.ValidName(name); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: skip credentials: %v\n", err)
						continue
					}
					blob, err := base64.StdEncoding.DecodeString(b64)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: decode credentials for %q: %v\n", name, err)
						continue
					}
					if err := a.creds.Write(a.ProfilePath(name), blob); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: write credentials for %q: %v\n", name, err)
					}
				}
			}

			if !noActive && m.Active != "" {
				if err := a.SetActive(m.Active); err != nil {
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
