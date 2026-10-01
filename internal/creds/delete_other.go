//go:build !darwin

package creds

// Delete is a no-op outside macOS: Claude Code keeps the login in
// <configDir>/.credentials.json, which goes away with the directory.
func Delete(string) error { return nil }
