package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/vika2603/ccs/internal/archive"
	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/tui/picker"
)

func newExportCmd() *cobra.Command {
	var outFile string
	var full, withCreds, interactive bool
	cmd := &cobra.Command{
		Use:               "export <name>",
		Short:             "Export a profile to a tar.gz",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			cfg, err := config.Load(p.ConfigFile())
			if err != nil {
				return err
			}
			reg := fields.NewRegistry(cfg)

			profileDir := p.ProfilePath(name)
			if _, err := os.Stat(profileDir); err != nil {
				return err
			}

			seedPreset := fields.PresetDefault
			switch {
			case full:
				seedPreset = fields.PresetFull
			case withCreds:
				seedPreset = fields.PresetWithCreds
			}

			var entryNames []string
			var includeCredentials bool
			includesHistoryFlag := full

			if interactive {
				if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
					return fmt.Errorf("-i requires a TTY on stdin and stdout; drop -i and use --full or --with-credentials")
				}
				rawItems, err := fields.ScanProfile(profileDir, reg)
				if err != nil {
					return fmt.Errorf("scan profile: %w", err)
				}
				items := rawItems[:0]
				for _, it := range rawItems {
					if reg.IsExcludedFromExport(it.Name) {
						continue
					}
					items = append(items, it)
				}
				seed := fields.PresetSelection(items, seedPreset)
				if len(seed.Entries) == 0 && !seed.Credentials {
					fmt.Fprintf(cmd.ErrOrStderr(), "ccs: nothing to export in profile %q\n", name)
					return fmt.Errorf("nothing to export")
				}
				result, err := picker.RunPicker(picker.Input{
					Items:           items,
					SeedSelection:   seed.Entries,
					SeedCredentials: seed.Credentials,
					ProfileName:     name,
					PresetLabel:     seedPreset.String(),
				})
				if err != nil {
					if errors.Is(err, picker.ErrSIGINT) {
						os.Exit(130)
					}
					return err
				}
				if result.Cancelled {
					return fmt.Errorf("export cancelled")
				}
				entryNames = result.Names
				includeCredentials = result.Credentials
			} else {
				mode := fields.ExportDefault
				switch {
				case full:
					mode = fields.ExportFull
				case withCreds:
					mode = fields.ExportWithCredentials
				}
				selected, err := fields.SelectExportMaterial(profileDir, reg, mode)
				if err != nil {
					return fmt.Errorf("select export material: %w", err)
				}
				entryNames = make([]string, 0, len(selected))
				for _, e := range selected {
					entryNames = append(entryNames, e.Name)
				}
				if withCreds || full {
					claudeJSON := filepath.Join(profileDir, ".claude.json")
					if _, err := os.Stat(claudeJSON); err == nil {
						entryNames = append(entryNames, ".claude.json")
					}
				}
				includeCredentials = withCreds
			}

			sharedPaths := map[string]string{}
			for _, f := range reg.Shared() {
				local := filepath.Join(profileDir, f.Name)
				info, err := os.Lstat(local)
				if err != nil {
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					sharedPaths[f.Name] = p.SharedField(f.Name)
				}
			}

			m := archive.Manifest{
				Version:             1,
				Profile:             name,
				SourcePlatform:      runtime.GOOS,
				IncludesCredentials: includeCredentials,
				IncludesHistory:     includesHistoryFlag,
				Fields: map[string][]string{
					"shared":   cfg.Shared,
					"isolated": cfg.Isolated,
					"excluded": cfg.Export.Exclude,
				},
			}
			opts := archive.PackOptions{
				ProfileDir:     profileDir,
				ProfileName:    name,
				ProfileEntries: entryNames,
				SharedPaths:    sharedPaths,
				Manifest:       m,
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Scope of protection: the tarball itself and manifest.json are plaintext; profile name, field classification, the user's CLAUDE.md, skills, commands, optional runtime data, and .claude.json (which carries the account identity -- email, user ID, organization/tenant identifiers) are all readable without the passphrase. Only the OAuth token is encrypted.")

			if includeCredentials {
				data, err := creds.New().Read(profileDir)
				if err != nil {
					return fmt.Errorf("read credentials: %w", err)
				}
				pass, err := readPassphrase("Passphrase for credentials: ", true)
				if err != nil {
					return err
				}
				enc, err := archive.EncryptPassphrase(data, pass)
				if err != nil {
					return err
				}
				opts.Credentials = enc
			}

			if outFile == "" {
				outFile = name + ".tar.gz"
			}
			if err := archive.Pack(outFile, opts); err != nil {
				return err
			}
			cmd.Printf("wrote %s\n", outFile)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outFile, "output", "o", "", "output file (default: <name>.tar.gz)")
	cmd.Flags().BoolVar(&full, "full", false, "include isolated runtime data (projects, todos, history.jsonl)")
	cmd.Flags().BoolVar(&withCreds, "with-credentials", false, "include the OAuth token, passphrase-encrypted")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "pick entries interactively (TTY required)")
	return cmd
}

func readPassphrase(prompt string, confirm bool) (string, error) {
	if v, ok := os.LookupEnv("CCS_PASSPHRASE"); ok {
		return v, nil
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr)
	if confirm {
		fmt.Fprint(os.Stderr, "Confirm: ")
		b2, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		fmt.Fprintln(os.Stderr)
		if string(b) != string(b2) {
			return "", fmt.Errorf("passphrases do not match")
		}
	}
	return string(b), nil
}
