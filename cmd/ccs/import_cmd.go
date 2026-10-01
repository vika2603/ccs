package main

import (
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

var importPlatformOverride = runtime.GOOS

func newImportCmd() *cobra.Command {
	var asName string
	var force bool
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import a single profile from a ccs export archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			// Only paths and the credential store are needed; skipping
			// config.toml keeps recovery possible when it is unreadable.
			a, err := loadPaths()
			if err != nil {
				return err
			}

			tmp, err := os.MkdirTemp("", "ccs-import-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)

			m, err := archive.Unpack(src, tmp)
			if err != nil {
				return err
			}
			if m.SourcePlatform != "" && m.SourcePlatform != importPlatformOverride {
				return fmt.Errorf("archive platform %q does not match current platform %q; cross-platform import is not supported yet", m.SourcePlatform, importPlatformOverride)
			}
			name := m.Profile
			if asName != "" {
				name = asName
			}
			if err := layout.ValidName(name); err != nil {
				return err
			}
			dst := a.ProfilePath(name)
			if _, err := os.Stat(dst); err == nil {
				if !force {
					return fmt.Errorf("profile %q already exists (use --force)", name)
				}
				if err := os.RemoveAll(dst); err != nil {
					return err
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			if profileSrc := filepath.Join(tmp, "profile"); dirExists(profileSrc) {
				if err := fsutil.CopyTree(profileSrc, dst); err != nil {
					return err
				}
			}
			sharedSrc := filepath.Join(tmp, "shared")
			if dirExists(sharedSrc) {
				entries, _ := os.ReadDir(sharedSrc)
				for _, e := range entries {
					target := a.SharedField(e.Name())
					install, err := fsutil.IsEmpty(target)
					if err != nil {
						return err
					}
					if install {
						if err := os.RemoveAll(target); err != nil {
							return err
						}
						if err := fsutil.CopyTree(filepath.Join(sharedSrc, e.Name()), target); err != nil {
							return err
						}
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "note: shared/%s already has content; kept it and linked the profile to it\n", e.Name())
					}
					inProfile := filepath.Join(dst, e.Name())
					if err := fsutil.ForceSymlink(target, inProfile); err != nil {
						return err
					}
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
				if err := a.creds.Write(dst, plain); err != nil {
					return err
				}
			}
			cmd.Printf("imported profile %q\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&asName, "as", "", "override profile name")
	cmd.Flags().BoolVar(&force, "force", false, "replace existing profile")
	return cmd
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
