package youtubebot

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newCancelingYouTubeService は、最初のリクエストで cancel を呼んで失敗させ、
// 以降も含めて送信されたリクエストのパスを記録する YouTube サービスを返す。
func newCancelingYouTubeService(t *testing.T, cancel context.CancelFunc, requestedPaths *[]string) *youtube.Service {
	t.Helper()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*requestedPaths = append(*requestedPaths, req.URL.Path)
		cancel()
		return nil, errors.New("transport failure")
	})}
	service, err := youtube.NewService(
		context.Background(),
		option.WithEndpoint("http://youtube.invalid/"),
		option.WithHTTPClient(client),
		option.WithoutAuthentication(),
	)
	require.NoError(t, err)
	return service
}

// 送信中に ctx がキャンセルされたら、再試行も live chat id の更新もせずにキャンセルのエラーを返す。
func TestPostMessageStopsRetryingWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requestedPaths []string
	service := newCancelingYouTubeService(t, cancel, &requestedPaths)
	bot := &YoutubeLiveChatBot{
		LiveChatID:            "live-chat-id",
		ChannelYoutubeService: service,
		BotYoutubeService:     service,
	}

	err := bot.postMessage(ctx, "hello")

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{"/youtube/v3/liveChat/messages"}, requestedPaths)
}

// ブロック要求中に ctx がキャンセルされたら、live chat id の更新も再試行もせずにキャンセルのエラーを返す。
func TestBanUserStopsRetryingWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requestedPaths []string
	service := newCancelingYouTubeService(t, cancel, &requestedPaths)
	bot := &YoutubeLiveChatBot{
		LiveChatID:            "live-chat-id",
		ChannelYoutubeService: service,
		BotYoutubeService:     service,
	}

	err := bot.BanUser(ctx, "user-id")

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{"/youtube/v3/liveChat/bans"}, requestedPaths)
}
