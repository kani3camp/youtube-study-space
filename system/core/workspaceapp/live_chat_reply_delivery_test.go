package workspaceapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
)

type fakeLiveChatReplyDeliveryRepository struct {
	items          []repository.LiveChatReplyOutboxDoc
	listErr        error
	claimResult    repository.LiveChatReplyOutboxDoc
	claimErr       error
	completeErr    error
	failStatus     repository.LiveChatReplyOutboxStatus
	failErr        error
	listLimit      int
	claimCalls     int
	completeCalls  int
	failCalls      int
	claimedWorker  string
	completeWorker string
	failedWorker   string
}

func (f *fakeLiveChatReplyDeliveryRepository) ListClaimableLiveChatReplies(
	_ context.Context,
	_ string,
	_ time.Time,
	limit int,
) ([]repository.LiveChatReplyOutboxDoc, error) {
	f.listLimit = limit
	return f.items, f.listErr
}

func (f *fakeLiveChatReplyDeliveryRepository) ClaimLiveChatReply(
	_ context.Context,
	_, _, _, workerID string,
	_ time.Time,
	_ time.Duration,
	_ int,
) (repository.LiveChatReplyOutboxDoc, error) {
	f.claimCalls++
	f.claimedWorker = workerID
	return f.claimResult, f.claimErr
}

func (f *fakeLiveChatReplyDeliveryRepository) CompleteLiveChatReply(
	_ context.Context,
	_, _, _, workerID string,
	_ time.Time,
) error {
	f.completeCalls++
	f.completeWorker = workerID
	return f.completeErr
}

func (f *fakeLiveChatReplyDeliveryRepository) FailLiveChatReply(
	_ context.Context,
	_, _, _, workerID string,
	_ time.Time,
	_ int,
	_ time.Duration,
	_ error,
) (repository.LiveChatReplyOutboxStatus, error) {
	f.failCalls++
	f.failedWorker = workerID
	return f.failStatus, f.failErr
}

type fakeLiveChatReplySender struct {
	messages []string
	err      error
}

func (f *fakeLiveChatReplySender) PostMessage(_ context.Context, message string) error {
	f.messages = append(f.messages, message)
	return f.err
}

func testReplyIntent() repository.LiveChatReplyOutboxDoc {
	return repository.LiveChatReplyOutboxDoc{
		LiveChatID:      "chat-1",
		SourceMessageID: "message-1",
		IntentSlot:      "primary",
		SourceSequence:  7,
		Message:         "reply text",
		Status:          repository.LiveChatReplyOutboxPending,
	}
}

func testReplyDeliveryOptions() LiveChatReplyDeliveryOptions {
	return LiveChatReplyDeliveryOptions{
		LeaseDuration: 30 * time.Second,
		RetryDelay:    5 * time.Second,
		MaxAttempts:   3,
	}
}

func TestLiveChatReplyDeliveryWorkerNoWork(t *testing.T) {
	repo := &fakeLiveChatReplyDeliveryRepository{}
	sender := &fakeLiveChatReplySender{}
	worker := NewLiveChatReplyDeliveryWorker(repo, sender)
	worker.now = func() time.Time { return time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC) }

	result, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", testReplyDeliveryOptions())
	require.NoError(t, err)
	assert.False(t, result.DidWork)
	assert.Equal(t, 1, repo.listLimit, "only the head reply should be considered per pass")
	assert.Zero(t, repo.claimCalls)
	assert.Empty(t, sender.messages)
}

func TestLiveChatReplyDeliveryWorkerSuccess(t *testing.T) {
	intent := testReplyIntent()
	claimed := intent
	claimed.Status = repository.LiveChatReplyOutboxDelivering
	repo := &fakeLiveChatReplyDeliveryRepository{
		items:       []repository.LiveChatReplyOutboxDoc{intent},
		claimResult: claimed,
	}
	sender := &fakeLiveChatReplySender{}
	worker := NewLiveChatReplyDeliveryWorker(repo, sender)
	worker.now = func() time.Time { return time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC) }

	result, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", testReplyDeliveryOptions())
	require.NoError(t, err)
	assert.True(t, result.DidWork)
	assert.Equal(t, repository.LiveChatReplyOutboxDelivered, result.Status)
	assert.Equal(t, []string{"reply text"}, sender.messages)
	assert.Equal(t, 1, repo.claimCalls)
	assert.Equal(t, 1, repo.completeCalls)
	assert.Zero(t, repo.failCalls)
	assert.Equal(t, "worker-a", repo.claimedWorker)
	assert.Equal(t, "worker-a", repo.completeWorker)
}

func TestLiveChatReplyDeliveryWorkerRecordsSendFailureAndStops(t *testing.T) {
	intent := testReplyIntent()
	claimed := intent
	claimed.Status = repository.LiveChatReplyOutboxDelivering
	repo := &fakeLiveChatReplyDeliveryRepository{
		items:       []repository.LiveChatReplyOutboxDoc{intent},
		claimResult: claimed,
		failStatus:  repository.LiveChatReplyOutboxPending,
	}
	senderErr := errors.New("youtube unavailable")
	sender := &fakeLiveChatReplySender{err: senderErr}
	worker := NewLiveChatReplyDeliveryWorker(repo, sender)
	worker.now = func() time.Time { return time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC) }

	result, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", testReplyDeliveryOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, senderErr)
	assert.True(t, result.DidWork)
	assert.Equal(t, repository.LiveChatReplyOutboxPending, result.Status)
	assert.Equal(t, 1, repo.failCalls)
	assert.Zero(t, repo.completeCalls)
	assert.Equal(t, "worker-a", repo.failedWorker)
}

func TestLiveChatReplyDeliveryWorkerReturnsBothSendAndFailureRecordErrors(t *testing.T) {
	intent := testReplyIntent()
	claimed := intent
	claimed.Status = repository.LiveChatReplyOutboxDelivering
	senderErr := errors.New("youtube unavailable")
	recordErr := errors.New("firestore unavailable")
	repo := &fakeLiveChatReplyDeliveryRepository{
		items:       []repository.LiveChatReplyOutboxDoc{intent},
		claimResult: claimed,
		failErr:     recordErr,
	}
	sender := &fakeLiveChatReplySender{err: senderErr}
	worker := NewLiveChatReplyDeliveryWorker(repo, sender)
	worker.now = time.Now

	_, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", testReplyDeliveryOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, senderErr)
	assert.ErrorIs(t, err, recordErr)
}

func TestLiveChatReplyDeliveryWorkerTreatsClaimRaceAsNoWork(t *testing.T) {
	intent := testReplyIntent()
	repo := &fakeLiveChatReplyDeliveryRepository{
		items:    []repository.LiveChatReplyOutboxDoc{intent},
		claimErr: repository.ErrLiveChatReplyNotClaimable,
	}
	sender := &fakeLiveChatReplySender{}
	worker := NewLiveChatReplyDeliveryWorker(repo, sender)
	worker.now = time.Now

	result, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", testReplyDeliveryOptions())
	require.NoError(t, err)
	assert.False(t, result.DidWork)
	assert.Empty(t, sender.messages)
}

func TestLiveChatReplyDeliveryWorkerSurfacesDeadLetterOnExhaustedClaim(t *testing.T) {
	intent := testReplyIntent()
	repo := &fakeLiveChatReplyDeliveryRepository{
		items:    []repository.LiveChatReplyOutboxDoc{intent},
		claimErr: repository.ErrLiveChatReplyRetryExhausted,
	}
	worker := NewLiveChatReplyDeliveryWorker(repo, &fakeLiveChatReplySender{})
	worker.now = time.Now

	result, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", testReplyDeliveryOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatReplyRetryExhausted)
	assert.True(t, result.DidWork)
	assert.Equal(t, repository.LiveChatReplyOutboxDeadLettered, result.Status)
}

func TestLiveChatReplyDeliveryWorkerWarnsWhenCompletionFailsAfterPost(t *testing.T) {
	intent := testReplyIntent()
	claimed := intent
	claimed.Status = repository.LiveChatReplyOutboxDelivering
	completeErr := errors.New("commit lost")
	repo := &fakeLiveChatReplyDeliveryRepository{
		items:       []repository.LiveChatReplyOutboxDoc{intent},
		claimResult: claimed,
		completeErr: completeErr,
	}
	worker := NewLiveChatReplyDeliveryWorker(repo, &fakeLiveChatReplySender{})
	worker.now = time.Now

	result, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", testReplyDeliveryOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, completeErr)
	assert.ErrorContains(t, err, "delivery may repeat")
	assert.True(t, result.DidWork)
	assert.Equal(t, repository.LiveChatReplyOutboxDelivering, result.Status)
}

func TestLiveChatReplyDeliveryWorkerRejectsInvalidOptionsBeforeRepositoryAccess(t *testing.T) {
	repo := &fakeLiveChatReplyDeliveryRepository{}
	worker := NewLiveChatReplyDeliveryWorker(repo, &fakeLiveChatReplySender{})
	worker.now = time.Now

	_, err := worker.DeliverNext(context.Background(), "chat-1", "worker-a", LiveChatReplyDeliveryOptions{})
	require.Error(t, err)
	assert.Zero(t, repo.listLimit)
}
