package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveChatReplyOutboxKey(t *testing.T) {
	t.Parallel()

	key, err := LiveChatReplyOutboxKey("chat-1", "message-1", "primary")
	require.NoError(t, err)
	same, err := LiveChatReplyOutboxKey("chat-1", "message-1", "primary")
	require.NoError(t, err)
	otherSlot, err := LiveChatReplyOutboxKey("chat-1", "message-1", "secondary")
	require.NoError(t, err)
	otherMessage, err := LiveChatReplyOutboxKey("chat-1", "message-2", "primary")
	require.NoError(t, err)

	assert.Equal(t, key, same)
	assert.Len(t, key, 64)
	assert.NotEqual(t, key, otherSlot)
	assert.NotEqual(t, key, otherMessage)
}

func TestValidateNewLiveChatReplyIntent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 15, 2, 0, 0, 0, time.UTC)
	valid := LiveChatReplyOutboxDoc{
		LiveChatID:            "chat-1",
		SourceMessageID:       "message-1",
		SourceAuthorChannelID: "author-1",
		IntentSlot:            "primary",
		SourceSequence:        1,
		Message:               "reply",
		Status:                LiveChatReplyOutboxPending,
		CreatedAt:             now,
		AvailableAt:           now,
	}
	require.NoError(t, validateNewLiveChatReplyIntent(valid))

	tests := []struct {
		name   string
		mutate func(*LiveChatReplyOutboxDoc)
	}{
		{name: "empty live chat", mutate: func(v *LiveChatReplyOutboxDoc) { v.LiveChatID = "" }},
		{name: "empty source message", mutate: func(v *LiveChatReplyOutboxDoc) { v.SourceMessageID = "" }},
		{name: "empty slot", mutate: func(v *LiveChatReplyOutboxDoc) { v.IntentSlot = "" }},
		{name: "negative sequence", mutate: func(v *LiveChatReplyOutboxDoc) { v.SourceSequence = -1 }},
		{name: "empty reply", mutate: func(v *LiveChatReplyOutboxDoc) { v.Message = "" }},
		{name: "zero created at", mutate: func(v *LiveChatReplyOutboxDoc) { v.CreatedAt = time.Time{} }},
		{name: "zero available at", mutate: func(v *LiveChatReplyOutboxDoc) { v.AvailableAt = time.Time{} }},
		{name: "wrong initial status", mutate: func(v *LiveChatReplyOutboxDoc) { v.Status = LiveChatReplyOutboxDelivering }},
		{name: "delivery attempt already set", mutate: func(v *LiveChatReplyOutboxDoc) { v.AttemptCount = 1 }},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			candidate := valid
			tt.mutate(&candidate)
			require.Error(t, validateNewLiveChatReplyIntent(candidate))
		})
	}
}
