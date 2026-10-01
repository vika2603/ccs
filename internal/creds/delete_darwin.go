//go:build darwin

package creds

import (
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

// Delete removes the Keychain item holding the login for configDir. A
// missing item is not an error.
func Delete(configDir string) error {
	service, err := serviceName(configDir)
	if err != nil {
		return err
	}
	u, err := user.Current()
	if err != nil {
		return err
	}
	cmd := exec.Command("/usr/bin/security", "delete-generic-password", "-s", service, "-a", u.Username)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "could not be found") {
			return nil
		}
		return fmt.Errorf("security delete-generic-password: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
