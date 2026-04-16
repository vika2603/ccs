package main

import (
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
)

func newExportCmd() *cobra.Command {
	var outFile string
	var full, withCreds bool
	cmd := &cobra.Command{
		Use:   "export <name>",
		Short: "Export a profile to a tar.gz",
		Args:  cobra.ExactArgs(1),
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
			reg := fields.NewRegistry(cfg.Fields)

			profileDir := p.ProfilePath(name)
			if _, err := os.Stat(profileDir); err != nil {
				return err
			}

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
			entryNames := make([]string, 0, len(selected))
			for _, e := range selected {
				entryNames = append(entryNames, e.Name)
			}
			if withCreds || full {
				claudeJSON := filepath.Join(profileDir, ".claude.json")
				if _, err := os.Stat(claudeJSON); err == nil {
					entryNames = append(entryNames, ".claude.json")
				}
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
				IncludesCredentials: withCreds,
				IncludesHistory:     full,
				Fields: map[string][]string{
					"shared":    cfg.Fields.Shared,
					"isolated":  cfg.Fields.Isolated,
					"transient": cfg.Fields.Transient,
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

			if withCreds {
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
	return cmd
}

func readPassphrase(prompt string, confirm bool) (string, error) {
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
