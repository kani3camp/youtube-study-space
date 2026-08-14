package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/youtube/v3"

	"app.modules/core/repository"
	"app.modules/core/youtubebot"
)

type fakeDurableLiveChatPageIngester struct {
	calls             int
	liveChatID        string
	expectedPageToken string
	nextPageToken     string
	messages          []repository.LiveChatIngestMessage
	ingestedAt        time.Time
	err               error
}

func (f *fakeDurableLiveChatPageIngester) IngestLiveChatSourcePage(
	_ context.Context,
	liveChatID string,
	expectedPageToken string,
	nextPageToken string,
	messages []repository.LiveChatIngestMessage,
	ingestedAt time.Time,
) error {
	f.calls++
	f.liveChatID = liveChatID
	f.expectedPageToken = expectedPageToken
	f.nextPageToken = nextPageToken
	f.messages = append([]repository.LiveChatIngestMessage(nil), messages...)
	f.ingestedAt = ingestedAt
	return f.err
}

func TestReadDurableLiveChatIngestEnabled(t *testing.T) {
	t.Run("default off", func(t *testing.T) {
		t.Setenv(durableLiveChatIngestEnabledEnv, "")
		enabled, err := readDurableLiveChatIngestEnabled()
		require.NoError(t, err)
		assert.False(t, enabled)
	})

	t.Run("explicit true", func(t *testing.T) {
		t.Setenv(durableLiveChatIngestEnabledEnv, " true ")
		enabled, err := readDurableLiveChatIngestEnabled()
		require.NoError(t, err)
		assert.True(t, enabled)
	})

	t.Run("invalid value", func(t *testing.T) {
		t.Setenv(durableLiveChatIngestEnabledEnv, "sometimes")
		_, err := readDurableLiveChatIngestEnabled()
		require.Error(t, err)
		assert.ErrorContains(t, err, durableLiveChatIngestEnabledEnv)
	})
}

func TestIngestFetchedLiveChatPagePersistsOnlyAuthorTextMessages(t *testing.T) {
	ingester := &fakeDurableLiveChatPageIngester{}
	ingestedAt := time.Date(2026, 8, 15, 1, 30, 0, 0, time.UTC)
	messages := []*youtube.LiveChatMessage{
		{
			Id: "text-1",
			AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
				ChannelId:   "author-1",
				DisplayName: "Viewer",
			},
			Snippet: &youtube.LiveChatMessageSnippet{
				LiveChatId:  "chat-1",
				PublishedAt: "2026-08-15T01:29:00Z",
				Type:        youtubebot.TextMessageEvent,
				TextMessageDetails: &youtube.LiveChatTextMessageDetails{
					MessageText: "!info",
				},
			},
		},
		{
			Id: "sponsor-event",
			AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
				ChannelId: "author-2",
			},
			Snippet: &youtube.LiveChatMessageSnippet{
				LiveChatId:  "chat-1",
				PublishedAt: "2026-08-15T01:29:30Z",
				Type:        youtubebot.NewSponsorEvent,
			},
		},
	}

	err := ingestFetchedLiveChatPage(
		context.Background(),
		ingester,
		"chat-1",
		"page-a",
		"page-b",
		messages,
		ingestedAt,
	)
	require.NoError(t, err)
	assert.Equal(t, 1, ingester.calls)
	assert.Equal(t, "chat-1", ingester.liveChatID)
	assert.Equal(t, "page-a", ingester.expectedPageToken)
	assert.Equal(t, "page-b", ingester.nextPageToken)
	assert.Equal(t, ingestedAt, ingester.ingestedAt)
	require.Len(t, ingester.messages, 1)
	assert.Equal(t, "text-1", ingester.messages[0].History.ID)
	assert.Equal(t, "!info", ingester.messages[0].History.MessageText)
}

func TestIngestFetchedLiveChatPageAdvancesEmptyPage(t *testing.T) {
	ingester := &fakeDurableLiveChatPageIngester{}
	ingestedAt := time.Date(2026, 8, 15, 1, 31, 0, 0, time.UTC)

	require.NoError(t, ingestFetchedLiveChatPage(
		context.Background(),
		ingester,
		"chat-1",
		"page-a",
		"page-b",
		nil,
		ingestedAt,
	))
	assert.Equal(t, 1, ingester.calls)
	assert.Empty(t, ingester.messages)
	assert.Equal(t, "page-b", ingester.nextPageToken)
}

func TestIngestFetchedLiveChatPageRejectsMalformedSourceBeforeCommit(t *testing.T) {
	ingester := &fakeDurableLiveChatPageIngester{}

	err := ingestFetchedLiveChatPage(
		context.Background(),
		ingester,
		"chat-1",
		"page-a",
		"page-b",
		[]*youtube.LiveChatMessage{nil},
		time.Now(),
	)
	require.Error(t, err)
	assert.Zero(t, ingester.calls)
}

func TestIngestFetchedLiveChatPageWrapsRepositoryFailure(t *testing.T) {
	ingester := &fakeDurableLiveChatPageIngester{err: errors.New("firestore unavailable")}

	err := ingestFetchedLiveChatPage(
		context.Background(),
		ingester,
		"chat-1",
		"page-a",
		"page-b",
		nil,
		time.Now(),
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "durable live chat page")
	assert.ErrorContains(t, err, "firestore unavailable")
}
