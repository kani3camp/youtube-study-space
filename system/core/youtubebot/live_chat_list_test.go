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

// HTTP 500 は呼び出し元へエラーとして返し、取得位置を進めさせない。
func TestListMessagesReturnsErrorOnInternalServerError(t *testing.T) {
	var requestedPageTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/youtube/v3/liveChat/messages", r.URL.Path)
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
