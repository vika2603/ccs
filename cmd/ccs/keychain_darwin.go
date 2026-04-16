//go:build darwin

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/layout"
)

func newKeychainPruneCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove Keychain entries whose profile directory no longer exists",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			profiles, _ := os.ReadDir(p.ProfilesDir())
			known := map[string]bool{}
			for _, e := range profiles {
				if !e.IsDir() {
					continue
				}
				svc, err := creds.ServiceName(filepath.Join(p.ProfilesDir(), e.Name()), defaultClaudePath())
				if err == nil {
					known[svc] = true
				}
			}
			entries, err := dumpKeychainServices()
			if err != nil {
				return err
			}
			u, _ := user.Current()
			orphans := []string{}
			for _, svc := range entries {
				if strings.HasPrefix(svc, "Claude Code-credentials") && !known[svc] && svc != "Claude Code-credentials" {
					orphans = append(orphans, svc)
				}
			}
			if len(orphans) == 0 {
				cmd.Println("no orphans")
				return nil
			}
			for _, svc := range orphans {
				cmd.Println("orphan:", svc)
			}
			if !yes {
				cmd.Print("delete these? (y/N) ")
				ans, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.TrimSpace(ans) != "y" {
					return fmt.Errorf("aborted")
				}
			}
			for _, svc := range orphans {
				_ = exec.Command("/usr/bin/security", "delete-generic-password",
					"-s", svc, "-a", u.Username).Run()
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	return cmd
}

func dumpKeychainServices() ([]string, error) {
	out, err := exec.Command("/usr/bin/security", "dump-keychain").Output()
	if err != nil {
		return nil, err
	}
	var services []string
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "\"svce\"<blob>=\"") {
			svc := strings.TrimPrefix(l, "\"svce\"<blob>=\"")
			svc = strings.TrimSuffix(svc, "\"")
			services = append(services, svc)
		}
	}
	return services, nil
}
