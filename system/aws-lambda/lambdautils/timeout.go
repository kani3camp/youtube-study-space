package lambdautils

import (
	"context"
	"time"
)

const (
	// DefaultGraceSeconds はLambdaタイムアウトの何秒前にグレースフル終了処理（Discord通知など）を開始するかのデフォルト値
	DefaultGraceSeconds = 5
)

// CreateGracefulContext はLambdaタイムアウトの graceSeconds 秒前に
// キャンセルされる派生コンテキストを作成する
func CreateGracefulContext(ctx context.Context, graceSeconds int) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		// デッドラインがない場合はキャンセル可能なコンテキストを返す
		return context.WithCancel(ctx)
	}
	gracefulDeadline := deadline.Add(-time.Duration(graceSeconds) * time.Second)
	return context.WithDeadline(ctx, gracefulDeadline)
}
