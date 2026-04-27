package main

import (
	"bytes"
	"context"
	"encoding/json"
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

	eventChunks := make([]string, 0, len(data.LogEvents))
	allEventsAreJSON := true
	for _, le := range data.LogEvents {
		message, ok := formatLogEventMessage(le.Message)
		if !ok {
			allEventsAreJSON = false
		}
		eventChunks = append(eventChunks, message+"\n")
	}
	bodyLanguage := "json"
	if !allEventsAreJSON {
		bodyLanguage = "text"
	}

	headerWithoutChunk := fmt.Sprintf("%slogGroup=%s\ninvoker_request_id=%s\n",
		notifyPrefix, data.LogGroup, invokerRequestID)
	bodyLimit := discordBodyLimit(headerWithoutChunk, bodyLanguage, 1)

	var bodyChunks []string
	for {
		bodyChunks = buildDiscordBodyChunks(eventChunks, bodyLimit)
		nextBodyLimit := discordBodyLimit(headerWithoutChunk, bodyLanguage, len(bodyChunks))
		if nextBodyLimit == bodyLimit {
			break
		}
		bodyLimit = nextBodyLimit
	}
	messageChunks := make([]string, 0, len(bodyChunks))
	total := len(bodyChunks)
	for i, body := range bodyChunks {
		header := fmt.Sprintf("%schunk=%d/%d\n", headerWithoutChunk, i+1, total)
		messageChunks = append(messageChunks, header+wrapDiscordCodeBlock(bodyLanguage, body))
	}

	return messageChunks
}

func formatLogEventMessage(message string) (string, bool) {
	var b bytes.Buffer
	if err := json.Indent(&b, []byte(message), "", "  "); err != nil {
		return message, false
	}
	return b.String(), true
}

func wrapDiscordCodeBlock(language string, body string) string {
	return fmt.Sprintf("```%s\n%s```", language, body)
}

func discordBodyLimit(headerWithoutChunk string, bodyLanguage string, totalChunks int) int {
	if totalChunks < 1 {
		totalChunks = 1
	}
	chunkHeader := fmt.Sprintf("chunk=%d/%d\n", totalChunks, totalChunks)
	codeBlockOverhead := len([]rune(fmt.Sprintf("```%s\n", bodyLanguage))) + len([]rune("```"))
	bodyLimit := maxDiscordMessageLength - len([]rune(headerWithoutChunk)) - len([]rune(chunkHeader)) - codeBlockOverhead
	if bodyLimit <= 0 {
		return 1
	}
	return bodyLimit
}

func buildDiscordBodyChunks(parts []string, limit int) []string {
	if len(parts) == 0 {
		return nil
	}

	chunks := make([]string, 0, len(parts))
	var current strings.Builder

	flushCurrent := func() {
		if current.Len() == 0 {
			return
		}
		chunks = append(chunks, current.String())
		current.Reset()
	}

	for _, part := range parts {
		for _, piece := range splitToDiscordSizedChunks(part, limit) {
			if current.Len() == 0 {
				current.WriteString(piece)
				continue
			}

			if len([]rune(current.String()))+len([]rune(piece)) > limit {
				flushCurrent()
			}
			current.WriteString(piece)
		}
	}

	flushCurrent()
	return chunks
}

// splitToDiscordSizedChunks splits s into UTF-8 safe chunks no longer than limit runes.
func splitToDiscordSizedChunks(s string, limit int) []string {
	return coreutils.SplitStringRunes(s, limit)
}

func main() {
	lambda.Start(handler)
}
