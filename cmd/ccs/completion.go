package main

import (
	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/layout"
)

func completeProfileNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	m, _, err := manager()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names, err := m.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeFieldNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	p, err := layout.FromEnv()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	cfg, err := config.Load(p.ConfigFile())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	out := make([]string, 0, len(cfg.Fields.Shared)+len(cfg.Fields.Isolated)+len(cfg.Fields.Transient))
	out = append(out, cfg.Fields.Shared...)
	out = append(out, cfg.Fields.Isolated...)
	out = append(out, cfg.Fields.Transient...)
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeProfileNamesAtArg0(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return completeProfileNames(cmd, args, toComplete)
}

func completeFieldThenProfile(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completeFieldNames(cmd, args, toComplete)
	case 1:
		return completeProfileNames(cmd, args, toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}
