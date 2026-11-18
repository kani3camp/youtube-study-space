package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"app.modules/core/youtubebot"
)

// fakeLiveChatPoller は取得位置をメモリに保存し、チャット取得は実際の YoutubeLiveChatBot に任せる。
type fakeLiveChatPoller struct {
	bot             *youtubebot.YoutubeLiveChatBot
	savedPageToken  string
	saveCalls       int
	ownerErrorCalls int
}

func (p *fakeLiveChatPoller) GetNextPageToken(context.Context, *firestore.Transaction) (string, error) {
	return p.savedPageToken, nil
}

func (p *fakeLiveChatPoller) ListLiveChatMessages(ctx context.Context, pageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	messages, nextPageToken, pollingIntervalMillis, err := p.bot.ListMessages(ctx, pageToken)
	if err != nil {
		return nil, "", 0, fmt.Errorf("list live chat messages: %w", err)
	}
	return messages, nextPageToken, pollingIntervalMillis, nil
}

func (p *fakeLiveChatPoller) SaveNextPageToken(_ context.Context, nextPageToken string) error {
	p.saveCalls++
	p.savedPageToken = nextPageToken
	return nil
}

func (p *fakeLiveChatPoller) MessageToOwnerWithError(context.Context, string, error) {
	p.ownerErrorCalls++
}

// YouTube API が一時的に HTTP 500 を返しても取得位置を失わず、復旧後は新着分だけを処理する（#1620）。
func TestPollLiveChatOnceKeepsPageTokenAcrossInternalServerError(t *testing.T) {
	var requestedPageTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/youtube/v3/liveChat/messages", r.URL.Path)
		requestedPageTokens = append(requestedPageTokens, r.URL.Query().Get("pageToken"))
		w.Header().Set("Content-Type", "application/json")
		if len(requestedPageTokens) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := fmt.Fprint(w, `{"error":{"code":500,"message":"backendError","errors":[{"reason":"backendError"}]}}`)
			require.NoError(t, err)
			return
		}
		_, err := fmt.Fprint(w, `{
			"nextPageToken": "next-token",
			"pollingIntervalMillis": 5000,
			"items": [{"id": "new-message", "snippet": {"type": "textMessageEvent", "displayMessage": "!in"}}]
		}`)
		require.NoError(t, err)
	}))
	defer server.Close()

	service, err := youtube.NewService(
		context.Background(),
		option.WithEndpoint(server.URL+"/"),
		option.WithHTTPClient(server.Client()),
		option.WithoutAuthentication(),
	)
	require.NoError(t, err)

	poller := &fakeLiveChatPoller{
		bot: &youtubebot.YoutubeLiveChatBot{
			LiveChatID:        "live-chat-id",
			BotYoutubeService: service,
		},
		savedPageToken: "saved-token",
	}
	var handledMessageIDs []string
	handle := func(_ context.Context, m *youtube.LiveChatMessage) {
		handledMessageIDs = append(handledMessageIDs, m.Id)
	}
	ctx := context.Background()

	// HTTP 500: 取得失敗として返し、取得位置は保存せず、メッセージも処理しない。
	_, err = pollLiveChatOnce(ctx, poller, handle)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errListLiveChatMessages), "err = %v", err)
	assert.Equal(t, "saved-token", poller.savedPageToken)
	assert.Zero(t, poller.saveCalls)
	assert.Empty(t, handledMessageIDs)

	// 復旧後: 同じ取得位置で再取得し、新着分を処理して取得位置を進める。
	result, err := pollLiveChatOnce(ctx, poller, handle)
	require.NoError(t, err)
	assert.Equal(t, 5000, result.pollingIntervalMillis)
	assert.Equal(t, "next-token", poller.savedPageToken)
	assert.Equal(t, 1, poller.saveCalls)
	assert.Equal(t, []string{"new-message"}, handledMessageIDs)

	assert.Equal(t, []string{"saved-token", "saved-token"}, requestedPageTokens)
	assert.Zero(t, poller.ownerErrorCalls)
}
