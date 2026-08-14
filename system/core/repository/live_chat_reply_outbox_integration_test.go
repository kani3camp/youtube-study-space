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

func testReplyIntent(now time.Time, sequence int64, sourceMessageID, authorID, message string) repository.LiveChatReplyOutboxDoc {
	return repository.LiveChatReplyOutboxDoc{
		LiveChatID:            "live-chat-reply",
		SourceMessageID:       sourceMessageID,
		SourceAuthorChannelID: authorID,
		IntentSlot:            "primary",
		SourceSequence:        sequence,
		Message:               message,
		Status:                repository.LiveChatReplyOutboxPending,
		CreatedAt:             now,
		AvailableAt:           now,
	}
}

func createReplyIntent(t *testing.T, controller *repository.FirestoreControllerImplements, intent repository.LiveChatReplyOutboxDoc) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return controller.CreateLiveChatReplyIntent(ctx, tx, intent)
	}))
}

func readReplyIntent(t *testing.T, controller *repository.FirestoreControllerImplements, intent repository.LiveChatReplyOutboxDoc) repository.LiveChatReplyOutboxDoc {
	t.Helper()
	key, err := repository.LiveChatReplyOutboxKey(intent.LiveChatID, intent.SourceMessageID, intent.IntentSlot)
	require.NoError(t, err)
	snapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(key).Get(context.Background())
	require.NoError(t, err)
	var stored repository.LiveChatReplyOutboxDoc
	require.NoError(t, snapshot.DataTo(&stored))
	return stored
}

func TestFirestoreRepository_ReplyIntentRollsBackWithDomainTransaction(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC)
	intent := testReplyIntent(now, 1, "message-rollback", "author-1", "reply")
	domainRef := controller.FirestoreClient().Collection("test-domain-effects").Doc("effect-1")

	expectedErr := errors.New("abort domain transaction")
	err := controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := tx.Set(domainRef, map[string]any{"applied": true}); err != nil {
			return err
		}
		if err := controller.CreateLiveChatReplyIntent(ctx, tx, intent); err != nil {
			return err
		}
		return expectedErr
	})
	require.ErrorIs(t, err, expectedErr)

	_, err = domainRef.Get(ctx)
	require.Error(t, err)
	key, keyErr := repository.LiveChatReplyOutboxKey(intent.LiveChatID, intent.SourceMessageID, intent.IntentSlot)
	require.NoError(t, keyErr)
	_, err = controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(key).Get(ctx)
	require.Error(t, err)
}

func TestFirestoreRepository_ReplyOutboxDeliveryLifecycle(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 4, 0, 0, 0, time.UTC)
	intent := testReplyIntent(now, 7, "message-1", "author-1", "hello")
	createReplyIntent(t, controller, intent)

	candidates, err := controller.ListClaimableLiveChatReplies(ctx, intent.LiveChatID, now, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, intent.SourceMessageID, candidates[0].SourceMessageID)

	claimAt := now.Add(time.Minute)
	claimed, err := controller.ClaimLiveChatReply(
		ctx,
		intent.LiveChatID,
		intent.SourceMessageID,
		intent.IntentSlot,
		"worker-a",
		claimAt,
		30*time.Second,
		3,
	)
	require.NoError(t, err)
	assert.Equal(t, repository.LiveChatReplyOutboxDelivering, claimed.Status)
	assert.Equal(t, 1, claimed.AttemptCount)
	assert.Equal(t, "worker-a", claimed.LeaseOwner)

	_, err = controller.ClaimLiveChatReply(
		ctx,
		intent.LiveChatID,
		intent.SourceMessageID,
		intent.IntentSlot,
		"worker-b",
		claimAt.Add(10*time.Second),
		30*time.Second,
		3,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatReplyNotClaimable)

	err = controller.CompleteLiveChatReply(ctx, intent.LiveChatID, intent.SourceMessageID, intent.IntentSlot, "worker-b", claimAt.Add(10*time.Second))
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatReplyLeaseLost)

	failureAt := claimAt.Add(15 * time.Second)
	status, err := controller.FailLiveChatReply(
		ctx,
		intent.LiveChatID,
		intent.SourceMessageID,
		intent.IntentSlot,
		"worker-a",
		failureAt,
		3,
		20*time.Second,
		errors.New("temporary YouTube failure"),
	)
	require.NoError(t, err)
	assert.Equal(t, repository.LiveChatReplyOutboxPending, status)

	stored := readReplyIntent(t, controller, intent)
	assert.Equal(t, 1, stored.AttemptCount)
	assert.Equal(t, failureAt.Add(20*time.Second), stored.AvailableAt)
	assert.Equal(t, "temporary YouTube failure", stored.LastError)

	candidates, err = controller.ListClaimableLiveChatReplies(ctx, intent.LiveChatID, failureAt.Add(19*time.Second), 10)
	require.NoError(t, err)
	assert.Empty(t, candidates)
	candidates, err = controller.ListClaimableLiveChatReplies(ctx, intent.LiveChatID, failureAt.Add(20*time.Second), 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)

	secondClaimAt := failureAt.Add(20 * time.Second)
	claimed, err = controller.ClaimLiveChatReply(
		ctx,
		intent.LiveChatID,
		intent.SourceMessageID,
		intent.IntentSlot,
		"worker-b",
		secondClaimAt,
		time.Minute,
		3,
	)
	require.NoError(t, err)
	assert.Equal(t, 2, claimed.AttemptCount)

	completeAt := secondClaimAt.Add(10 * time.Second)
	require.NoError(t, controller.CompleteLiveChatReply(
		ctx,
		intent.LiveChatID,
		intent.SourceMessageID,
		intent.IntentSlot,
		"worker-b",
		completeAt,
	))
	stored = readReplyIntent(t, controller, intent)
	assert.Equal(t, repository.LiveChatReplyOutboxDelivered, stored.Status)
	require.NotNil(t, stored.DeliveredAt)
	assert.Equal(t, completeAt, *stored.DeliveredAt)
	assert.Empty(t, stored.LeaseOwner)
	assert.Nil(t, stored.LeaseUntil)
	assert.Empty(t, stored.LastError)
}

func TestFirestoreRepository_ReplyOutboxExpiredLeaseCanBeReclaimedAndDeadLettered(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 5, 0, 0, 0, time.UTC)
	intent := testReplyIntent(now, 8, "message-expired", "author-2", "reply")
	createReplyIntent(t, controller, intent)

	claimAt := now.Add(time.Minute)
	_, err := controller.ClaimLiveChatReply(ctx, intent.LiveChatID, intent.SourceMessageID, intent.IntentSlot, "worker-a", claimAt, 10*time.Second, 1)
	require.NoError(t, err)

	expiredAt := claimAt.Add(10 * time.Second)
	candidates, err := controller.ListClaimableLiveChatReplies(ctx, intent.LiveChatID, expiredAt, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, repository.LiveChatReplyOutboxDelivering, candidates[0].Status)

	claimed, err := controller.ClaimLiveChatReply(ctx, intent.LiveChatID, intent.SourceMessageID, intent.IntentSlot, "worker-b", expiredAt, time.Minute, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatReplyRetryExhausted)
	assert.Equal(t, repository.LiveChatReplyOutboxDeadLettered, claimed.Status)

	stored := readReplyIntent(t, controller, intent)
	assert.Equal(t, repository.LiveChatReplyOutboxDeadLettered, stored.Status)
	assert.Equal(t, 1, stored.AttemptCount)
	assert.NotEmpty(t, stored.LastError)
}
