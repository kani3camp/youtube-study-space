package main

import (
	"context"
	"encoding/json"
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

func handler(evt events.SNSEvent) error {
	ctx := context.Background()

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		slog.Error("failed to get Firestore client option", "err", err)
		return err
	}
	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
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
		slog.InfoContext(ctx, "sns notify full message", "record_index", i, "subject", subject, "message_full", message)

		if len(message) > 1800 {
			message = message[:1800] + "... (truncated)"
		}

		notify := fmt.Sprintf("[SNS] %s\n%s", subject, message)
		app.MessageToOwner(ctx, notify)
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
