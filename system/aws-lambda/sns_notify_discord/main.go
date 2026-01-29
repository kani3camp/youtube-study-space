package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/workspaceapp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	lambdautils.InitLogger()
}

func handler(ctx context.Context, evt events.SNSEvent) error {
	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		slog.Error("failed to get Firestore client option", "err", err)
		return err
	}

	app, err := workspaceapp.NewWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// NOTE: このLambdaは通知Lambda自体なので、タイムアウト時はログに出力するのみ（自分自身への通知は循環になる）
			slog.Error("timeout warning in sns_notify_discord during initialization", "err", err)
			return nil
		}
		slog.Error("failed to init WorkspaceApp", "err", err)
		return err
	}
	defer app.CloseFirestoreClient()

	if len(evt.Records) == 0 {
		slog.Warn("SNS event has no records")
		return nil
	}

	for i, record := range evt.Records {
		rec := record.SNS
		subject := rec.Subject
		message := rec.Message

		// Try to compact JSON messages
		var tmp map[string]any
		if json.Unmarshal([]byte(message), &tmp) == nil {
			if b, e := json.Marshal(tmp); e == nil {
				message = string(b)
			}
		}

		// Log full message before truncation for console inspection
		slog.InfoContext(gracefulCtx, "sns notify full message", "record_index", i, "subject", subject, "message_full", message)

		if len(message) > 1800 {
			message = message[:1800] + "... (truncated)"
		}

		notify := fmt.Sprintf("[SNS] %s\n%s", subject, message)
		app.MessageToOwner(gracefulCtx, notify)
	}

	// 処理完了後にコンテキストがキャンセルされていたらログ出力
	if errors.Is(gracefulCtx.Err(), context.DeadlineExceeded) {
		slog.Error("timeout warning in sns_notify_discord after processing", "processed_records", len(evt.Records))
	}

	return nil
}

func main() {
	lambda.Start(handler)
}
