package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
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

func TestDefaultOutputFileNameIncludesNanoseconds(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 25, 1, 2, 3, 4567, time.UTC)
	got := defaultOutputFileName(now, 0)
	want := "prompt-20260425010203-000004567.txt"
	if got != want {
		t.Fatalf("default output file name\ngot  %q\nwant %q", got, want)
	}
}

func TestWriteFileExclusiveDoesNotOverwriteExisting(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFileExclusive(path, "new"); !os.IsExist(err) {
		t.Fatalf("expected exist error, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("existing file was overwritten: %q", got)
	}
}
