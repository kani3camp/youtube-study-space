package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatalf("go.mod が見つかりません (wd=%s)", wd)
	return ""
}

func TestCLI_Version_TC_E1(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/room-image-prompt", "-version")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	s := strings.TrimSpace(string(out))
	if !strings.Contains(s, "room-image-prompt") {
		t.Fatalf("unexpected output: %q", s)
	}
}

func TestCLI_StdoutPath_TC_C2_extension(t *testing.T) {
	t.Parallel()
	dir := moduleRoot(t)
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "p.txt")
	cmd := exec.Command("go", "run", "./cmd/room-image-prompt", "-seed", "1", "-out", outFile)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%v", err)
	}
	got := strings.TrimSpace(string(out))
	abs, err := filepath.Abs(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("stdout path\ngot  %q\nwant %q", got, abs)
	}
}
