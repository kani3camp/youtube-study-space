package lambdautils

import (
	"log/slog"
	"os"
)

// InitLogger は slog のデフォルトロガーをJSON形式に設定します。
// Lambda関数の init() から呼び出してください。
func InitLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	})
	slog.SetDefault(slog.New(handler))
}

