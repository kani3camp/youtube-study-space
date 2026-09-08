package look

import (
	"fmt"
	"io/fs"
	"math/rand/v2"
	"strings"
)

const (
	AutoName = "auto"
	NoneName = "none"
)

var bundledNames = []string{
	"indigo-violet-fantasy",
	"airy-garden",
	"coral-aqua-glow",
	"crystal-lucent",
}

type Selection struct {
	Name string
	Text string
}

func BundledNames() []string {
	return append([]string(nil), bundledNames...)
}

func Resolve(fsys fs.FS, name string, rng *rand.Rand) (Selection, error) {
	switch name {
	case "", AutoName:
		if rng == nil {
			return Selection{}, fmt.Errorf("look %q requires RNG", AutoName)
		}
		name = bundledNames[rng.IntN(len(bundledNames))]
	case NoneName:
		return Selection{Name: NoneName}, nil
	}

	filename, err := filenameFor(name)
	if err != nil {
		return Selection{}, err
	}
	body, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return Selection{}, fmt.Errorf("read look %q: %w", name, err)
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	if strings.TrimSpace(text) == "" {
		return Selection{}, fmt.Errorf("look %q: text is empty", name)
	}
	return Selection{Name: name, Text: text}, nil
}

func filenameFor(name string) (string, error) {
	for _, bundled := range bundledNames {
		if name == bundled {
			return "look_" + strings.ReplaceAll(name, "-", "_") + ".txt", nil
		}
	}
	return "", fmt.Errorf("unknown look %q", name)
}
