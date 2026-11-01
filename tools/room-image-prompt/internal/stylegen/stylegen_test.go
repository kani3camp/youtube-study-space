package stylegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPromptGuidance(t *testing.T) {
	t.Parallel()

	markdown := "# Direction A\r\n\r\n## Prompt guidance\r\n\r\n- one\r\n- two\r\n\r\n## Review checklist\r\n- [ ] check\r\n"
	got, err := ExtractPromptGuidance(markdown)
	if err != nil {
		t.Fatal(err)
	}
	want := "- one\n- two\n"
	if got != want {
		t.Fatalf("fragment mismatch: got %q want %q", got, want)
	}
}

func TestExtractPromptGuidance_RequiresExactlyOneSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
	}{
		{name: "missing", markdown: "# Direction A\n"},
		{name: "duplicate", markdown: "## Prompt guidance\n- one\n## Prompt guidance\n- two\n"},
		{name: "empty", markdown: "## Prompt guidance\n\n## Review checklist\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ExtractPromptGuidance(tt.markdown); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestGeneratedStylesUpToDate(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot, err := FindRepoRoot(wd)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := Generate(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(repoRoot, artifacts); err != nil {
		t.Fatal(err)
	}

	styles := make(map[string]bool, len(artifacts))
	for _, artifact := range artifacts {
		styles[artifact.StyleName] = true
	}
	for _, required := range []string{"direction-a", "direction-b", "direction-c", "direction-d"} {
		if !styles[required] {
			t.Fatalf("required generated style is missing: %q", required)
		}
	}

	for _, artifact := range artifacts {
		if filepath.Ext(artifact.OutputPath) != ".txt" || !strings.Contains(filepath.Base(artifact.OutputPath), ".generated.") {
			t.Fatalf("unexpected generated output path: %q", artifact.OutputPath)
		}
	}
}
