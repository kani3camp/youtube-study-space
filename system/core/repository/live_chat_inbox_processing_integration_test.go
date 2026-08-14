//go:build integration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"
)

func TestFirestoreRepository_LiveChatInboxClaimCompleteLifecycle(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-processing"
	ingestedAt := time.Date(2026, 8, 14, 20, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!in", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{message}, ingestedAt))

	claimAt := ingestedAt.Add(time.Minute)
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-a",
		claimAt,
		30*time.Second,
		3,
	)
	require.NoError(t, err)
	assert.Equal(t, repository.LiveChatInboxProcessing, claimed.Status)
	assert.Equal(t, 1, claimed.AttemptCount)
	assert.Equal(t, "worker-a", claimed.LeaseOwner)
	require.NotNil(t, claimed.LeaseUntil)
	assert.Equal(t, claimAt.Add(30*time.Second), *claimed.LeaseUntil)

	_, err = controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-b",
		claimAt.Add(10*time.Second),
		30*time.Second,
		3,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxNotClaimable)

	err = controller.CompleteLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-b",
		claimAt.Add(15*time.Second),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxLeaseLost)

	completeAt := claimAt.Add(20 * time.Second)
	require.NoError(t, controller.CompleteLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-a",
		completeAt,
	))

	stored := readInboxMessage(t, controller, liveChatID, message.ID)
	assert.Equal(t, repository.LiveChatInboxProcessed, stored.Status)
	assert.Equal(t, 1, stored.AttemptCount)
	assert.Empty(t, stored.LeaseOwner)
	assert.Nil(t, stored.LeaseUntil)
	require.NotNil(t, stored.ProcessedAt)
	assert.Equal(t, completeAt, *stored.ProcessedAt)

	_, err = controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-c",
		completeAt.Add(time.Second),
		30*time.Second,
		3,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxNotClaimable)
}

func TestFirestoreRepository_LiveChatInboxExpiredLeaseCanBeReclaimed(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-reclaim"
	ingestedAt := time.Date(2026, 8, 14, 21, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!out", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{message}, ingestedAt))

	firstClaimAt := ingestedAt.Add(time.Minute)
	_, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, message.ID, "worker-a", firstClaimAt, 10*time.Second, 3)
	require.NoError(t, err)

	candidates, err := controller.ListClaimableLiveChatInboxMessages(ctx, liveChatID, firstClaimAt.Add(5*time.Second), 10)
	require.NoError(t, err)
	assert.Empty(t, candidates)

	reclaimAt := firstClaimAt.Add(10 * time.Second)
	candidates, err = controller.ListClaimableLiveChatInboxMessages(ctx, liveChatID, reclaimAt, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, message.ID, candidates[0].MessageID)
	assert.Equal(t, repository.LiveChatInboxProcessing, candidates[0].Status)

	reclaimed, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, message.ID, "worker-b", reclaimAt, 20*time.Second, 3)
	require.NoError(t, err)
	assert.Equal(t, 2, reclaimed.AttemptCount)
	assert.Equal(t, "worker-b", reclaimed.LeaseOwner)

	err = controller.CompleteLiveChatInboxMessage(ctx, liveChatID, message.ID, "worker-a", reclaimAt.Add(time.Second))
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxLeaseLost)
}

func TestFirestoreRepository_LiveChatInboxFailureRetriesThenDeadLetters(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-retry"
	ingestedAt := time.Date(2026, 8, 14, 22, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!change x", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{message}, ingestedAt))

	firstClaimAt := ingestedAt.Add(time.Minute)
	_, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, message.ID, "worker-a", firstClaimAt, time.Minute, 2)
	require.NoError(t, err)

	failureOne := errors.New("temporary processing failure")
	status, err := controller.FailLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-a",
		firstClaimAt.Add(10*time.Second),
		2,
		failureOne,
	)
	require.NoError(t, err)
	assert.Equal(t, repository.LiveChatInboxPending, status)

	stored := readInboxMessage(t, controller, liveChatID, message.ID)
	assert.Equal(t, repository.LiveChatInboxPending, stored.Status)
	assert.Equal(t, 1, stored.AttemptCount)
	assert.Equal(t, failureOne.Error(), stored.LastError)
	assert.Empty(t, stored.LeaseOwner)
	assert.Nil(t, stored.LeaseUntil)

	secondClaimAt := firstClaimAt.Add(20 * time.Second)
	_, err = controller.ClaimLiveChatInboxMessage(ctx, liveChatID, message.ID, "worker-b", secondClaimAt, time.Minute, 2)
	require.NoError(t, err)

	failureTwo := errors.New("permanent processing failure")
	status, err = controller.FailLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-b",
		secondClaimAt.Add(10*time.Second),
		2,
		failureTwo,
	)
	require.NoError(t, err)
	assert.Equal(t, repository.LiveChatInboxDeadLettered, status)

	stored = readInboxMessage(t, controller, liveChatID, message.ID)
	assert.Equal(t, repository.LiveChatInboxDeadLettered, stored.Status)
	assert.Equal(t, 2, stored.AttemptCount)
	assert.Equal(t, failureTwo.Error(), stored.LastError)
	assert.Empty(t, stored.LeaseOwner)
	assert.Nil(t, stored.LeaseUntil)

	candidates, err := controller.ListClaimableLiveChatInboxMessages(ctx, liveChatID, secondClaimAt.Add(time.Minute), 10)
	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestFirestoreRepository_LiveChatInboxExpiredFinalAttemptDeadLettersOnClaim(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-expired-final"
	ingestedAt := time.Date(2026, 8, 14, 23, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!more", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{message}, ingestedAt))

	claimAt := ingestedAt.Add(time.Minute)
	_, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, message.ID, "worker-a", claimAt, 10*time.Second, 1)
	require.NoError(t, err)

	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		message.ID,
		"worker-b",
		claimAt.Add(10*time.Second),
		10*time.Second,
		1,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxRetryExhausted)
	assert.Equal(t, repository.LiveChatInboxDeadLettered, claimed.Status)
	assert.Equal(t, 1, claimed.AttemptCount)

	stored := readInboxMessage(t, controller, liveChatID, message.ID)
	assert.Equal(t, repository.LiveChatInboxDeadLettered, stored.Status)
	assert.Empty(t, stored.LeaseOwner)
	assert.Nil(t, stored.LeaseUntil)
	assert.NotEmpty(t, stored.LastError)
}

func TestFirestoreRepository_ListClaimableLiveChatInboxMessagesPreservesSequence(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-order"
	ingestedAt := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	messages := []repository.LiveChatHistoryDoc{
		liveChatHistoryDoc(liveChatID, "message-1", "author-1", "one", ingestedAt),
		liveChatHistoryDoc(liveChatID, "message-2", "author-2", "two", ingestedAt.Add(time.Second)),
		liveChatHistoryDoc(liveChatID, "message-3", "author-3", "three", ingestedAt.Add(2*time.Second)),
	}
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", messages, ingestedAt))

	// Make the first message an expired Processing candidate while messages 2-3
	// remain Pending. The merged advisory list should still be sequence ordered.
	claimAt := ingestedAt.Add(time.Minute)
	_, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, messages[0].ID, "worker-a", claimAt, 5*time.Second, 3)
	require.NoError(t, err)

	candidates, err := controller.ListClaimableLiveChatInboxMessages(ctx, liveChatID, claimAt.Add(5*time.Second), 10)
	require.NoError(t, err)
	require.Len(t, candidates, 3)
	assert.Equal(t, []string{"message-1", "message-2", "message-3"}, []string{
		candidates[0].MessageID,
		candidates[1].MessageID,
		candidates[2].MessageID,
	})
}

func readInboxMessage(
	t *testing.T,
	controller *repository.FirestoreControllerImplements,
	liveChatID string,
	messageID string,
) repository.LiveChatInboxDoc {
	t.Helper()
	key, err := repository.LiveChatMessageKey(liveChatID, messageID)
	require.NoError(t, err)
	snapshot, err := controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(key).Get(context.Background())
	require.NoError(t, err)
	var message repository.LiveChatInboxDoc
	require.NoError(t, snapshot.DataTo(&message))
	return message
}
