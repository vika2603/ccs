package main

import (
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
	"github.com/vika2603/ccs/internal/link"
	"github.com/vika2603/ccs/internal/state"
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
			p, err := layout.FromEnv()
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
				if err := state.ValidName(asName); err != nil {
					return err
				}
				name = asName
			}
			dst := p.ProfilePath(name)
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
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := moveTree(filepath.Join(tmp, "profile"), dst); err != nil {
				return err
			}
			sharedSrc := filepath.Join(tmp, "shared")
			if _, err := os.Stat(sharedSrc); err == nil {
				entries, _ := os.ReadDir(sharedSrc)
				for _, e := range entries {
					target := p.SharedField(e.Name())
					install, err := sharedSlotAvailable(target)
					if err != nil {
						return err
					}
					if install {
						if err := os.RemoveAll(target); err != nil {
							return err
						}
						if err := moveTree(filepath.Join(sharedSrc, e.Name()), target); err != nil {
							return err
						}
					}
					inProfile := filepath.Join(dst, e.Name())
					if err := link.ReplaceCopyWithSymlink(inProfile, target); err != nil {
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
				if err := creds.New().Write(dst, plain); err != nil {
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

func sharedSlotAvailable(target string) (bool, error) {
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return info.Size() == 0, nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func moveTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
