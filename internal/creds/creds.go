package creds

type Store interface {
	Read(profileAbsPath string) ([]byte, error)
	Write(profileAbsPath string, data []byte) error
	Delete(profileAbsPath string) error
}

var ErrNotFound = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string { return "credentials not found" }
