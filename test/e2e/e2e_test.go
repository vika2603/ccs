package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "ccs")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/ccs")
	cmd.Dir = filepath.Join("..", "..")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	return out
}

func run(t *testing.T, ccs, home string, args ...string) string {
	t.Helper()
	cmd := exec.Command(ccs, args...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", ccs, args, err, out)
	}
	return string(out)
}

func TestFullFlow(t *testing.T) {
	bin := buildBinary(t)
	home := t.TempDir()
	run(t, bin, home, "init")

	src := filepath.Join(home, "src.claude")
	os.MkdirAll(filepath.Join(src, "skills", "hello"), 0o755)
	os.WriteFile(filepath.Join(src, "skills", "hello", "SKILL.md"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(src, "CLAUDE.md"), []byte("mem"), 0o644)
	os.MkdirAll(filepath.Join(src, "projects"), 0o755)
	os.WriteFile(filepath.Join(src, "projects", "p.txt"), []byte("p"), 0o644)

	run(t, bin, home, "import", src, "main")
	run(t, bin, home, "new", "work")

	run(t, bin, home, "use", "work")
	run(t, bin, home, "fork", "skills", "work")

	out := run(t, bin, home, "status", "work")
	if !strings.Contains(out, "forked") {
		t.Errorf("status: %q", out)
	}

	exportFile := filepath.Join(t.TempDir(), "main.tar.gz")
	run(t, bin, home, "export", "main", "-o", exportFile)
	dst := t.TempDir()
	run(t, bin, dst, "init")
	run(t, bin, dst, "restore", exportFile, "--as", "main2")

	b, err := os.ReadFile(filepath.Join(dst, ".ccs", "shared", "skills", "hello", "SKILL.md"))
	if err != nil || string(b) != "hi" {
		t.Errorf("restore content: %v / %q", err, b)
	}

	if got := run(t, bin, home, "doctor"); !strings.Contains(got, "clean") && !strings.Contains(got, "orphan-shared-field") {
		t.Errorf("doctor: %q", got)
	}
}
