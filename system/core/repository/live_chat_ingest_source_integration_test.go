//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"
)

func TestFirestoreRepository_IngestLiveChatSourcePagePersistsActorMetadata(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-source-actor"
	ingestedAt := time.Date(2026, 8, 15, 13, 0, 0, 0, time.UTC)
	history := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!info", ingestedAt.Add(-time.Minute))
	history.AuthorDisplayName = "@Member User"
	history.AuthorIsChatModerator = true
	source := repository.LiveChatIngestMessage{
		History:             history,
		AuthorIsChatOwner:   false,
		AuthorIsChatSponsor: true,
	}

	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"legacy-cursor",
		"cursor-1",
		[]repository.LiveChatIngestMessage{source},
		ingestedAt,
	))

	key, err := repository.LiveChatMessageKey(liveChatID, history.ID)
	require.NoError(t, err)
	snapshot, err := controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(key).Get(ctx)
	require.NoError(t, err)
	var inbox repository.LiveChatInboxDoc
	require.NoError(t, snapshot.DataTo(&inbox))

	assert.Equal(t, "Member User", inbox.AuthorDisplayName)
	assert.True(t, inbox.AuthorIsChatModerator)
	assert.False(t, inbox.AuthorIsChatOwner)
	assert.True(t, inbox.AuthorIsChatMember)
	assert.Equal(t, history.AuthorChannelID, inbox.AuthorChannelID)
	assert.Equal(t, history.MessageText, inbox.MessageText)

	// The analytics/history document remains the historical schema and preserves
	// its raw display name. Actor-only metadata lives in Inbox.
	historySnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatHistory).Doc(key).Get(ctx)
	require.NoError(t, err)
	var storedHistory repository.LiveChatHistoryDoc
	require.NoError(t, historySnapshot.DataTo(&storedHistory))
	assert.Equal(t, "@Member User", storedHistory.AuthorDisplayName)
	assert.Equal(t, history, storedHistory)
}

func TestFirestoreRepository_IngestLiveChatSourcePageRejectsConflictingActorMetadata(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-source-conflict"
	ingestedAt := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	history := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!info", ingestedAt)
	first := repository.LiveChatIngestMessage{History: history}
	second := repository.LiveChatIngestMessage{History: history, AuthorIsChatOwner: true}

	err := controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-1",
		[]repository.LiveChatIngestMessage{first, second},
		ingestedAt,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting payload or actor metadata")
}
