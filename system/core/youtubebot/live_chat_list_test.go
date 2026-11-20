package youtubebot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
)

// HTTP 500 を成功扱いにすると空の取得位置が保存され、処理済みの投稿が再実行される（#1620）。
func TestListMessagesReturnsErrorOnInternalServerError(t *testing.T) {
	var requestedPageTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/youtube/v3/liveChat/messages", r.URL.Path)
		requestedPageTokens = append(requestedPageTokens, r.URL.Query().Get("pageToken"))
		writeGoogleAPIError(t, w, http.StatusInternalServerError, "backendError")
	}))
	defer server.Close()

	bot := &YoutubeLiveChatBot{
		LiveChatID:        "live-chat-id",
		BotYoutubeService: newTestYouTubeService(t, server),
	}

	messages, nextPageToken, pollingIntervalMillis, err := bot.ListMessages(context.Background(), "saved-token")

	var apiErr *googleapi.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	assert.Nil(t, messages)
	assert.Empty(t, nextPageToken)
	assert.Zero(t, pollingIntervalMillis)
	// 500 ではライブチャットIDの更新や同じ呼び出し内での再試行はせず、呼び出し元のバックオフに任せる。
	assert.Equal(t, []string{"saved-token"}, requestedPageTokens)
}
