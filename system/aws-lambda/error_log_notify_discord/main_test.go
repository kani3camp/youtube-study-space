package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/aws-lambda-go/events"
	"google.golang.org/api/option"
)

type mockErrorLogNotifyApp struct {
	sendErr  error
	messages []string
	closed   bool
}

func (m *mockErrorLogNotifyApp) MessageToOwnerOrError(ctx context.Context, message string) error {
	m.messages = append(m.messages, message)
	return m.sendErr
}

func (m *mockErrorLogNotifyApp) CloseFirestoreClient() {
	m.closed = true
}

func TestBuildDiscordMessageChunksIncludesLogLines(t *testing.T) {
	data := &events.CloudwatchLogsData{
		LogGroup:  "/aws/lambda/youtube_organize_database",
		LogStream: "2025/01/01/[$LATEST]abc",
		LogEvents: []events.CloudwatchLogsLogEvent{
			{ID: "1", Timestamp: 123, Message: `{"level":"ERROR","msg":"boom"}`},
		},
	}
	chunks := buildDiscordMessageChunks(data, "req-1")
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "/aws/lambda/youtube_organize_database") {
		t.Fatalf("expected log group in chunk: %q", chunks[0])
	}
	if !strings.Contains(chunks[0], "invoker_request_id=req-1") {
		t.Fatalf("expected invoker request id: %q", chunks[0])
	}
	if !strings.Contains(chunks[0], "chunk=1/1") {
		t.Fatalf("expected chunk number: %q", chunks[0])
	}
	if !strings.Contains(chunks[0], `"msg":"boom"`) {
		t.Fatalf("expected raw message: %q", chunks[0])
	}
}

func TestBuildDiscordMessageChunksIncludesHeaderInEveryChunk(t *testing.T) {
	data := &events.CloudwatchLogsData{
		LogGroup:  "/aws/lambda/check_live_stream_status",
		LogStream: "2025/01/01/[$LATEST]abc",
		LogEvents: []events.CloudwatchLogsLogEvent{
			{ID: "1", Timestamp: 123, Message: strings.Repeat("勉強🚀", 2000)},
		},
	}

	chunks := buildDiscordMessageChunks(data, "req-1")
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len([]rune(chunk)) > maxDiscordMessageLength {
			t.Fatalf("chunk %d over limit: %d", i, len([]rune(chunk)))
		}
		if !strings.Contains(chunk, "/aws/lambda/check_live_stream_status") {
			t.Fatalf("expected log group in chunk %d: %q", i, chunk)
		}
		if !strings.Contains(chunk, "logStream=2025/01/01/[$LATEST]abc") {
			t.Fatalf("expected log stream in chunk %d: %q", i, chunk)
		}
		expectedChunkNumber := "chunk=" + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(chunks))
		if !strings.Contains(chunk, expectedChunkNumber) {
			t.Fatalf("expected %s in chunk %d: %q", expectedChunkNumber, i, chunk)
		}
	}
}

func TestSplitToDiscordSizedChunksUTF8Safe(t *testing.T) {
	long := strings.Repeat("勉強🚀", 400)
	chunks := splitToDiscordSizedChunks(long, 200)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len([]rune(c)) > 200 {
			t.Fatalf("chunk over limit: %d", len([]rune(c)))
		}
		if !utf8.ValidString(c) {
			t.Fatalf("invalid utf-8: %q", c)
		}
	}
}

func TestHandlerReturnsErrorWhenWorkspaceInitFails(t *testing.T) {
	restore := stubErrorLogNotifyDeps(t, nil, errors.New("workspace init failed"), nil)
	defer restore()

	err := handler(context.Background(), events.CloudwatchLogsEvent{})
	if err == nil || !strings.Contains(err.Error(), "workspace init failed") {
		t.Fatalf("expected workspace init error, got %v", err)
	}
}

func TestHandlerReturnsErrorWhenOwnerMessageFails(t *testing.T) {
	app := &mockErrorLogNotifyApp{sendErr: errors.New("discord unavailable")}
	restore := stubErrorLogNotifyDeps(t, nil, nil, app)
	defer restore()

	ev := mustCloudwatchLogsEvent(t, events.CloudwatchLogsData{
		LogGroup:  "/aws/lambda/check_live_stream_status",
		LogStream: "2025/01/01/[$LATEST]abc",
		LogEvents: []events.CloudwatchLogsLogEvent{
			{ID: "1", Timestamp: 123, Message: `{"level":"ERROR","msg":"boom"}`},
		},
	})

	err := handler(context.Background(), ev)
	if err == nil || !strings.Contains(err.Error(), "send log notification to owner") {
		t.Fatalf("expected owner send error, got %v", err)
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient")
	}
}

func TestHandlerSuccessSendsAllChunksAndClosesClient(t *testing.T) {
	app := &mockErrorLogNotifyApp{}
	restore := stubErrorLogNotifyDeps(t, nil, nil, app)
	defer restore()

	ev := mustCloudwatchLogsEvent(t, events.CloudwatchLogsData{
		LogGroup:  "/aws/lambda/check_live_stream_status",
		LogStream: "2025/01/01/[$LATEST]abc",
		LogEvents: []events.CloudwatchLogsLogEvent{
			{ID: "1", Timestamp: 123, Message: strings.Repeat("勉強🚀", 2000)},
		},
	})

	if err := handler(context.Background(), ev); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(app.messages) < 2 {
		t.Fatalf("expected multiple chunks to be sent, got %d", len(app.messages))
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient")
	}
}

func TestHandlerReturnsErrorWhenFirestoreInitFails(t *testing.T) {
	restore := stubErrorLogNotifyDeps(t, errors.New("firestore init failed"), nil, nil)
	defer restore()

	err := handler(context.Background(), events.CloudwatchLogsEvent{})
	if err == nil || !strings.Contains(err.Error(), "firestore init failed") {
		t.Fatalf("expected firestore init error, got %v", err)
	}
}

func TestHandlerReturnsErrorWhenPayloadParseFails(t *testing.T) {
	app := &mockErrorLogNotifyApp{}
	restore := stubErrorLogNotifyDeps(t, nil, nil, app)
	defer restore()

	ev := events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{Data: "not-base64"},
	}

	err := handler(context.Background(), ev)
	if err == nil || !strings.Contains(err.Error(), "parse CloudWatch Logs") {
		t.Fatalf("expected parse error, got %v", err)
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient")
	}
}

func TestHandlerReturnsNilWhenNoLogEvents(t *testing.T) {
	app := &mockErrorLogNotifyApp{}
	restore := stubErrorLogNotifyDeps(t, nil, nil, app)
	defer restore()

	ev := mustCloudwatchLogsEvent(t, events.CloudwatchLogsData{
		LogGroup:  "/aws/lambda/check_live_stream_status",
		LogStream: "2025/01/01/[$LATEST]abc",
	})

	if err := handler(context.Background(), ev); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(app.messages) != 0 {
		t.Fatalf("expected no owner messages, got %#v", app.messages)
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient")
	}
}

func stubErrorLogNotifyDeps(
	t *testing.T,
	firestoreErr error,
	newAppErr error,
	app errorLogNotifyApp,
) func() {
	t.Helper()

	origF := firestoreClientOptionErrorLog
	origN := newErrorLogWorkspaceApp

	firestoreClientOptionErrorLog = func() (option.ClientOption, error) {
		return option.WithoutAuthentication(), firestoreErr
	}
	newErrorLogWorkspaceApp = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (errorLogNotifyApp, error) {
		if newAppErr != nil {
			return nil, newAppErr
		}
		return app, nil
	}

	return func() {
		firestoreClientOptionErrorLog = origF
		newErrorLogWorkspaceApp = origN
	}
}

func mustCloudwatchLogsEvent(t *testing.T, data events.CloudwatchLogsData) events.CloudwatchLogsEvent {
	t.Helper()

	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	return events.CloudwatchLogsEvent{
		AWSLogs: events.CloudwatchLogsRawData{
			Data: base64.StdEncoding.EncodeToString(compressed.Bytes()),
		},
	}
}
