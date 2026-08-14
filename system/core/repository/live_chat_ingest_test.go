package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveChatMessageKey(t *testing.T) {
	t.Parallel()

	key1, err := LiveChatMessageKey("chat-a", "message-1")
	require.NoError(t, err)
	key2, err := LiveChatMessageKey("chat-a", "message-1")
	require.NoError(t, err)
	otherMessage, err := LiveChatMessageKey("chat-a", "message-2")
	require.NoError(t, err)
	otherChat, err := LiveChatMessageKey("chat-b", "message-1")
	require.NoError(t, err)

	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 64)
	assert.NotContains(t, key1, "/")
	assert.NotEqual(t, key1, otherMessage)
	assert.NotEqual(t, key1, otherChat)
}

func TestLiveChatMessageKeyUsesLengthDelimitedParts(t *testing.T) {
	t.Parallel()

	first, err := LiveChatMessageKey("ab", "c")
	require.NoError(t, err)
	second, err := LiveChatMessageKey("a", "bc")
	require.NoError(t, err)
	assert.NotEqual(t, first, second)
}

func TestLiveChatMessageKeyRejectsEmptyIDs(t *testing.T) {
	t.Parallel()

	_, err := LiveChatMessageKey("   ", "message")
	require.Error(t, err)
	_, err = LiveChatMessageKey("chat", "\t")
	require.Error(t, err)
	_, err = LiveChatStreamKey("\n")
	require.Error(t, err)
}

func TestValidateAndDeduplicateLiveChatMessages(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	message := LiveChatHistoryDoc{
		AuthorChannelID:       "author-1",
		AuthorDisplayName:     "User",
		AuthorProfileImageURL: "https://example.com/u.png",
		AuthorIsChatModerator: true,
		ID:                    "message-1",
		LiveChatID:            "chat-1",
		MessageText:           "!in",
		PublishedAt:           publishedAt,
		Type:                  "textMessageEvent",
	}

	got, err := validateAndDeduplicateLiveChatMessages("chat-1", []LiveChatHistoryDoc{message, message})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, message, got[0])

	conflict := message
	conflict.MessageText = "!out"
	_, err = validateAndDeduplicateLiveChatMessages("chat-1", []LiveChatHistoryDoc{message, conflict})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting payload")

	wrongChat := message
	wrongChat.LiveChatID = "chat-2"
	_, err = validateAndDeduplicateLiveChatMessages("chat-1", []LiveChatHistoryDoc{wrongChat})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected")

	emptyID := message
	emptyID.ID = strings.Repeat(" ", 2)
	_, err = validateAndDeduplicateLiveChatMessages("chat-1", []LiveChatHistoryDoc{emptyID})
	require.Error(t, err)
}

func TestMaxAtomicLiveChatIngestMessagesFitsFirestoreLimit(t *testing.T) {
	t.Parallel()

	writes := 2*MaxAtomicLiveChatIngestMessages + 1
	assert.LessOrEqual(t, writes, FirestoreWritesLimitPerRequest)
	assert.Greater(t, 2*(MaxAtomicLiveChatIngestMessages+1)+1, FirestoreWritesLimitPerRequest)
}
