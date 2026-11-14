package workspaceapp

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type privateOwnerBotFake struct {
	messages []string
	err      error
}

func (f *privateOwnerBotFake) SendMessage(_ context.Context, message string) error {
	f.messages = append(f.messages, message)
	return f.err
}

func (f *privateOwnerBotFake) SendMessageWithError(_ context.Context, message string, _ error) error {
	f.messages = append(f.messages, message)
	return f.err
}

func TestOwnerDeliveryFailureDoesNotLogNotificationOrProviderError(t *testing.T) {
	const privateValue = "PRIVATE_NOTIFICATION_BODY_712"
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	bot := &privateOwnerBotFake{err: errors.New("provider error: " + privateValue)}
	app := WorkspaceApp{alertOwnerBot: bot}
	app.MessageToOwner(context.Background(), privateValue)
	app.MessageToOwnerWithError(context.Background(), privateValue, errors.New(privateValue))

	if len(bot.messages) != 2 || bot.messages[0] != privateValue || bot.messages[1] != privateValue {
		t.Fatalf("owner notifications changed: %q", bot.messages)
	}
	if strings.Count(logs.String(), `"error_class":"delivery_failed"`) != 2 {
		t.Fatalf("missing safe failure class for both sends: %s", logs.String())
	}
	if strings.Contains(logs.String(), privateValue) {
		t.Fatalf("logs exposed notification/provider payload: %s", logs.String())
	}
}

func TestSeatExitResultDoesNotLogTransactionPayload(t *testing.T) {
	const privateValue = "PRIVATE_DISPLAY_NAME_711"
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "completed", want: "seat exit completed"},
		{name: "failed", err: errors.New("transaction for " + privateValue + " failed"), want: `"error_class":"transaction_failed"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })

			logSeatExitResult(tc.err)
			if !strings.Contains(logs.String(), tc.want) {
				t.Fatalf("missing safe event/status: %s", logs.String())
			}
			if strings.Contains(logs.String(), privateValue) {
				t.Fatalf("logs exposed display name: %s", logs.String())
			}
		})
	}
}
