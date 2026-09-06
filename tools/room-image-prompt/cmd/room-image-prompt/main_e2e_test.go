package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%v\nstderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	got := strings.TrimSpace(stdout.String())
	abs, err := filepath.Abs(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("stdout path\ngot  %q\nwant %q", got, abs)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "出力: p.txt") {
		t.Fatalf("stderr should log basename\ngot %q", errOut)
	}
	okCopy := strings.Contains(errOut, "クリップボードにコピーしました")
	failCopy := strings.Contains(errOut, "コピーに失敗しました")
	if !okCopy && !failCopy {
		t.Fatalf("stderr should log clipboard result\ngot %q", errOut)
	}
	if okCopy && failCopy {
		t.Fatalf("stderr should contain only one clipboard status\ngot %q", errOut)
	}

	body, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "写真風、3D建築レンダリング風、フォトリアル表現にはしないでください。") {
		t.Fatalf("default output should keep legacy style:\n%s", body)
	}
}

func TestCLI_StyleFile(t *testing.T) {
	t.Parallel()

	dir := moduleRoot(t)
	tmp := t.TempDir()
	styleFile := filepath.Join(tmp, "custom-style.txt")
	if err := os.WriteFile(styleFile, []byte("CUSTOM_STYLE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(tmp, "custom.txt")

	cmd := exec.Command(
		"go", "run", "./cmd/room-image-prompt",
		"-seed", "1",
		"-style-file", styleFile,
		"-out", outFile,
	)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	body, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "CUSTOM_STYLE") {
		t.Fatalf("custom style was not injected:\n%s", got)
	}
	if strings.Contains(got, "写真風、3D建築レンダリング風、フォトリアル表現にはしないでください。") {
		t.Fatalf("legacy style should not be injected with -style-file:\n%s", got)
	}
}

func TestResolveStyle(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"style_legacy.txt": {Data: []byte("LEGACY_STYLE\n")},
	}
	customPath := filepath.Join(t.TempDir(), "custom-style.txt")
	if err := os.WriteFile(customPath, []byte("CUSTOM_STYLE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		styleName string
		styleFile string
		want      string
		wantErr   bool
	}{
		{name: "default legacy", want: "LEGACY_STYLE\n"},
		{name: "explicit legacy", styleName: legacyStyleName, want: "LEGACY_STYLE\n"},
		{name: "custom file", styleFile: customPath, want: "CUSTOM_STYLE\n"},
		{name: "conflicting sources", styleName: legacyStyleName, styleFile: customPath, wantErr: true},
		{name: "unsupported named style", styleName: "direction-a", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveStyle(fsys, tt.styleName, tt.styleFile)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("style mismatch: got %q want %q", got, tt.want)
			}
		})
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
	if err := writeFileExclusive(path, "new"); !errors.Is(err, os.ErrExist) {
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
