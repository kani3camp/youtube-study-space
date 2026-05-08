package theme

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadLines_TC_A1(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"a.txt": {Data: []byte("a\n\n  b  \n")},
	}
	got, err := LoadLines(fsys, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestLoadLines_TC_A2(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"a.txt": {Data: []byte("# comment\nd\n")},
	}
	got, err := LoadLines(fsys, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"d"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestLoadLines_TC_A3(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"a.txt": {Data: []byte("#only1\n#only2\n\n")},
	}
	_, err := LoadLines(fsys, "a.txt")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "0件") {
		t.Fatalf("error should mention 0件: %v", err)
	}
}

func TestLoadLines_CRLF(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"a.txt": {Data: []byte("a\r\nb\r\n")},
	}
	got, err := LoadLines(fsys, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %v", got)
	}
}
