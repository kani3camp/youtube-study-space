package mypage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicFixturesMatchWireTypes(t *testing.T) {
	paths, err := filepath.Glob("../../../docs/mypage/fixtures/*.json")
	if err != nil || len(paths) < 2 {
		t.Fatal("synthetic contract fixtures are required")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var response Response
			decoder := json.NewDecoder(bytes.NewReader(input))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&response); err != nil {
				t.Fatal(err)
			}
			output, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			var before, after any
			if err := json.Unmarshal(input, &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(output, &after); err != nil {
				t.Fatal(err)
			}
			// Round-trip equality detects missing fields and wrong nullable shapes.
			b, err := json.Marshal(before)
			if err != nil {
				t.Fatal(err)
			}
			a, err := json.Marshal(after)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a, b) {
				t.Fatal("wire types changed the public fixture")
			}
		})
	}
}
