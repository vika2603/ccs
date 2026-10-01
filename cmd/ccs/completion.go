package main

import (
	"github.com/spf13/cobra"
)

func completeProfileNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	a, err := loadApp()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names, err := a.mgr.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func completeFieldNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	a, err := loadApp()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	cfg := a.cfg
	out := make([]string, 0, len(cfg.Shared)+len(cfg.Isolated)+len(cfg.Export.Exclude))
	out = append(out, cfg.Shared...)
	out = append(out, cfg.Isolated...)
	out = append(out, cfg.Export.Exclude...)
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
