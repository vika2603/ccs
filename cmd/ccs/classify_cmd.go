package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/layout"
)

func newClassifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "classify <name> <category>",
		Short: "Add an unclassified entry to a category in config.toml",
		Long:  "Append <name> to the top-level <category> array in config.toml. Category must be one of: shared, isolated.",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 1 {
				return []string{"shared", "isolated"}, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			category := args[1]
			if name == "" {
				return fmt.Errorf("name must not be empty")
			}
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			cfg, err := config.Load(p.ConfigFile())
			if err != nil {
				return err
			}
			if existing, ok := existingCategory(cfg, name); ok {
				return fmt.Errorf("%q is already classified as %s", name, existing)
			}
			switch category {
			case "shared":
				cfg.Shared = append(cfg.Shared, name)
			case "isolated":
				cfg.Isolated = append(cfg.Isolated, name)
			default:
				return fmt.Errorf("invalid category %q; must be shared or isolated", category)
			}
			if err := config.Save(p.ConfigFile(), cfg); err != nil {
				return err
			}
			cmd.Printf("classified %s as %s\n", name, category)
			return nil
		},
	}
}

func existingCategory(c config.Config, name string) (string, bool) {
	for _, v := range c.Shared {
		if v == name {
			return "shared", true
		}
	}
	for _, v := range c.Isolated {
		if v == name {
			return "isolated", true
		}
	}
	return "", false
}
