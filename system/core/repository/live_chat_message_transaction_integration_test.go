//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"
)

func TestFirestoreRepository_LiveChatMessageTransactionCommitsDomainReplyAndInboxTogether(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-reply"
	ingestedAt := time.Date(2026, 8, 15, 7, 0, 0, 0, time.UTC)
	source := liveChatHistoryDoc(liveChatID, "message-atomic", "author-1", "!in", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{source}, ingestedAt))

	claimAt := ingestedAt.Add(time.Minute)
	claimed, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, source.ID, "worker-a", claimAt, time.Minute, 3)
	require.NoError(t, err)
	markerRef := controller.FirestoreClient().Collection("test-domain-effects").Doc("atomic-effect")
	_, err = markerRef.Set(ctx, map[string]any{"count": int64(0)})
	require.NoError(t, err)

	finalizeAt := claimAt.Add(10 * time.Second)
	intent := repository.LiveChatReplyOutboxDoc{
		LiveChatID:            liveChatID,
		SourceMessageID:       source.ID,
		SourceAuthorChannelID: source.AuthorChannelID,
		IntentSlot:            "primary",
		SourceSequence:        claimed.Sequence,
		Message:               "welcome",
		Status:                repository.LiveChatReplyOutboxPending,
		CreatedAt:             finalizeAt,
		AvailableAt:           finalizeAt,
	}

	require.NoError(t, controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		guard, err := controller.BeginLiveChatMessageTransaction(tx, liveChatID, source.ID, "worker-a", finalizeAt)
		if err != nil {
			return err
		}
		marker, err := tx.Get(markerRef)
		if err != nil {
			return err
		}
		count, err := marker.DataAt("count")
		if err != nil {
			return err
		}
		if err := tx.Set(markerRef, map[string]any{"count": count.(int64) + 1}); err != nil {
			return err
		}
		return controller.FinalizeLiveChatMessageTransaction(ctx, tx, guard, []repository.LiveChatReplyOutboxDoc{intent}, finalizeAt)
	}))

	marker, err := markerRef.Get(ctx)
	require.NoError(t, err)
	count, err := marker.DataAt("count")
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	inbox := readInboxMessage(t, controller, liveChatID, source.ID)
	assert.Equal(t, repository.LiveChatInboxProcessed, inbox.Status)
	require.NotNil(t, inbox.ProcessedAt)
	assert.Equal(t, finalizeAt, *inbox.ProcessedAt)
	assert.Empty(t, inbox.LeaseOwner)
	assert.Nil(t, inbox.LeaseUntil)

	outbox := readReplyIntent(t, controller, intent)
	assert.Equal(t, repository.LiveChatReplyOutboxPending, outbox.Status)
	assert.Equal(t, "welcome", outbox.Message)

	// Unknown commit outcome retry: the Inbox itself is the effect ledger. A
	// second execution cannot enter the domain mutation path after Processed.
	err = controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		_, err := controller.BeginLiveChatMessageTransaction(tx, liveChatID, source.ID, "worker-a", finalizeAt.Add(time.Second))
		return err
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxAlreadyProcessed)
	marker, err = markerRef.Get(ctx)
	require.NoError(t, err)
	count, err = marker.DataAt("count")
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestFirestoreRepository_LiveChatMessageTransactionRollsBackAllThreeParts(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-reply"
	ingestedAt := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	source := liveChatHistoryDoc(liveChatID, "message-rollback-finalize", "author-2", "!out", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{source}, ingestedAt))

	claimAt := ingestedAt.Add(time.Minute)
	claimed, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, source.ID, "worker-a", claimAt, time.Minute, 3)
	require.NoError(t, err)
	markerRef := controller.FirestoreClient().Collection("test-domain-effects").Doc("rollback-effect")
	_, err = markerRef.Set(ctx, map[string]any{"count": int64(0)})
	require.NoError(t, err)
	finalizeAt := claimAt.Add(10 * time.Second)
	intent := repository.LiveChatReplyOutboxDoc{
		LiveChatID:            liveChatID,
		SourceMessageID:       source.ID,
		SourceAuthorChannelID: source.AuthorChannelID,
		IntentSlot:            "primary",
		SourceSequence:        claimed.Sequence,
		Message:               "bye",
		Status:                repository.LiveChatReplyOutboxPending,
		CreatedAt:             finalizeAt,
		AvailableAt:           finalizeAt,
	}

	expectedErr := errors.New("abort after finalization was staged")
	err = controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		guard, err := controller.BeginLiveChatMessageTransaction(tx, liveChatID, source.ID, "worker-a", finalizeAt)
		if err != nil {
			return err
		}
		marker, err := tx.Get(markerRef)
		if err != nil {
			return err
		}
		count, err := marker.DataAt("count")
		if err != nil {
			return err
		}
		if err := tx.Set(markerRef, map[string]any{"count": count.(int64) + 1}); err != nil {
			return err
		}
		if err := controller.FinalizeLiveChatMessageTransaction(ctx, tx, guard, []repository.LiveChatReplyOutboxDoc{intent}, finalizeAt); err != nil {
			return err
		}
		return expectedErr
	})
	require.ErrorIs(t, err, expectedErr)

	marker, err := markerRef.Get(ctx)
	require.NoError(t, err)
	count, err := marker.DataAt("count")
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)

	inbox := readInboxMessage(t, controller, liveChatID, source.ID)
	assert.Equal(t, repository.LiveChatInboxProcessing, inbox.Status)
	assert.Equal(t, "worker-a", inbox.LeaseOwner)
	assert.Nil(t, inbox.ProcessedAt)

	key, keyErr := repository.LiveChatReplyOutboxKey(liveChatID, source.ID, intent.IntentSlot)
	require.NoError(t, keyErr)
	_, err = controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(key).Get(ctx)
	require.Error(t, err)
}

func TestFirestoreRepository_LiveChatMessageTransactionRejectsMismatchedReplyIntent(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-reply"
	ingestedAt := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	source := liveChatHistoryDoc(liveChatID, "message-mismatch", "author-3", "!info", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{source}, ingestedAt))
	claimAt := ingestedAt.Add(time.Minute)
	claimed, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, source.ID, "worker-a", claimAt, time.Minute, 3)
	require.NoError(t, err)

	badIntent := repository.LiveChatReplyOutboxDoc{
		LiveChatID:            liveChatID,
		SourceMessageID:       "different-message",
		SourceAuthorChannelID: source.AuthorChannelID,
		IntentSlot:            "primary",
		SourceSequence:        claimed.Sequence,
		Message:               "bad",
		Status:                repository.LiveChatReplyOutboxPending,
		CreatedAt:             claimAt,
		AvailableAt:           claimAt,
	}

	err = controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		guard, err := controller.BeginLiveChatMessageTransaction(tx, liveChatID, source.ID, "worker-a", claimAt)
		if err != nil {
			return err
		}
		return controller.FinalizeLiveChatMessageTransaction(ctx, tx, guard, []repository.LiveChatReplyOutboxDoc{badIntent}, claimAt)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match source Inbox message")

	inbox := readInboxMessage(t, controller, liveChatID, source.ID)
	assert.Equal(t, repository.LiveChatInboxProcessing, inbox.Status)
}
