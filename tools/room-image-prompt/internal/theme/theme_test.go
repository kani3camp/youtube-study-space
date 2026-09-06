package theme

import (
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func testdataDir(t *testing.T, name string) fs.FS {
	t.Helper()
	p := filepath.Join("testdata", name)
	return os.DirFS(p)
}

func TestBuildTheme_SeatCountInRange(t *testing.T) {
	t.Parallel()
	fsys := testdataDir(t, "build_single")
	for i := range 1000 {
		r := rand.New(rand.NewPCG(uint64(i), 0))
		th, err := BuildTheme(fsys, r)
		if err != nil {
			t.Fatalf("i=%d: %v", i, err)
		}
		if th.SeatCount < seatCountMin || th.SeatCount > seatCountMax {
			t.Fatalf("i=%d: SeatCount=%d, want in [%d,%d]", i, th.SeatCount, seatCountMin, seatCountMax)
		}
	}
}

func TestBuildTheme_TC_B1(t *testing.T) {
	t.Parallel()
	fsys := testdataDir(t, "build_single")
	r := rand.New(rand.NewPCG(1, 0))
	th, err := BuildTheme(fsys, r)
	if err != nil {
		t.Fatal(err)
	}
	if th.World != "only_world" || th.TimeOfDay != "only_tod" || th.WorkspaceType != "only_space" ||
		th.SeatLayout != "only_layout" {
		t.Fatalf("unexpected theme: %+v", th)
	}
	if th.SeatCount != 14 {
		t.Fatalf("unexpected SeatCount: %d (want 14 for PCG(1,0))", th.SeatCount)
	}
}

func TestBuildTheme_TC_B2(t *testing.T) {
	t.Parallel()
	fsys := testdataDir(t, "build_multi")
	r := rand.New(rand.NewPCG(42, 0))
	th, err := BuildTheme(fsys, r)
	if err != nil {
		t.Fatal(err)
	}
	got := th.FormatThemeBlock()
	wantBytes, err := fs.ReadFile(fsys, "expected_theme.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(string(wantBytes), "\r\n", "\n")
	if got != want {
		t.Fatalf("theme block mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderFinal_TC_C1(t *testing.T) {
	t.Parallel()
	fsys := testdataDir(t, "build_single")
	tmpl, err := ReadTemplate(fsys)
	if err != nil {
		t.Fatal(err)
	}
	style, err := ReadLegacyStyle(fsys)
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err = ApplyStyle(tmpl, style)
	if err != nil {
		t.Fatal(err)
	}
	th, err := BuildTheme(fsys, rand.New(rand.NewPCG(1, 0)))
	if err != nil {
		t.Fatal(err)
	}
	got := RenderFinal(tmpl, th.FormatThemeBlock())
	wantBytes, err := fs.ReadFile(fsys, "expected_final.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(string(wantBytes), "\r\n", "\n")
	if got != want {
		t.Fatalf("final mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestBuildTheme_TC_D1_missing(t *testing.T) {
	t.Parallel()
	fsys := testdataDir(t, "err_missing")
	r := rand.New(rand.NewPCG(1, 0))
	_, err := BuildTheme(fsys, r)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "03_workspace_type.txt") {
		t.Fatalf("expected file name in error: %v", err)
	}
}

func TestWriteFinal_TC_C2(t *testing.T) {
	t.Parallel()
	fsys := testdataDir(t, "build_single")
	tmpl, err := ReadTemplate(fsys)
	if err != nil {
		t.Fatal(err)
	}
	style, err := ReadLegacyStyle(fsys)
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err = ApplyStyle(tmpl, style)
	if err != nil {
		t.Fatal(err)
	}
	th, err := BuildTheme(fsys, rand.New(rand.NewPCG(1, 0)))
	if err != nil {
		t.Fatal(err)
	}
	body := RenderFinal(tmpl, th.FormatThemeBlock())
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gotBytes, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := fs.ReadFile(fsys, "expected_final.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(string(wantBytes), "\r\n", "\n")
	got := strings.ReplaceAll(string(gotBytes), "\r\n", "\n")
	if got != want {
		t.Fatalf("written file mismatch")
	}
}

func TestReadTemplate_TC_D2_empty(t *testing.T) {
	t.Parallel()
	fsys := testdataDir(t, "err_empty_template")
	_, err := ReadTemplate(fsys)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "空") {
		t.Fatalf("expected empty-template hint: %v", err)
	}
}

func TestReadTemplate_RequiresExactlyOneStylePlaceholder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
	}{
		{name: "missing", template: "COMMON_ONLY\n"},
		{name: "multiple", template: "{{STYLE}}\n{{STYLE}}\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fsys := fstest.MapFS{
				templateFile:    {Data: []byte(tt.template)},
				legacyStyleFile: {Data: []byte("STYLE\n")},
			}
			_, err := ReadTemplate(fsys)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), stylePlaceholder) {
				t.Fatalf("expected placeholder hint: %v", err)
			}
		})
	}
}

func TestReadLegacyStyle_RejectsEmpty(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		legacyStyleFile: {Data: []byte("\n")},
	}
	_, err := ReadLegacyStyle(fsys)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), legacyStyleFile) || !strings.Contains(err.Error(), "空") {
		t.Fatalf("expected empty-style hint: %v", err)
	}
}

func TestApplyStyle_CustomStyle(t *testing.T) {
	t.Parallel()

	got, err := ApplyStyle("BEFORE\r\n{{STYLE}}\r\nAFTER\r\n", "CUSTOM\r\nSTYLE\r\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "BEFORE\nCUSTOM\nSTYLE\nAFTER\n"
	if got != want {
		t.Fatalf("styled template mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestApplyStyle_RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		style    string
	}{
		{name: "missing placeholder", template: "COMMON_ONLY\n", style: "STYLE\n"},
		{name: "multiple placeholders", template: "{{STYLE}}\n{{STYLE}}\n", style: "STYLE\n"},
		{name: "empty style", template: "BEFORE\n{{STYLE}}\nAFTER\n", style: "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ApplyStyle(tt.template, tt.style); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
