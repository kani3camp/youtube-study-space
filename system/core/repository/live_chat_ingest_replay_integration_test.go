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

func TestFirestoreRepository_IngestLiveChatPageReplaysUnknownCommitOutcome(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-replay"
	firstIngestedAt := time.Date(2026, 8, 14, 18, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!in", firstIngestedAt)

	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"cursor-before",
		"cursor-after",
		[]repository.LiveChatHistoryDoc{message},
		firstIngestedAt,
	))

	// Simulate a caller that lost the first transaction's successful response.
	// It must be safe to repeat the exact same cursor transition and payload.
	retryAt := firstIngestedAt.Add(time.Minute)
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"cursor-before",
		"cursor-after",
		[]repository.LiveChatHistoryDoc{message},
		retryAt,
	))

	state := readLiveChatStreamState(t, controller, liveChatID)
	assert.Equal(t, "cursor-after", state.NextPageToken)
	assert.EqualValues(t, 1, state.NextSequence)
	// Replay is a no-op: retain the timestamp from the actual commit rather than
	// pretending another page fetch was persisted.
	assert.Equal(t, firstIngestedAt, state.UpdatedAt)
	assert.Equal(t, firstIngestedAt, state.LastFetchSucceededAt)
	assertDurableLiveChatMessage(t, controller, message, 0, firstIngestedAt)
}

func TestFirestoreRepository_IngestLiveChatPageRejectsPartialReplayAfterCursorAdvance(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-partial-replay"
	ingestedAt := time.Date(2026, 8, 14, 19, 0, 0, 0, time.UTC)
	first := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "first", ingestedAt)

	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"cursor-before",
		"cursor-after",
		[]repository.LiveChatHistoryDoc{first},
		ingestedAt,
	))

	unexpected := liveChatHistoryDoc(liveChatID, "message-2", "author-2", "unexpected", ingestedAt)
	err := controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"cursor-before",
		"cursor-after",
		[]repository.LiveChatHistoryDoc{first, unexpected},
		ingestedAt.Add(time.Minute),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatIngestCorruptState)

	state := readLiveChatStreamState(t, controller, liveChatID)
	assert.EqualValues(t, 1, state.NextSequence)
}
