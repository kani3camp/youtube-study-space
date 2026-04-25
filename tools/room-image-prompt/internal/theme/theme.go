package theme

import (
	"bytes"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"strings"
)

const templateFile = "prompt_template.txt"

// Fixed candidate filenames in step order (do not rely on directory listing order).
var candidateFiles = []string{
	"01_main_category.txt",
	"02_scene.txt",
	"03_space_type.txt",
	"04_mood.txt",
	"05_seat_layout.txt",
}

// Theme holds one chosen line per step.
type Theme struct {
	MainCategory string
	Scene        string
	SpaceType    string
	Mood         string
	SeatLayout   string
}

// FormatThemeBlock returns exactly 5 lines: "key: value\n" each, POSIX trailing newline.
func (t Theme) FormatThemeBlock() string {
	var b strings.Builder
	lines := []struct {
		key, val string
	}{
		{"main_category", t.MainCategory},
		{"scene", t.Scene},
		{"space_type", t.SpaceType},
		{"mood", t.Mood},
		{"seat_layout", t.SeatLayout},
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
	vals := []*string{&t.MainCategory, &t.Scene, &t.SpaceType, &t.Mood, &t.SeatLayout}
	for i, name := range candidateFiles {
		lines, err := LoadLines(fsys, name)
		if err != nil {
			return Theme{}, err
		}
		pick := lines[r.IntN(len(lines))]
		*vals[i] = pick
	}
	return t, nil
}

// ReadTemplate loads and normalizes prompt_template.txt (CRLF -> LF). Returns error if empty after trim.
func ReadTemplate(fsys fs.FS) (string, error) {
	b, err := fs.ReadFile(fsys, templateFile)
	if err != nil {
		return "", fmt.Errorf("read %q: %w", templateFile, err)
	}
	s := string(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%q: テンプレートが空です", templateFile)
	}
	return s, nil
}
