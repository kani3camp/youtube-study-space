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

	// graceSeconds が負の場合は 0 に丸める
	if graceSeconds < 0 {
		graceSeconds = 0
	}

	now := time.Now()
	gracefulDeadline := deadline.Add(-time.Duration(graceSeconds) * time.Second)

	// グレースフルデッドラインが現在時刻より前になる場合は、
	// 即時キャンセルを避けるため元のデッドラインを使用する
	if gracefulDeadline.Before(now) {
		return context.WithDeadline(ctx, deadline)
	}

	return context.WithDeadline(ctx, gracefulDeadline)
}
