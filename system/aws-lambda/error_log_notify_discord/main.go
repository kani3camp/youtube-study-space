package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"app.modules/aws-lambda/lambdautils"
	coreutils "app.modules/core/utils"
	"app.modules/core/workspaceapp"
	"app.modules/internal/logging"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"google.golang.org/api/option"
)

func init() {
	logging.InitLogger()
}

const (
	maxDiscordMessageLength = 1800
	notifyPrefix            = "[ERROR_LOG] "
)

type errorLogNotifyApp interface {
	MessageToOwnerOrError(ctx context.Context, message string) error
	CloseFirestoreClient()
}

var (
	// Unit test で初期化失敗や通知失敗を差し替え検証できるようにしている。
	firestoreClientOptionErrorLog = lambdautils.FirestoreClientOption
	newErrorLogWorkspaceApp       = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (errorLogNotifyApp, error) {
		return workspaceapp.NewWorkspaceApp(ctx, isTest, clientOption)
	}
)

func handler(ctx context.Context, ev events.CloudwatchLogsEvent) error {
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	// NOTE: 定期 3 Lambda と違い、この Lambda は通知経路そのものの故障を
	// Errors Alarm + Email バックストップで拾いたいため、初期化や parse 失敗は return err を維持する。
	clientOption, err := firestoreClientOptionErrorLog()
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Firestore client option for error_log_notify_discord", "err", err)
		return err
	}

	app, err := newErrorLogWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "failed to init WorkspaceApp for error_log_notify_discord", "err", err)
		return err
	}
	defer app.CloseFirestoreClient()

	data, err := ev.AWSLogs.Parse()
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse CloudWatch Logs payload", "err", err)
		return fmt.Errorf("parse CloudWatch Logs: %w", err)
	}

	chunks := buildDiscordMessageChunks(&data, invokerRequestIDFromContext(ctx))
	if len(chunks) == 0 {
		slog.WarnContext(ctx, "CloudWatch Logs event had no log events")
		return nil
	}

	for _, chunk := range chunks {
		if err := app.MessageToOwnerOrError(gracefulCtx, chunk); err != nil {
			slog.ErrorContext(ctx, "failed to send log notification to owner", "err", err)
			return fmt.Errorf("send log notification to owner: %w", err)
		}
	}

	return nil
}

func invokerRequestIDFromContext(ctx context.Context) string {
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		return lc.AwsRequestID
	}
	return ""
}

func buildDiscordMessageChunks(data *events.CloudwatchLogsData, invokerRequestID string) []string {
	if len(data.LogEvents) == 0 {
		return nil
	}

	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%slogGroup=%s\nlogStream=%s\ninvoker_request_id=%s\n",
		notifyPrefix, data.LogGroup, data.LogStream, invokerRequestID)

	for _, le := range data.LogEvents {
		_, _ = fmt.Fprintf(&b, "--- id=%s ts=%d ---\n%s\n", le.ID, le.Timestamp, le.Message)
	}

	return splitToDiscordSizedChunks(b.String(), maxDiscordMessageLength)
}

// splitToDiscordSizedChunks splits s into UTF-8 safe chunks no longer than limit runes.
func splitToDiscordSizedChunks(s string, limit int) []string {
	return coreutils.SplitStringRunes(s, limit)
}

func main() {
	lambda.Start(handler)
}
