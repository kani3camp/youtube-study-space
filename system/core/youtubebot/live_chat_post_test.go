package youtubebot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mock_repository "app.modules/core/repository/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func TestPostMessageLogsWarnAndSkipsLiveChatEnded(t *testing.T) {
	var insertCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/youtube/v3/liveChat/messages", r.URL.Path)
		insertCount++
		if insertCount == 1 {
			writeGoogleAPIError(t, w, http.StatusInternalServerError, "backendError")
			return
		}
		writeGoogleAPIError(t, w, http.StatusForbidden, "liveChatEnded")
	}))
	defer server.Close()

	service := newTestYouTubeService(t, server)
	bot := &YoutubeLiveChatBot{
		LiveChatID:        "live-chat-id",
		BotYoutubeService: service,
	}
	logs := captureSlog(t)

	err := bot.postMessage(context.Background(), "hello")

	assert.NoError(t, err)
	assert.Equal(t, 2, insertCount)

	entries := readLogEntries(t, logs)
	assert.True(t, hasLogEntry(entries, "WARN", "first post failed; retrying"))
	assert.True(t, hasLogEntry(entries, "WARN", "post skipped because live chat ended"))
	assert.False(t, hasLogLevel(entries, "ERROR"))
}

func TestPostMessageSkipsRetryWhenFirstPostFailsWithLiveChatEnded(t *testing.T) {
	var insertCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/youtube/v3/liveChat/messages", r.URL.Path)
		insertCount++
		writeGoogleAPIError(t, w, http.StatusForbidden, "liveChatEnded")
	}))
	defer server.Close()

	service := newTestYouTubeService(t, server)
	bot := &YoutubeLiveChatBot{
		LiveChatID:        "live-chat-id",
		BotYoutubeService: service,
	}
	logs := captureSlog(t)

	err := bot.postMessage(context.Background(), "hello")

	assert.NoError(t, err)
	assert.Equal(t, 1, insertCount)

	entries := readLogEntries(t, logs)
	assert.True(t, hasLogEntry(entries, "WARN", "post skipped because live chat ended"))
	assert.False(t, hasLogEntry(entries, "WARN", "first post failed; retrying"))
	assert.False(t, hasLogLevel(entries, "ERROR"))
}

func TestPostMessageLogsErrorAfterRefreshRetryFails(t *testing.T) {
	var insertCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/youtube/v3/liveChat/messages":
			insertCount++
			writeGoogleAPIError(t, w, http.StatusInternalServerError, "backendError")
		case "/youtube/v3/liveBroadcasts":
			w.Header().Set("Content-Type", "application/json")
			writeResponseBody(t, w, `{"items":[{"snippet":{"liveChatId":"new-live-chat-id"}}]}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repository := mock_repository.NewMockRepository(ctrl)
	repository.EXPECT().
		UpdateLiveChatID(gomock.Any(), nil, "new-live-chat-id").
		Return(nil)

	service := newTestYouTubeService(t, server)
	bot := &YoutubeLiveChatBot{
		LiveChatID:            "old-live-chat-id",
		ChannelYoutubeService: service,
		BotYoutubeService:     service,
		FirestoreController:   repository,
	}
	logs := captureSlog(t)

	err := bot.postMessage(context.Background(), "hello")

	assert.Error(t, err)
	assert.Equal(t, 3, insertCount)
	assert.Equal(t, "new-live-chat-id", bot.LiveChatID)

	entries := readLogEntries(t, logs)
	assert.True(t, hasLogEntry(entries, "WARN", "first post failed; retrying"))
	assert.True(t, hasLogEntry(entries, "WARN", "second post failed; refreshing live chat id"))
	assert.True(t, hasLogEntry(entries, "ERROR", "third post failed"))
}

func newTestYouTubeService(t *testing.T, server *httptest.Server) *youtube.Service {
	t.Helper()

	service, err := youtube.NewService(
		context.Background(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(server.Client()),
		option.WithoutAuthentication(),
	)
	if err != nil {
		t.Fatalf("failed to create YouTube service: %v", err)
	}
	return service
}

func writeGoogleAPIError(t *testing.T, w http.ResponseWriter, statusCode int, reason string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	writeResponseBody(
		t,
		w,
		`{"error":{"code":%d,"message":"%s","errors":[{"reason":"%s","message":"%s"}]}}`,
		statusCode,
		reason,
		reason,
		reason,
	)
}

func writeResponseBody(t *testing.T, w http.ResponseWriter, format string, args ...any) {
	t.Helper()

	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		t.Fatalf("failed to write response body: %v", err)
	}
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(old)
	})
	return &buf
}

func readLogEntries(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log output is not valid JSON: %v, output=%s", err, line)
		}
		entries = append(entries, entry)
	}
	return entries
}

func hasLogEntry(entries []map[string]any, level string, message string) bool {
	for _, entry := range entries {
		if entry["level"] == level && entry["msg"] == message {
			return true
		}
	}
	return false
}

func hasLogLevel(entries []map[string]any, level string) bool {
	for _, entry := range entries {
		if entry["level"] == level {
			return true
		}
	}
	return false
}
