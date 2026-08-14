//go:build integration

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"
)

func liveChatHistoryDoc(liveChatID, messageID, authorID, text string, publishedAt time.Time) repository.LiveChatHistoryDoc {
	return repository.LiveChatHistoryDoc{
		AuthorChannelID:       authorID,
		AuthorDisplayName:     "Test User",
		AuthorProfileImageURL: "https://example.com/profile.png",
		AuthorIsChatModerator: false,
		ID:                    messageID,
		LiveChatID:            liveChatID,
		MessageText:           text,
		PublishedAt:           publishedAt,
		Type:                  "textMessageEvent",
	}
}

func TestFirestoreRepository_IngestLiveChatPageIsAtomicAndIdempotent(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-ingest"
	firstIngestedAt := time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC)
	messages := []repository.LiveChatHistoryDoc{
		liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!in 25 test", firstIngestedAt.Add(-2*time.Minute)),
		liveChatHistoryDoc(liveChatID, "message-2", "author-2", "hello", firstIngestedAt.Add(-time.Minute)),
	}

	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"legacy-token",
		"page-token-2",
		messages,
		firstIngestedAt,
	))

	assertDurableLiveChatMessage(t, controller, messages[0], 0, firstIngestedAt)
	assertDurableLiveChatMessage(t, controller, messages[1], 1, firstIngestedAt)
	state := readLiveChatStreamState(t, controller, liveChatID)
	assert.Equal(t, liveChatID, state.LiveChatID)
	assert.Equal(t, "page-token-2", state.NextPageToken)
	assert.EqualValues(t, 2, state.NextSequence)
	assert.Equal(t, firstIngestedAt, state.UpdatedAt)
	assert.Equal(t, firstIngestedAt, state.LastFetchSucceededAt)

	// A retry of the same fetched page may use the current persisted cursor.
	// Existing deterministic message documents must not allocate new sequence IDs.
	retryAt := firstIngestedAt.Add(time.Minute)
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"page-token-2",
		"page-token-2",
		messages,
		retryAt,
	))
	state = readLiveChatStreamState(t, controller, liveChatID)
	assert.EqualValues(t, 2, state.NextSequence)
	assert.Equal(t, retryAt, state.UpdatedAt)
	assert.Equal(t, retryAt, state.LastFetchSucceededAt)

	third := liveChatHistoryDoc(liveChatID, "message-3", "author-3", "!out", retryAt)
	secondPageAt := retryAt.Add(time.Minute)
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"page-token-2",
		"page-token-3",
		[]repository.LiveChatHistoryDoc{third},
		secondPageAt,
	))
	assertDurableLiveChatMessage(t, controller, third, 2, secondPageAt)
	state = readLiveChatStreamState(t, controller, liveChatID)
	assert.Equal(t, "page-token-3", state.NextPageToken)
	assert.EqualValues(t, 3, state.NextSequence)
}

func TestFirestoreRepository_IngestLiveChatPageRejectsStaleCursorWithoutMutation(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-cursor"
	ingestedAt := time.Date(2026, 8, 14, 14, 0, 0, 0, time.UTC)
	first := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "first", ingestedAt)

	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{first}, ingestedAt))

	second := liveChatHistoryDoc(liveChatID, "message-2", "author-2", "second", ingestedAt.Add(time.Minute))
	err := controller.IngestLiveChatPage(ctx, liveChatID, "stale-cursor", "cursor-2", []repository.LiveChatHistoryDoc{second}, ingestedAt.Add(time.Minute))
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatStreamCursorConflict)

	state := readLiveChatStreamState(t, controller, liveChatID)
	assert.Equal(t, "cursor-1", state.NextPageToken)
	assert.EqualValues(t, 1, state.NextSequence)

	secondKey, keyErr := repository.LiveChatMessageKey(liveChatID, second.ID)
	require.NoError(t, keyErr)
	_, err = controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(secondKey).Get(ctx)
	require.Error(t, err)
}

func TestFirestoreRepository_IngestLiveChatPageDetectsBrokenInboxHistoryPair(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-corrupt"
	ingestedAt := time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "hello", ingestedAt)

	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{message}, ingestedAt))
	key, err := repository.LiveChatMessageKey(liveChatID, message.ID)
	require.NoError(t, err)
	_, err = controller.FirestoreClient().Collection(repository.LiveChatHistory).Doc(key).Delete(ctx)
	require.NoError(t, err)

	err = controller.IngestLiveChatPage(ctx, liveChatID, "cursor-1", "cursor-2", []repository.LiveChatHistoryDoc{message}, ingestedAt.Add(time.Minute))
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatIngestCorruptState)

	state := readLiveChatStreamState(t, controller, liveChatID)
	assert.Equal(t, "cursor-1", state.NextPageToken)
	assert.EqualValues(t, 1, state.NextSequence)
}

func TestFirestoreRepository_IngestLiveChatPageDeduplicatesSameMessageWithinPage(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-page-duplicate"
	ingestedAt := time.Date(2026, 8, 14, 16, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "hello", ingestedAt)

	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"",
		"cursor-1",
		[]repository.LiveChatHistoryDoc{message, message},
		ingestedAt,
	))

	state := readLiveChatStreamState(t, controller, liveChatID)
	assert.EqualValues(t, 1, state.NextSequence)
	assertDurableLiveChatMessage(t, controller, message, 0, ingestedAt)
}

func TestFirestoreRepository_IngestLiveChatPageRejectsOversizedPageBeforeWrites(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-too-large"
	ingestedAt := time.Date(2026, 8, 14, 17, 0, 0, 0, time.UTC)
	messages := make([]repository.LiveChatHistoryDoc, 0, repository.MaxAtomicLiveChatIngestMessages+1)
	for i := 0; i <= repository.MaxAtomicLiveChatIngestMessages; i++ {
		messages = append(messages, liveChatHistoryDoc(
			liveChatID,
			fmt.Sprintf("message-%03d", i),
			"author",
			"hello",
			ingestedAt,
		))
	}

	err := controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", messages, ingestedAt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum atomic ingest size")

	streamKey, keyErr := repository.LiveChatStreamKey(liveChatID)
	require.NoError(t, keyErr)
	_, getErr := controller.FirestoreClient().Collection(repository.LiveChatStreamState).Doc(streamKey).Get(ctx)
	require.Error(t, getErr)
}

func assertDurableLiveChatMessage(
	t *testing.T,
	controller *repository.FirestoreControllerImplements,
	message repository.LiveChatHistoryDoc,
	wantSequence int64,
	wantIngestedAt time.Time,
) {
	t.Helper()
	ctx := context.Background()
	key, err := repository.LiveChatMessageKey(message.LiveChatID, message.ID)
	require.NoError(t, err)

	inboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(key).Get(ctx)
	require.NoError(t, err)
	var inbox repository.LiveChatInboxDoc
	require.NoError(t, inboxSnapshot.DataTo(&inbox))
	assert.Equal(t, message.LiveChatID, inbox.LiveChatID)
	assert.Equal(t, message.ID, inbox.MessageID)
	assert.Equal(t, wantSequence, inbox.Sequence)
	assert.Equal(t, repository.LiveChatInboxPending, inbox.Status)
	assert.Equal(t, message.AuthorChannelID, inbox.AuthorChannelID)
	assert.Equal(t, message.MessageText, inbox.MessageText)
	assert.Equal(t, message.PublishedAt, inbox.PublishedAt)
	assert.Equal(t, wantIngestedAt, inbox.IngestedAt)
	assert.Zero(t, inbox.AttemptCount)
	assert.Empty(t, inbox.LastError)
	assert.Nil(t, inbox.ProcessedAt)
	assert.Nil(t, inbox.LeaseUntil)

	historySnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatHistory).Doc(key).Get(ctx)
	require.NoError(t, err)
	var history repository.LiveChatHistoryDoc
	require.NoError(t, historySnapshot.DataTo(&history))
	assert.Equal(t, message, history)
}

func readLiveChatStreamState(
	t *testing.T,
	controller *repository.FirestoreControllerImplements,
	liveChatID string,
) repository.LiveChatStreamStateDoc {
	t.Helper()
	streamKey, err := repository.LiveChatStreamKey(liveChatID)
	require.NoError(t, err)
	snapshot, err := controller.FirestoreClient().Collection(repository.LiveChatStreamState).Doc(streamKey).Get(context.Background())
	require.NoError(t, err)
	var state repository.LiveChatStreamStateDoc
	require.NoError(t, snapshot.DataTo(&state))
	return state
}

func TestLiveChatIngestErrorsAreSentinels(t *testing.T) {
	t.Parallel()
	assert.True(t, errors.Is(fmt.Errorf("wrapped: %w", repository.ErrLiveChatStreamCursorConflict), repository.ErrLiveChatStreamCursorConflict))
	assert.True(t, errors.Is(fmt.Errorf("wrapped: %w", repository.ErrLiveChatIngestCorruptState), repository.ErrLiveChatIngestCorruptState))
}
