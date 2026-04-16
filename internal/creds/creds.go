package creds

import (
	"bytes"
	"fmt"
)

type Store interface {
	Read(profileAbsPath string) ([]byte, error)
	Write(profileAbsPath string, data []byte) error
	Delete(profileAbsPath string) error
	Exists(profileAbsPath string) (bool, error)
}

var ErrNotFound = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string { return "credentials not found" }

// Migrate moves credentials from oldProfile to newProfile.
// Contract: write new first, verify new is readable and identical, then delete old.
func Migrate(s Store, oldProfile, newProfile, defaultPath string) error {
	data, err := s.Read(oldProfile)
	if err == ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.Write(newProfile, data); err != nil {
		return err
	}
	verify, err := s.Read(newProfile)
	if err != nil || !bytes.Equal(verify, data) {
		oldService, _ := ServiceName(oldProfile, defaultPath)
		newService, _ := ServiceName(newProfile, defaultPath)
		return fmt.Errorf("credential migration verification failed: kept %s and refused to delete %s", oldService, newService)
	}
	return s.Delete(oldProfile)
}
