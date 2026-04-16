//go:build linux

package creds

import (
	"errors"
	"os"
	"path/filepath"
)

const credFilename = ".credentials.json"

type fileStore struct{}

func NewFileStore() Store { return fileStore{} }
func New() Store          { return fileStore{} }

func (fileStore) path(profile string) string {
	return filepath.Join(profile, credFilename)
}

func (s fileStore) Read(profile string) ([]byte, error) {
	b, err := os.ReadFile(s.path(profile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return b, err
}

func (s fileStore) Write(profile string, data []byte) error {
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path(profile), data, 0o600)
}

func (s fileStore) Delete(profile string) error {
	err := os.Remove(s.path(profile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s fileStore) Exists(profile string) (bool, error) {
	_, err := os.Stat(s.path(profile))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
