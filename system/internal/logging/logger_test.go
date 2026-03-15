package logging

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewJSONLoggerOutputsJSON(t *testing.T) {
	var buf bytes.Buffer

	logger := NewJSONLogger(&buf)
	logger.Info("test message", "component", "batch")

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v, output=%s", err, buf.Bytes())
	}

	if got := entry["level"]; got != "INFO" {
		t.Fatalf("unexpected level: got=%v", got)
	}
	if got := entry["msg"]; got != "test message" {
		t.Fatalf("unexpected msg: got=%v", got)
	}
	if got := entry["component"]; got != "batch" {
		t.Fatalf("unexpected component: got=%v", got)
	}

	source, ok := entry["source"].(map[string]any)
	if !ok {
		t.Fatalf("source field is missing or invalid: %#v", entry["source"])
	}
	if file, ok := source["file"].(string); !ok || file == "" {
		t.Fatalf("source.file is missing or empty: %#v", source["file"])
	}
	if _, ok := source["line"].(float64); !ok {
		t.Fatalf("source.line is missing or invalid: %#v", source["line"])
	}
}
