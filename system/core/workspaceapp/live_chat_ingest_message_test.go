package workspaceapp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/youtube/v3"

	"app.modules/core/timeutil"
	"app.modules/core/youtubebot"
)

func TestBuildLiveChatIngestMessage(t *testing.T) {
	t.Parallel()

	message := &youtube.LiveChatMessage{
		Id: "message-1",
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
			ChannelId:       "author-1",
			DisplayName:     "@Member User",
			ProfileImageUrl: "https://example.com/profile.png",
			IsChatModerator: true,
			IsChatOwner:     false,
			IsChatSponsor:   true,
		},
		Snippet: &youtube.LiveChatMessageSnippet{
			LiveChatId:  "chat-1",
			PublishedAt: "2026-08-15T06:00:00.123456Z",
			Type:        youtubebot.TextMessageEvent,
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: "!info",
			},
		},
	}

	got, err := BuildLiveChatIngestMessage(message)
	require.NoError(t, err)
	assert.Equal(t, "message-1", got.History.ID)
	assert.Equal(t, "chat-1", got.History.LiveChatID)
	assert.Equal(t, "author-1", got.History.AuthorChannelID)
	assert.Equal(t, "@Member User", got.History.AuthorDisplayName, "history keeps the raw display name")
	assert.Equal(t, "https://example.com/profile.png", got.History.AuthorProfileImageURL)
	assert.True(t, got.History.AuthorIsChatModerator)
	assert.Equal(t, "!info", got.History.MessageText)
	assert.Equal(t, youtubebot.TextMessageEvent, got.History.Type)
	assert.True(t, got.AuthorIsChatSponsor)
	assert.False(t, got.AuthorIsChatOwner)

	wantPublishedAt := time.Date(2026, 8, 15, 15, 0, 0, 123456000, timeutil.JapanLocation())
	assert.Equal(t, wantPublishedAt, got.History.PublishedAt)
}

func TestBuildLiveChatIngestMessagePreservesOwnerMetadata(t *testing.T) {
	t.Parallel()

	message := &youtube.LiveChatMessage{
		Id: "message-owner",
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
			ChannelId:   "owner",
			DisplayName: "Owner",
			IsChatOwner: true,
		},
		Snippet: &youtube.LiveChatMessageSnippet{
			LiveChatId:  "chat-1",
			PublishedAt: "2026-08-15T06:00:00Z",
			Type:        youtubebot.TextMessageEvent,
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: "hello",
			},
		},
	}

	got, err := BuildLiveChatIngestMessage(message)
	require.NoError(t, err)
	assert.True(t, got.AuthorIsChatOwner)
	assert.False(t, got.AuthorIsChatSponsor)
}

func TestBuildLiveChatIngestMessageRejectsIncompleteSource(t *testing.T) {
	t.Parallel()

	_, err := BuildLiveChatIngestMessage(nil)
	require.Error(t, err)

	_, err = BuildLiveChatIngestMessage(&youtube.LiveChatMessage{})
	require.Error(t, err)

	_, err = BuildLiveChatIngestMessage(&youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{},
	})
	require.Error(t, err)

	_, err = BuildLiveChatIngestMessage(&youtube.LiveChatMessage{
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{},
		Snippet: &youtube.LiveChatMessageSnippet{
			Type: youtubebot.NewSponsorEvent,
		},
	})
	require.Error(t, err)
}

func TestBuildLiveChatIngestMessageRejectsInvalidPublishedAt(t *testing.T) {
	t.Parallel()

	message := &youtube.LiveChatMessage{
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{},
		Snippet: &youtube.LiveChatMessageSnippet{
			PublishedAt: "not-a-timestamp",
			Type:        youtubebot.TextMessageEvent,
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: "hello",
			},
		},
	}
	_, err := BuildLiveChatIngestMessage(message)
	require.Error(t, err)
	assert.ErrorContains(t, err, "published at")
}
