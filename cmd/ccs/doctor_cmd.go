package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/doctor"
	"github.com/vika2603/ccs/internal/fields"
)

func newDoctorCmd() *cobra.Command {
	var fix, yes bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check ccs tree consistency (--fix to repair)",
		Long: `Check ccs tree consistency.

With --fix, recreate missing shared links and missing shared targets, and
delete orphaned env files after confirmation (-y skips it).

Orphan keychain entries are only reported: a service name carries just a hash
of its config directory, so ccs cannot tell whether that directory (for
example an adopted source still in use) is gone. Delete one manually with
  security delete-generic-password -s '<service>'
Unclassified entries, orphan shared fields, and classification drift are also
left for you to resolve.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			findings, err := a.checker().Check()
			if err != nil {
				return err
			}
			var fixErr error
			if fix && len(findings) > 0 {
				fixErr = a.fix(findings, yes, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
				if findings, err = a.checker().Check(); err != nil {
					return errors.Join(fixErr, err)
				}
			}
			if len(findings) == 0 && fixErr == nil {
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
			if len(findings) == 0 {
				return fixErr
			}
			return errors.Join(fixErr, fmt.Errorf("%d finding(s)", len(findings)))
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "repair what can be repaired automatically")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "with --fix, delete orphan env files without asking")
	return cmd
}

// fix repairs the findings that have a safe automatic remedy. Deleting orphan
// env files is destructive, so it asks first unless yes is set.
func (a app) fix(findings []doctor.Finding, yes bool, in io.Reader, out, errOut io.Writer) error {
	var orphans []doctor.Finding
	var errs []error
	for _, f := range findings {
		switch f.Kind {
		case doctor.MissingSharedLink:
			if err := a.ops().Relink(f.Profile, f.Detail); err != nil {
				errs = append(errs, err)
				continue
			}
			fmt.Fprintf(out, "fixed: linked %s in profile %s\n", f.Detail, f.Profile)
		case doctor.BrokenSymlink:
			if a.reg.Classify(f.Detail) != fields.Shared {
				continue
			}
			if target, err := os.Readlink(f.Path); err != nil || target != a.SharedField(f.Detail) {
				continue
			}
			if err := fields.CreateSharedTargets(a.SharedDir(), []fields.Classification{a.reg.Describe(f.Detail)}); err != nil {
				errs = append(errs, err)
				continue
			}
			fmt.Fprintf(out, "fixed: recreated empty shared/%s\n", f.Detail)
		case doctor.OrphanEnvFile:
			orphans = append(orphans, f)
		}
	}
	if len(orphans) > 0 {
		for _, f := range orphans {
			fmt.Fprintf(out, "orphan: %s %s\n", f.Kind, f.Path)
		}
		if !yes {
			fmt.Fprint(out, "delete these? (y/N) ")
		}
		if yes || confirmed(in) {
			for _, f := range orphans {
				if err := os.Remove(f.Path); err != nil {
					errs = append(errs, fmt.Errorf("delete %s: %w", f.Path, err))
				}
			}
		} else {
			fmt.Fprintln(errOut, "kept orphans")
		}
	}
	return errors.Join(errs...)
}
