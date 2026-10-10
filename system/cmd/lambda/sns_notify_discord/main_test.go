package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"
	"google.golang.org/api/option"
)

type fakeSNSNotifyApp struct {
	messages []string
	sendErr  error
	closed   bool
}

func (f *fakeSNSNotifyApp) MessageToOwnerOrError(_ context.Context, message string) error {
	f.messages = append(f.messages, message)
	return f.sendErr
}

func (f *fakeSNSNotifyApp) CloseFirestoreClient() { f.closed = true }

func TestHandlerDoesNotLogSNSPayload(t *testing.T) {
	const subject = "PRIVATE_SNS_SUBJECT_129"
	const message = "PRIVATE_SNS_BODY_830"
	for _, tc := range []struct {
		name      string
		sendErr   error
		wantClass string
	}{
		{name: "sent"},
		{name: "delivery failed", sendErr: errors.New("delivery failure: " + message), wantClass: "delivery_failed"},
		{name: "deadline", sendErr: context.DeadlineExceeded, wantClass: "deadline_exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })

			app := &fakeSNSNotifyApp{sendErr: tc.sendErr}
			previousOption, previousApp := firestoreClientOptionSNS, newSNSWorkspaceApp
			firestoreClientOptionSNS = func() (option.ClientOption, error) { return option.WithoutAuthentication(), nil }
			newSNSWorkspaceApp = func(context.Context, bool, option.ClientOption) (snsNotifyApp, error) { return app, nil }
			t.Cleanup(func() { firestoreClientOptionSNS, newSNSWorkspaceApp = previousOption, previousApp })

			event := events.SNSEvent{Records: []events.SNSEventRecord{{SNS: events.SNSEntity{Subject: subject, Message: message}}}}
			err := handler(context.Background(), event)
			if tc.sendErr == nil {
				if err != nil {
					t.Fatalf("handler: %v", err)
				}
				if !strings.Contains(logs.String(), `"status":"sent"`) {
					t.Fatalf("missing successful dispatch status: %s", logs.String())
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tc.wantClass) {
					t.Fatalf("expected safe delivery failure class, got %v", err)
				}
				if strings.Contains(err.Error(), message) {
					t.Fatalf("handler error exposed payload: %v", err)
				}
				if !strings.Contains(logs.String(), `"error_class":"`+tc.wantClass+`"`) {
					t.Fatalf("missing safe error class: %s", logs.String())
				}
			}
			if len(app.messages) != 1 || app.messages[0] != buildDiscordNotification(subject, message) {
				t.Fatalf("notification content changed: %q", app.messages)
			}
			if !app.closed {
				t.Fatal("workspace app was not closed")
			}
			for _, private := range []string{subject, message} {
				if strings.Contains(logs.String(), private) {
					t.Fatalf("logs exposed SNS payload: %s", logs.String())
				}
			}
		})
	}
}

func TestHandlerInitializationErrorsDoNotExposeDependencyPayload(t *testing.T) {
	const privateValue = "PRIVATE_DEPENDENCY_BODY_831"
	for _, tc := range []struct {
		name      string
		optionErr error
		initErr   error
		wantClass string
	}{
		{name: "firestore option", optionErr: errors.New(privateValue), wantClass: "firestore_option_failed"},
		{name: "workspace init", initErr: errors.New(privateValue), wantClass: "workspace_init_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })

			previousOption, previousApp := firestoreClientOptionSNS, newSNSWorkspaceApp
			firestoreClientOptionSNS = func() (option.ClientOption, error) { return option.WithoutAuthentication(), tc.optionErr }
			newSNSWorkspaceApp = func(context.Context, bool, option.ClientOption) (snsNotifyApp, error) { return nil, tc.initErr }
			t.Cleanup(func() { firestoreClientOptionSNS, newSNSWorkspaceApp = previousOption, previousApp })

			err := handler(context.Background(), events.SNSEvent{})
			if err == nil || strings.Contains(err.Error(), privateValue) {
				t.Fatalf("expected sanitized initialization error, got %v", err)
			}
			if !strings.Contains(logs.String(), `"error_class":"`+tc.wantClass+`"`) || strings.Contains(logs.String(), privateValue) {
				t.Fatalf("expected sanitized initialization log: %s", logs.String())
			}
		})
	}
}

func TestBuildDiscordNotificationTruncatesMessageUTF8Safely(t *testing.T) {
	subject := "subject"
	message := strings.Repeat("勉強🚀", 700)

	notify := buildDiscordNotification(subject, message)

	if len([]rune(notify)) > maxDiscordMessageLength {
		t.Fatalf("expected notification to fit length limit, got %d", len([]rune(notify)))
	}
	if !utf8.ValidString(notify) {
		t.Fatalf("expected valid UTF-8 notification, got %q", notify)
	}
	if !strings.HasSuffix(notify, truncatedSuffix) {
		t.Fatalf("expected truncated suffix, got %q", notify)
	}
}

func TestBuildDiscordNotificationAccountsForSubjectLength(t *testing.T) {
	subject := strings.Repeat("件名", 600)
	message := strings.Repeat("本文", 700)

	notify := buildDiscordNotification(subject, message)

	if len([]rune(notify)) > maxDiscordMessageLength {
		t.Fatalf("expected notification to fit length limit, got %d", len([]rune(notify)))
	}
	if !utf8.ValidString(notify) {
		t.Fatalf("expected valid UTF-8 notification, got %q", notify)
	}
	if !strings.Contains(notify, "件名") {
		t.Fatalf("expected notification to retain subject context, got %q", notify)
	}
}
