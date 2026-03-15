package logging

import (
	"io"
	"log/slog"
	"os"
)

// NewJSONLogger returns a slog logger configured for one-line JSON output.
func NewJSONLogger(w io.Writer) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		AddSource: true,
	})
	return slog.New(handler)
}

// InitLogger sets the default slog logger to JSON output for Lambda and ECS entrypoints.
func InitLogger() {
	slog.SetDefault(NewJSONLogger(os.Stdout))
}
