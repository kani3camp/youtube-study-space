package theme

import (
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if th.SeatCount != 17 {
		t.Fatalf("unexpected SeatCount: %d (want 17 for PCG(1,0))", th.SeatCount)
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
