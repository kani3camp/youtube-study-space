package stylegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	referenceDirRelative = ".agents/skills/room-art-direction/references"
	outputDirRelative    = "tools/room-image-prompt/data"
	promptGuidanceHeader = "## Prompt guidance"
	generatedFilePrefix  = "style_direction_"
	generatedFileSuffix  = ".generated.txt"
)

// Artifact is one generated CLI style asset derived from a canonical Direction Markdown file.
type Artifact struct {
	StyleName  string
	SourcePath string
	OutputPath string
	Content    string
}

// FindRepoRoot walks upward from start until both the canonical Direction directory and
// room-image-prompt directory exist.
func FindRepoRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start path: %w", err)
	}

	for dir := abs; ; dir = filepath.Dir(dir) {
		if isDir(filepath.Join(dir, referenceDirRelative)) && isDir(filepath.Join(dir, outputDirRelative)) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("repository root not found from %q", start)
}

// Generate reads every canonical direction-*.md reference and derives the CLI style asset
// from its Prompt guidance section. Direction Markdown remains the single source of truth.
func Generate(repoRoot string) ([]Artifact, error) {
	referenceDir := filepath.Join(repoRoot, referenceDirRelative)
	entries, err := os.ReadDir(referenceDir)
	if err != nil {
		return nil, fmt.Errorf("read direction references: %w", err)
	}

	var artifacts []Artifact
	seenStyles := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		styleName, ok := styleNameFromReferenceFile(entry.Name())
		if !ok {
			continue
		}
		if previous, exists := seenStyles[styleName]; exists {
			return nil, fmt.Errorf("duplicate style %q from %q and %q", styleName, previous, entry.Name())
		}
		seenStyles[styleName] = entry.Name()

		sourcePath := filepath.Join(referenceDirRelative, entry.Name())
		b, err := os.ReadFile(filepath.Join(repoRoot, sourcePath))
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", sourcePath, err)
		}
		content, err := ExtractPromptGuidance(string(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", sourcePath, err)
		}

		outputName := "style_" + strings.ReplaceAll(styleName, "-", "_") + generatedFileSuffix
		artifacts = append(artifacts, Artifact{
			StyleName:  styleName,
			SourcePath: sourcePath,
			OutputPath: filepath.Join(outputDirRelative, outputName),
			Content:    content,
		})
	}

	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no direction references found in %q", referenceDirRelative)
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].StyleName < artifacts[j].StyleName })
	return artifacts, nil
}

// ExtractPromptGuidance returns the exact Markdown body under one "## Prompt guidance" heading,
// normalized to LF and terminated by one newline.
func ExtractPromptGuidance(markdown string) (string, error) {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(markdown, "\n")

	start := -1
	found := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == promptGuidanceHeader {
			found++
			start = i + 1
		}
	}
	if found != 1 {
		return "", fmt.Errorf("%q section must appear exactly once (actual: %d)", promptGuidanceHeader, found)
	}

	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	body := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if body == "" {
		return "", fmt.Errorf("%q section is empty", promptGuidanceHeader)
	}
	return body + "\n", nil
}

// Write updates generated style assets and removes stale generated Direction assets.
func Write(repoRoot string, artifacts []Artifact) error {
	wanted := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		wanted[filepath.Base(artifact.OutputPath)] = struct{}{}
	}

	outputDir := filepath.Join(repoRoot, outputDirRelative)
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return fmt.Errorf("read output dir: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isGeneratedDirectionFile(name) {
			continue
		}
		if _, ok := wanted[name]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(outputDir, name)); err != nil {
			return fmt.Errorf("remove stale generated style %q: %w", name, err)
		}
	}

	for _, artifact := range artifacts {
		path := filepath.Join(repoRoot, artifact.OutputPath)
		if err := os.WriteFile(path, []byte(artifact.Content), 0o644); err != nil {
			return fmt.Errorf("write %q: %w", artifact.OutputPath, err)
		}
	}
	return nil
}

// Check verifies that committed generated style assets exactly match canonical Markdown.
func Check(repoRoot string, artifacts []Artifact) error {
	wanted := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		wanted[filepath.Base(artifact.OutputPath)] = struct{}{}
		b, err := os.ReadFile(filepath.Join(repoRoot, artifact.OutputPath))
		if err != nil {
			return fmt.Errorf("generated style %q: %w", artifact.OutputPath, err)
		}
		if string(b) != artifact.Content {
			return fmt.Errorf("generated style %q is stale; run go generate ./data", artifact.OutputPath)
		}
	}

	entries, err := os.ReadDir(filepath.Join(repoRoot, outputDirRelative))
	if err != nil {
		return fmt.Errorf("read output dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !isGeneratedDirectionFile(entry.Name()) {
			continue
		}
		if _, ok := wanted[entry.Name()]; !ok {
			return fmt.Errorf("stale generated style %q; run go generate ./data", filepath.Join(outputDirRelative, entry.Name()))
		}
	}
	return nil
}

func styleNameFromReferenceFile(name string) (string, bool) {
	if !strings.HasPrefix(name, "direction-") || !strings.HasSuffix(name, ".md") {
		return "", false
	}
	base := strings.TrimSuffix(name, ".md")
	parts := strings.Split(base, "-")
	if len(parts) < 3 || parts[0] != "direction" || !validStyleID(parts[1]) {
		return "", false
	}
	return "direction-" + parts[1], true
}

func validStyleID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func isGeneratedDirectionFile(name string) bool {
	return strings.HasPrefix(name, generatedFilePrefix) && strings.HasSuffix(name, generatedFileSuffix)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
