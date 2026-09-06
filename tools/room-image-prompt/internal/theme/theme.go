package theme

import (
	"fmt"
	"io/fs"
	"math/rand/v2"
	"strconv"
	"strings"
)

const (
	templateFile     = "prompt_template.txt"
	legacyStyleFile  = "style_legacy.txt"
	stylePlaceholder = "{{STYLE}}"
)

// Seat count is chosen uniformly in [seatCountMin, seatCountMax] (inclusive) without a data file.
const (
	seatCountMin  = 10
	seatCountMax  = 15
	seatCountSpan = seatCountMax - seatCountMin + 1
)

// Fixed candidate filenames in step order (do not rely on directory listing order).
var candidateFiles = []string{
	"01_world.txt",
	"02_time_of_day.txt",
	"03_workspace_type.txt",
	"04_seat_layout.txt",
}

// Theme holds one chosen line per data-file step, plus a random seat count in [10, 15].
type Theme struct {
	World         string
	TimeOfDay     string
	WorkspaceType string
	SeatLayout    string
	SeatCount     int
}

// FormatThemeBlock returns exactly 5 lines: "key: value\n" each, POSIX trailing newline.
// Keys are fixed Japanese labels for LLM/prompt consumers.
func (t Theme) FormatThemeBlock() string {
	var b strings.Builder
	lines := []struct {
		key, val string
	}{
		{"世界観", t.World},
		{"時間帯", t.TimeOfDay},
		{"作業空間", t.WorkspaceType},
		{"座席レイアウト", t.SeatLayout},
		{"座席数", strconv.Itoa(t.SeatCount)},
	}
	for _, row := range lines {
		b.WriteString(row.key)
		b.WriteString(": ")
		b.WriteString(row.val)
		b.WriteByte('\n')
	}
	return b.String()
}

// BuildTheme loads candidates from fsys, picks one line per step using r, and returns Theme.
func BuildTheme(fsys fs.FS, r *rand.Rand) (Theme, error) {
	var t Theme
	vals := []*string{&t.World, &t.TimeOfDay, &t.WorkspaceType, &t.SeatLayout}
	for i, name := range candidateFiles {
		lines, err := LoadLines(fsys, name)
		if err != nil {
			return Theme{}, err
		}
		pick := lines[r.IntN(len(lines))]
		*vals[i] = pick
	}
	t.SeatCount = seatCountMin + r.IntN(seatCountSpan)
	return t, nil
}

// ReadTemplate loads and validates the common prompt template without choosing a style.
func ReadTemplate(fsys fs.FS) (string, error) {
	tmpl, err := readRequiredText(fsys, templateFile)
	if err != nil {
		return "", err
	}
	if err := validateStylePlaceholder(tmpl); err != nil {
		return "", fmt.Errorf("%q: %w", templateFile, err)
	}
	return tmpl, nil
}

// ReadLegacyStyle loads the bundled legacy style used when no explicit style is supplied.
func ReadLegacyStyle(fsys fs.FS) (string, error) {
	return readRequiredText(fsys, legacyStyleFile)
}

// ApplyStyle injects style into exactly one {{STYLE}} placeholder.
// It accepts arbitrary style text so callers do not need to encode named art directions here.
func ApplyStyle(template, style string) (string, error) {
	template = normalizeNewlines(template)
	if err := validateStylePlaceholder(template); err != nil {
		return "", err
	}

	style = normalizeNewlines(style)
	if strings.TrimSpace(style) == "" {
		return "", fmt.Errorf("style: テキストが空です")
	}
	style = strings.TrimRight(style, "\n")

	return strings.Replace(template, stylePlaceholder, style, 1), nil
}

func validateStylePlaceholder(template string) error {
	count := strings.Count(template, stylePlaceholder)
	if count != 1 {
		return fmt.Errorf("%s は1個必要です（実際: %d個）", stylePlaceholder, count)
	}
	return nil
}

func readRequiredText(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", name, err)
	}
	s := normalizeNewlines(string(b))
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%q: テキストが空です", name)
	}
	return s, nil
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
