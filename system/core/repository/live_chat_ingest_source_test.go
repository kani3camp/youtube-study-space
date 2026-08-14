package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLiveChatInboxFromSourcePreservesLegacyActorSemantics(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	ingestedAt := publishedAt.Add(time.Minute)
	source := LiveChatIngestMessage{
		History: LiveChatHistoryDoc{
			AuthorChannelID:       "author-1",
			AuthorDisplayName:     "@Display Name",
			AuthorProfileImageURL: "https://example.com/profile.png",
			AuthorIsChatModerator: true,
			ID:                    "message-1",
			LiveChatID:            "chat-1",
			MessageText:           "!info",
			PublishedAt:           publishedAt,
			Type:                  "textMessageEvent",
		},
		AuthorIsChatOwner:   true,
		AuthorIsChatSponsor: false,
	}

	inbox := liveChatInboxFromSource(source, 7, ingestedAt)

	assert.Equal(t, "Display Name", inbox.AuthorDisplayName)
	assert.True(t, inbox.AuthorIsChatModerator)
	assert.True(t, inbox.AuthorIsChatOwner)
	assert.True(t, inbox.AuthorIsChatMember, "owner must be treated as member like legacy ProcessMessage")
	assert.EqualValues(t, 7, inbox.Sequence)
}

func TestLiveChatInboxFromSourceSponsorBecomesMember(t *testing.T) {
	t.Parallel()

	source := LiveChatIngestMessage{
		History: LiveChatHistoryDoc{
			AuthorDisplayName: "Viewer",
		},
		AuthorIsChatSponsor: true,
	}
	inbox := liveChatInboxFromSource(source, 0, time.Now())
	assert.False(t, inbox.AuthorIsChatOwner)
	assert.True(t, inbox.AuthorIsChatMember)
}
