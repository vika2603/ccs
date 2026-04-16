package state

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestReadMissingIsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "active")
	got, err := Read(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "" {
		t.Errorf("got %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "active")
	if err := Write(p, "work"); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Read(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "work" {
		t.Errorf("got %q", got)
	}
}

func TestClear(t *testing.T) {
	p := filepath.Join(t.TempDir(), "active")
	Write(p, "work")
	if err := Clear(p); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ := Read(p)
	if got != "" {
		t.Errorf("expected empty after clear, got %q", got)
	}
}

func TestConcurrentWritesDoNotCorrupt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "active")
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			name := "p"
			if i%2 == 0 {
				name = "q"
			}
			_ = Write(p, name)
		}(i)
	}
	wg.Wait()
	got, err := Read(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != "p" && got != "q" {
		t.Errorf("corrupted value: %q", got)
	}
}

func TestWriteLockRetriesThenFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "active")
	held := flock.New(p + ".lock")
	ok, err := held.TryLock()
	if err != nil || !ok {
		t.Fatalf("hold lock: %v %v", ok, err)
	}
	defer held.Unlock()

	start := time.Now()
	err = Write(p, "work")
	if err == nil {
		t.Fatalf("expected lock timeout")
	}
	if time.Since(start) > 600*time.Millisecond {
		t.Fatalf("write should fail within ~600ms, got %v", time.Since(start))
	}
}

func TestRejectInvalidName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "active")
	if err := Write(p, "bad name"); err == nil {
		t.Fatalf("expected error for name with space")
	}
	if err := Write(p, "../esc"); err == nil {
		t.Fatalf("expected error for name with path separator")
	}
}
