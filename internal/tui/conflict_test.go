package tui

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestPromptConflictOverwrite(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("o\n"))
	var out bytes.Buffer
	got, err := PromptConflict(Entry{Name: "skills"}, Entry{Name: "skills"}, &out, in)
	if err != nil {
		t.Fatalf("PromptConflict: %v", err)
	}
	if got != ResolveOverwrite {
		t.Fatalf("got %v", got)
	}
}

func TestPromptConflictAbort(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("a\n"))
	var out bytes.Buffer
	got, err := PromptConflict(Entry{Name: "skills"}, Entry{Name: "skills"}, &out, in)
	if err != nil {
		t.Fatalf("PromptConflict: %v", err)
	}
	if got != ResolveAbort {
		t.Fatalf("got %v", got)
	}
}

func TestPromptConflictShowDiffRedisplaysPrompt(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("d\no\n"))
	var out bytes.Buffer
	got, err := PromptConflict(Entry{Name: "skills"}, Entry{Name: "skills"}, &out, in)
	if err != nil {
		t.Fatalf("PromptConflict: %v", err)
	}
	if got != ResolveOverwrite {
		t.Fatalf("got %v", got)
	}
	if strings.Count(out.String(), "overwrite/abort/diff") < 2 {
		t.Fatalf("expected prompt to be re-displayed after diff")
	}
}

func TestPromptConflictNonTTYFailsClosed(t *testing.T) {
	var out bytes.Buffer
	got, err := PromptConflict(Entry{Name: "skills"}, Entry{Name: "skills"}, &out, strings.NewReader("o\n"))
	if err != nil {
		t.Fatalf("PromptConflict: %v", err)
	}
	if got != ResolveAbort {
		t.Fatalf("expected fail-closed abort, got %v", got)
	}
}
