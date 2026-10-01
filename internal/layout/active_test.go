package layout

import (
	"os"
	"testing"
)

func TestActiveRoundTrip(t *testing.T) {
	p := New(t.TempDir())
	if got, err := p.Active(); err != nil || got != "" {
		t.Fatalf("missing state should read as empty: %q %v", got, err)
	}
	if err := p.SetActive("work"); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.Active(); got != "work" {
		t.Errorf("got %q", got)
	}
	if err := p.ClearActive(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ActiveFile()); !os.IsNotExist(err) {
		t.Errorf("state file should be removed: %v", err)
	}
	if err := p.ClearActive(); err != nil {
		t.Errorf("clearing twice: %v", err)
	}
}

func TestSetActiveRejectsInvalidName(t *testing.T) {
	p := New(t.TempDir())
	for _, name := range []string{"bad name", "../esc", "default"} {
		if err := p.SetActive(name); err == nil {
			t.Errorf("SetActive(%q) should fail", name)
		}
	}
}
