package theme

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"strings"
)

// LoadLines reads path from fsys, normalizes CRLF to LF, and returns non-empty
// candidate lines (trimmed). Lines where strings.TrimSpace starts with '#' are skipped.
func LoadLines(fsys fs.FS, path string) ([]string, error) {
	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	s := string(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %q: %w", path, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%q: 候補が0件です", path)
	}
	return out, nil
}
