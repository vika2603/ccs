package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/doctor"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check ccs tree consistency",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			cfg, err := config.Load(p.ConfigFile())
			if err != nil {
				return err
			}
			configured := fields.NewRegistry(cfg.Fields)
			defaults := fields.NewRegistry(config.Default().Fields)
			kc := newDoctorKeychainLister()
			findings, err := doctor.NewChecker(p, configured, defaults, kc, defaultClaudePath()).Check()
			if err != nil {
				return err
			}
			if len(findings) == 0 {
				cmd.Println("clean")
				return nil
			}
			for _, f := range findings {
				if f.Profile != "" {
					cmd.Printf("%s [%s] %s\n", f.Kind, f.Profile, f.Path)
				} else {
					cmd.Printf("%s %s (%s)\n", f.Kind, f.Path, f.Detail)
				}
			}
			return fmt.Errorf("%d finding(s)", len(findings))
		},
	}
}

func defaultClaudePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}
