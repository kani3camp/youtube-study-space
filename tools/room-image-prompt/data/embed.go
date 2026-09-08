package data

import "embed"

//go:generate go run ../cmd/generate-styles

//go:embed *.txt
var FS embed.FS
