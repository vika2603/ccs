package main

import (
	"bytes"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := buf.String(); got == "" {
		t.Fatalf("empty output")
	}
}
