//go:build darwin

package creds

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

type keychainStore struct{ user string }

func NewKeychainStore() Store {
	u, err := user.Current()
	name := ""
	if err == nil {
		name = u.Username
	}
	return keychainStore{user: name}
}

func New() Store { return NewKeychainStore() }

func (k keychainStore) Read(profile string) ([]byte, error) {
	service, err := ServiceName(profile, DefaultClaudeDir())
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("/usr/bin/security", "find-generic-password",
		"-s", service, "-a", k.user, "-w")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "could not be found") ||
			strings.Contains(stderr.String(), "item could not be found") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("security find: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}

func (k keychainStore) Write(profile string, data []byte) error {
	service, err := ServiceName(profile, DefaultClaudeDir())
	if err != nil {
		return err
	}
	cmd := exec.Command("/usr/bin/security", "add-generic-password",
		"-s", service, "-a", k.user, "-w", string(data), "-U")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("security add: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (k keychainStore) Delete(profile string) error {
	service, err := ServiceName(profile, DefaultClaudeDir())
	if err != nil {
		return err
	}
	return k.deleteService(service)
}

func (k keychainStore) deleteService(service string) error {
	cmd := exec.Command("/usr/bin/security", "delete-generic-password",
		"-s", service, "-a", k.user)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		s := stderr.String()
		if strings.Contains(s, "could not be found") {
			return nil
		}
		return fmt.Errorf("security delete: %w: %s", err, strings.TrimSpace(s))
	}
	return nil
}

func (k keychainStore) Exists(profile string) (bool, error) {
	_, err := k.Read(profile)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return false, err
}

// ListServices returns the service names of all generic-password items in the
// user's default keychain.
func ListServices() ([]string, error) {
	out, err := exec.Command("/usr/bin/security", "dump-keychain").Output()
	if err != nil {
		return nil, err
	}
	var services []string
	for l := range strings.SplitSeq(string(out), "\n") {
		if svc, ok := strings.CutPrefix(strings.TrimSpace(l), `"svce"<blob>="`); ok {
			services = append(services, strings.TrimSuffix(svc, `"`))
		}
	}
	return services, nil
}
