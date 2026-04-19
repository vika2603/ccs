package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/archive"
	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profile"
	"github.com/vika2603/ccs/internal/state"
)

func newBackupCmd() *cobra.Command {
	var outFile string
	var force bool
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up the entire ccs directory to a single tar.gz",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			if _, err := os.Stat(p.Root()); err != nil {
				return fmt.Errorf("ccs root %s does not exist; run `ccs init` first", p.Root())
			}
			cfg, err := config.Load(p.ConfigFile())
			if err != nil {
				return err
			}
			reg := fields.NewRegistry(cfg)
			mgr := profile.NewManager(p, reg).WithCreds(creds.New())

			profiles, err := mgr.List()
			if err != nil {
				return err
			}
			active, _ := state.Read(p.ActiveFile())

			store := creds.New()
			bundle := map[string]string{}
			for _, name := range profiles {
				dir := p.ProfilePath(name)
				data, err := store.Read(dir)
				if errors.Is(err, creds.ErrNotFound) {
					continue
				}
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: read credentials for %q: %v\n", name, err)
					continue
				}
				bundle[name] = base64.StdEncoding.EncodeToString(data)
			}
			plain, err := json.Marshal(bundle)
			if err != nil {
				return err
			}
			pass, err := readPassphrase("Passphrase for credentials: ", true)
			if err != nil {
				return err
			}
			enc, err := archive.EncryptPassphrase(plain, pass)
			if err != nil {
				return err
			}

			if outFile == "" {
				outFile = fmt.Sprintf("ccs-backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))
			}
			if _, err := os.Stat(outFile); err == nil {
				if !force {
					return fmt.Errorf("%s already exists (use --force)", outFile)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			manifest := archive.BackupManifest{
				Version:        1,
				Type:           archive.BackupType,
				SourcePlatform: runtime.GOOS,
				Active:         active,
				Profiles:       profiles,
				Shared:         cfg.Shared,
				Isolated:       cfg.Isolated,
				Exclude:        cfg.Export.Exclude,
			}
			opts := archive.BackupPackOptions{
				CCSRoot:           p.Root(),
				Profiles:          profiles,
				PerProfileExclude: cfg.Export.Exclude,
				ConfigPath:        p.ConfigFile(),
				EnvDir:            p.EnvDir(),
				SharedDir:         p.SharedDir(),
				Credentials:       enc,
				Manifest:          manifest,
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Scope of protection: the tarball itself and backup-manifest.json are plaintext; config.toml, CLAUDE.md, skills, commands, env files, settings.json, and each profile's .claude.json (which carries the account identity) are all readable without the passphrase. Only the OAuth tokens are encrypted.")

			if err := archive.PackBackup(outFile, opts); err != nil {
				return err
			}

			abs, err := filepath.Abs(outFile)
			if err != nil {
				abs = outFile
			}
			cmd.Printf("wrote %s (%d profiles, %d with credentials)\n", abs, len(profiles), len(bundle))
			return nil
		},
	}
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "output file (default: ccs-backup-<timestamp>.tar.gz)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing output file")
	return cmd
}
