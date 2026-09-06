package look

import (
	"math/rand/v2"
	"strings"
	"testing"
	"testing/fstest"
)

func TestResolveNamed(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"look_airy_garden.txt": {Data: []byte("AIRY\n")},
	}
	got, err := Resolve(fsys, "airy-garden", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "airy-garden" || got.Text != "AIRY\n" {
		t.Fatalf("unexpected selection: %+v", got)
	}
}

func TestResolveNone(t *testing.T) {
	t.Parallel()

	got, err := Resolve(fstest.MapFS{}, NoneName, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != NoneName || got.Text != "" {
		t.Fatalf("unexpected selection: %+v", got)
	}
}

func TestResolveAutoIsDeterministicAndBundled(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"look_indigo_violet_fantasy.txt": {Data: []byte("INDIGO\n")},
		"look_airy_garden.txt":            {Data: []byte("AIRY\n")},
		"look_coral_aqua_glow.txt":        {Data: []byte("CORAL\n")},
		"look_crystal_lucent.txt":         {Data: []byte("CRYSTAL\n")},
	}
	a, err := Resolve(fsys, AutoName, rand.New(rand.NewPCG(42, 0)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(fsys, AutoName, rand.New(rand.NewPCG(42, 0)))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("auto selection should be deterministic: a=%+v b=%+v", a, b)
	}
	if a.Name == AutoName || a.Name == NoneName || strings.TrimSpace(a.Text) == "" {
		t.Fatalf("auto selection should resolve to bundled content: %+v", a)
	}
}

func TestResolveRejectsInvalidOrEmpty(t *testing.T) {
	t.Parallel()

	if _, err := Resolve(fstest.MapFS{}, "other", nil); err == nil {
		t.Fatal("expected invalid-look error")
	}

	fsys := fstest.MapFS{
		"look_airy_garden.txt": {Data: []byte("\n")},
	}
	if _, err := Resolve(fsys, "airy-garden", nil); err == nil {
		t.Fatal("expected empty-look error")
	}
}
