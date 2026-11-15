package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"unicode/utf8"

	coreutils "app.modules/core/utils"
	"app.modules/core/workspaceapp"
	"app.modules/internal/awsruntime"
	"app.modules/internal/logging"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"google.golang.org/api/option"
)

func init() {
	logging.InitLogger()
}

const (
	maxDiscordMessageLength = 1800
	truncatedSuffix         = "... (truncated)"
	notifyPrefix            = "[SNS] "
)

type snsNotifyApp interface {
	MessageToOwnerOrError(ctx context.Context, message string) error
	CloseFirestoreClient()
}

var (
	firestoreClientOptionSNS = awsruntime.FirestoreClientOption
	newSNSWorkspaceApp       = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (snsNotifyApp, error) {
		return workspaceapp.NewWorkspaceApp(ctx, isTest, clientOption)
	}
)

func handler(ctx context.Context, evt events.SNSEvent) error {
	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := awsruntime.CreateGracefulContext(ctx, awsruntime.DefaultGraceSeconds)
	defer cancel()

	clientOption, err := firestoreClientOptionSNS()
	if err != nil {
		slog.Error("failed to get Firestore client option", logging.ErrorClassAttrs("firestore_option_failed", err)...)
		return &logging.ClassifiedError{Message: "load Firestore client option failed", Cause: err}
	}

	app, err := newSNSWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// NOTE: このLambdaは通知Lambda自体なので、タイムアウト時はログに出力するのみ（自分自身への通知は循環になる）
			slog.Error("timeout warning in sns_notify_discord during initialization", logging.ErrorClassAttrs("deadline_exceeded", err)...)
			return nil
		}
		slog.Error("failed to init WorkspaceApp", logging.ErrorClassAttrs("workspace_init_failed", err)...)
		return &logging.ClassifiedError{Message: "initialize workspace app failed", Cause: err}
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

		slog.InfoContext(gracefulCtx, "sns notification dispatch", "record_index", i, "status", "started")

		notify := buildDiscordNotification(subject, message)
		if err := app.MessageToOwnerOrError(gracefulCtx, notify); err != nil {
			errorClass := "delivery_failed"
			if errors.Is(err, context.DeadlineExceeded) {
				errorClass = "deadline_exceeded"
			} else if errors.Is(err, context.Canceled) {
				errorClass = "canceled"
			}
			slog.ErrorContext(gracefulCtx, "failed to send SNS notification to owner", append([]any{"record_index", i}, logging.ErrorClassAttrs(errorClass, err)...)...)
			return &logging.ClassifiedError{Message: "send SNS notification to owner: " + errorClass, Cause: err}
		}
		slog.InfoContext(gracefulCtx, "sns notification dispatch", "record_index", i, "status", "sent")
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

func buildDiscordNotification(subject string, message string) string {
	notify := fmt.Sprintf("%s%s\n%s", notifyPrefix, subject, message)
	if utf8.RuneCountInString(notify) <= maxDiscordMessageLength {
		return notify
	}

	prefixRunes := utf8.RuneCountInString(notifyPrefix)
	subjectRunes := utf8.RuneCountInString(subject)
	suffixRunes := utf8.RuneCountInString(truncatedSuffix)
	availableMessageLength := maxDiscordMessageLength - prefixRunes - subjectRunes - 1
	if availableMessageLength <= suffixRunes {
		return coreutils.TruncateStringRunes(notify, maxDiscordMessageLength)
	}

	truncatedMessage := coreutils.TruncateStringRunes(message, availableMessageLength-suffixRunes) + truncatedSuffix
	return fmt.Sprintf("%s%s\n%s", notifyPrefix, subject, truncatedMessage)
}
