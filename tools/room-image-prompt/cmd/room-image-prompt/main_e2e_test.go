package main

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kani3camp/youtube-study-space/tools/room-image-prompt/data"
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
	if !strings.Contains(string(body), "## Look profile:") {
		t.Fatalf("default output should auto-select a bundled Look:\n%s", body)
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
		"-look", "none",
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
		"style_legacy.txt":                {Data: []byte("LEGACY_STYLE\n")},
		"style_direction_a.generated.txt": {Data: []byte("DIRECTION_A\n")},
	}
	customPath := filepath.Join(t.TempDir(), "custom-style.txt")
	if err := os.WriteFile(customPath, []byte("CUSTOM_STYLE\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyPath := filepath.Join(t.TempDir(), "empty-style.txt")
	if err := os.WriteFile(emptyPath, []byte(" \r\n\t"), 0o644); err != nil {
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
		{name: "custom file normalizes CRLF", styleFile: customPath, want: "CUSTOM_STYLE\n"},
		{name: "empty custom file", styleFile: emptyPath, wantErr: true},
		{name: "direction style", styleName: "direction-a", want: "DIRECTION_A\n"},
		{name: "conflicting sources", styleName: legacyStyleName, styleFile: customPath, wantErr: true},
		{name: "missing direction style", styleName: "direction-z", wantErr: true},
		{name: "unsupported named style", styleName: "other", wantErr: true},
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

func TestResolveLook(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"look_indigo_violet_fantasy.txt": {Data: []byte("INDIGO\n")},
		"look_airy_garden.txt":           {Data: []byte("AIRY\n")},
		"look_coral_aqua_glow.txt":       {Data: []byte("CORAL\n")},
		"look_crystal_lucent.txt":        {Data: []byte("CRYSTAL\n")},
	}
	customPath := filepath.Join(t.TempDir(), "custom-look.txt")
	if err := os.WriteFile(customPath, []byte("CUSTOM_LOOK\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		lookName string
		lookFile string
		wantName string
		wantText string
		wantAuto bool
		wantErr  bool
	}{
		{name: "default auto", wantAuto: true},
		{name: "explicit auto", lookName: "auto", wantAuto: true},
		{name: "none", lookName: "none", wantName: "none"},
		{name: "named", lookName: "airy-garden", wantName: "airy-garden", wantText: "AIRY\n"},
		{name: "custom file", lookFile: customPath, wantName: "custom", wantText: "CUSTOM_LOOK\n"},
		{name: "conflicting sources", lookName: "airy-garden", lookFile: customPath, wantErr: true},
		{name: "invalid", lookName: "other", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveLook(fsys, tt.lookName, tt.lookFile, rand.New(rand.NewPCG(42, 0)))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantAuto {
				if got.Name == "auto" || got.Name == "none" || strings.TrimSpace(got.Text) == "" {
					t.Fatalf("auto Look was not resolved: %+v", got)
				}
				return
			}
			if got.Name != tt.wantName || got.Text != tt.wantText {
				t.Fatalf("look mismatch: got %+v want name=%q text=%q", got, tt.wantName, tt.wantText)
			}
		})
	}
}

func TestComposeVisualGuidance(t *testing.T) {
	t.Parallel()

	if got := composeVisualGuidance("STYLE\n", ""); got != "STYLE\n" {
		t.Fatalf("none Look should preserve style exactly: %q", got)
	}
	got := composeVisualGuidance("STYLE\r\n", "LOOK\r\n")
	want := "STYLE\n\nLOOK\n"
	if got != want {
		t.Fatalf("composed guidance mismatch: got %q want %q", got, want)
	}
}

func TestCLI_LookProfile(t *testing.T) {
	t.Parallel()

	dir := moduleRoot(t)
	outFile := filepath.Join(t.TempDir(), "look.txt")
	cmd := exec.Command(
		"go", "run", "./cmd/room-image-prompt",
		"-seed", "1",
		"-style", "direction-d",
		"-look", "crystal-lucent",
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
	for _, required := range []string{
		"clean 2D digital environment illustration",
		"## Look profile: Crystal Lucent",
		"pale cyan, mint, milky white",
	} {
		if !strings.Contains(got, required) {
			t.Fatalf("combined Direction + Look output is missing %q:\n%s", required, got)
		}
	}
}

func TestResolveBundledDirectionStyles(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"direction-a", "direction-b", "direction-c", "direction-d"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			style, err := resolveStyle(data.FS, name, "")
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(style) == "" {
				t.Fatalf("style %q is empty", name)
			}
			if strings.Contains(style, "写真風、3D建築レンダリング風、フォトリアル表現にはしないでください。") {
				t.Fatalf("style %q unexpectedly contains legacy style: %s", name, style)
			}
		})
	}
}

func TestDirectionCBalancesPastelHueFamilies(t *testing.T) {
	t.Parallel()

	style, err := resolveStyle(data.FS, "direction-c", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"broad hue variety across cool and warm families",
		"lavender or periwinkle may appear only as small optional accents",
		"do not tint walls, floor, background and lighting all toward lavender, violet or purple",
		"avoid monochromatic or purple-biased palettes",
	} {
		if !strings.Contains(style, required) {
			t.Fatalf("direction-c style is missing %q:\n%s", required, style)
		}
	}
	for _, obsolete := range []string{
		"pale peach and periwinkle",
		"avoid neon-purple default palette",
	} {
		if strings.Contains(style, obsolete) {
			t.Fatalf("direction-c style still contains purple-biasing guidance %q:\n%s", obsolete, style)
		}
	}
}

func TestPurpleLooksRespectPastelDirections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		required string
	}{
		{
			name:     "indigo-violet-fantasy",
			required: "keep large surfaces within that Direction's broader pastel range",
		},
		{
			name:     "crystal-lucent",
			required: "do not let lavender, violet or periwinkle dominate",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveLook(data.FS, tt.name, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got.Text, tt.required) {
				t.Fatalf("look %q is missing pastel compatibility guidance %q:\n%s", tt.name, tt.required, got.Text)
			}
		})
	}
}

func TestAiryGardenLookDoesNotInjectSceneContent(t *testing.T) {
	t.Parallel()

	got, err := resolveLook(data.FS, "airy-garden", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"fantasy-garden",
		"stone, tile or architectural surfaces",
		"bookshelves, wood furniture",
	} {
		if strings.Contains(got.Text, forbidden) {
			t.Fatalf("airy-garden Look still injects scene content %q:\n%s", forbidden, got.Text)
		}
	}
	if !strings.Contains(got.Text, "do not introduce scenery, architecture, furniture or vegetation") {
		t.Fatalf("airy-garden Look is missing the no-content-injection contract:\n%s", got.Text)
	}
}

func TestDirectionDUsesConcreteRenderingOperations(t *testing.T) {
	t.Parallel()

	style, err := resolveStyle(data.FS, "direction-d", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"clean 2D digital environment illustration",
		"smooth matte color planes",
		"roughly 3〜4 value levels",
		"avoid continuous airbrushed gradients",
		"keep edges crisp and object separation clear",
		"omit wood grain or reduce it to only a few graphic lines",
		"reduce reflections to a small number of simplified color shapes",
		"avoid dull, muddy or washed-out color",
	} {
		if !strings.Contains(style, required) {
			t.Fatalf("direction-d style is missing %q:\n%s", required, style)
		}
	}
	for _, obsolete := range []string{
		"hospitality marketing art",
		"real-estate / showroom render",
		"oversized sculptural lighting",
		"monomaterial beige / white / pale-wood minimalism",
	} {
		if strings.Contains(style, obsolete) {
			t.Fatalf("direction-d style still contains obsolete Codex-specific workaround %q:\n%s", obsolete, style)
		}
	}
}

func TestCLI_DirectionStyle(t *testing.T) {
	t.Parallel()

	dir := moduleRoot(t)
	outFile := filepath.Join(t.TempDir(), "direction-a.txt")
	cmd := exec.Command(
		"go", "run", "./cmd/room-image-prompt",
		"-seed", "1",
		"-style", "direction-a",
		"-look", "none",
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
	if !strings.Contains(got, "premium game environment art") {
		t.Fatalf("direction-a style was not injected:\n%s", got)
	}
	if strings.Contains(got, "写真風、3D建築レンダリング風、フォトリアル表現にはしないでください。") {
		t.Fatalf("legacy style should not be injected with direction-a:\n%s", got)
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
