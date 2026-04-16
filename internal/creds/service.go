package creds

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

const servicePrefix = "Claude Code-credentials"

func ServiceName(path, defaultPath string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonicalPath := filepath.Clean(absPath)

	absDefault, err := filepath.Abs(defaultPath)
	if err != nil {
		return "", err
	}
	canonicalDefault := filepath.Clean(absDefault)
	if canonicalPath == canonicalDefault {
		return servicePrefix, nil
	}

	sum := sha256.Sum256([]byte(canonicalPath))
	return servicePrefix + "-" + hex.EncodeToString(sum[:])[:8], nil
}

func sha8(s string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(s)))
	return hex.EncodeToString(sum[:])[:8]
}
