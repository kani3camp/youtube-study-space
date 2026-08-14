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

type fakeLiveChatInboxWorkerRepository struct {
	items               []repository.LiveChatInboxDoc
	listErr             error
	claimResult         repository.LiveChatInboxDoc
	claimErr            error
	failStatus          repository.LiveChatInboxStatus
	failErr             error
	listedFromSequence  int64
	listLimit           int
	claimCalls          int
	failCalls           int
	claimedWorker       string
	failedWorker        string
	failedProcessingErr error
}

func (f *fakeLiveChatInboxWorkerRepository) ListClaimableLiveChatInboxMessagesFromSequence(
	_ context.Context,
	_ string,
	processFromSequence int64,
	_ time.Time,
	limit int,
) ([]repository.LiveChatInboxDoc, error) {
	f.listedFromSequence = processFromSequence
	f.listLimit = limit
	return f.items, f.listErr
}

func (f *fakeLiveChatInboxWorkerRepository) ClaimLiveChatInboxMessage(
	_ context.Context,
	_, _, workerID string,
	_ time.Time,
	_ time.Duration,
	_ int,
) (repository.LiveChatInboxDoc, error) {
	f.claimCalls++
	f.claimedWorker = workerID
	return f.claimResult, f.claimErr
}

func (f *fakeLiveChatInboxWorkerRepository) FailLiveChatInboxMessage(
	_ context.Context,
	_, _, workerID string,
	_ time.Time,
	_ int,
	processingErr error,
) (repository.LiveChatInboxStatus, error) {
	f.failCalls++
	f.failedWorker = workerID
	f.failedProcessingErr = processingErr
	return f.failStatus, f.failErr
}

type fakeLiveChatInboxMessageProcessor struct {
	preflightCalls int
	unsupported    bool
	preflightErr   error
	calls          int
	inbox          repository.LiveChatInboxDoc
	workerID       string
	err            error
}

func (f *fakeLiveChatInboxMessageProcessor) CanProcessDurableInboxMessage(repository.LiveChatInboxDoc) (bool, error) {
	f.preflightCalls++
	if f.preflightErr != nil {
		return false, f.preflightErr
	}
	return !f.unsupported, nil
}

func (f *fakeLiveChatInboxMessageProcessor) ProcessClaimedDurableInboxMessage(
	_ context.Context,
	inbox repository.LiveChatInboxDoc,
	workerID string,
) error {
	f.calls++
	f.inbox = inbox
	f.workerID = workerID
	return f.err
}

func testInboxCandidate() repository.LiveChatInboxDoc {
	return repository.LiveChatInboxDoc{
		LiveChatID: "chat-1",
		MessageID:  "message-1",
		Sequence:   12,
		Status:     repository.LiveChatInboxPending,
	}
}

func testInboxWorkerOptions() LiveChatInboxWorkerOptions {
	return LiveChatInboxWorkerOptions{
		ProcessFromSequence: 10,
		LeaseDuration:       30 * time.Second,
		MaxAttempts:         3,
	}
}

func TestLiveChatInboxWorkerNoWork(t *testing.T) {
	repo := &fakeLiveChatInboxWorkerRepository{}
	processor := &fakeLiveChatInboxMessageProcessor{}
	worker := NewLiveChatInboxWorker(repo, processor)
	worker.now = func() time.Time { return time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC) }

	result, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.NoError(t, err)
	assert.False(t, result.DidWork)
	assert.Equal(t, int64(10), repo.listedFromSequence)
	assert.Equal(t, 1, repo.listLimit)
	assert.Zero(t, repo.claimCalls)
	assert.Zero(t, processor.preflightCalls)
	assert.Zero(t, processor.calls)
}

func TestLiveChatInboxWorkerSuccessDelegatesAtomicCompletionToProcessor(t *testing.T) {
	candidate := testInboxCandidate()
	claimed := candidate
	claimed.Status = repository.LiveChatInboxProcessing
	repo := &fakeLiveChatInboxWorkerRepository{
		items:       []repository.LiveChatInboxDoc{candidate},
		claimResult: claimed,
	}
	processor := &fakeLiveChatInboxMessageProcessor{}
	worker := NewLiveChatInboxWorker(repo, processor)
	worker.now = time.Now

	result, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.NoError(t, err)
	assert.True(t, result.DidWork)
	assert.Equal(t, repository.LiveChatInboxProcessed, result.Status)
	assert.Equal(t, 1, processor.preflightCalls)
	assert.Equal(t, 1, repo.claimCalls)
	assert.Zero(t, repo.failCalls)
	assert.Equal(t, 1, processor.calls)
	assert.Equal(t, claimed, processor.inbox)
	assert.Equal(t, "worker-a", processor.workerID)
}

func TestLiveChatInboxWorkerTreatsClaimRaceAsNoWork(t *testing.T) {
	candidate := testInboxCandidate()
	repo := &fakeLiveChatInboxWorkerRepository{
		items:    []repository.LiveChatInboxDoc{candidate},
		claimErr: repository.ErrLiveChatInboxNotClaimable,
	}
	processor := &fakeLiveChatInboxMessageProcessor{}
	worker := NewLiveChatInboxWorker(repo, processor)
	worker.now = time.Now

	result, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.NoError(t, err)
	assert.False(t, result.DidWork)
	assert.Equal(t, 1, processor.preflightCalls)
	assert.Zero(t, processor.calls)
}

func TestLiveChatInboxWorkerRecordsProcessingFailure(t *testing.T) {
	candidate := testInboxCandidate()
	claimed := candidate
	claimed.Status = repository.LiveChatInboxProcessing
	processingErr := errors.New("domain write failed")
	repo := &fakeLiveChatInboxWorkerRepository{
		items:       []repository.LiveChatInboxDoc{candidate},
		claimResult: claimed,
		failStatus:  repository.LiveChatInboxPending,
	}
	processor := &fakeLiveChatInboxMessageProcessor{err: processingErr}
	worker := NewLiveChatInboxWorker(repo, processor)
	worker.now = time.Now

	result, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, processingErr)
	assert.True(t, result.DidWork)
	assert.Equal(t, repository.LiveChatInboxPending, result.Status)
	assert.Equal(t, 1, repo.failCalls)
	assert.ErrorIs(t, repo.failedProcessingErr, processingErr)
	assert.Equal(t, "worker-a", repo.failedWorker)
}

func TestLiveChatInboxWorkerDoesNotClaimUnsupportedCommand(t *testing.T) {
	candidate := testInboxCandidate()
	repo := &fakeLiveChatInboxWorkerRepository{items: []repository.LiveChatInboxDoc{candidate}}
	processor := &fakeLiveChatInboxMessageProcessor{unsupported: true}
	worker := NewLiveChatInboxWorker(repo, processor)
	worker.now = time.Now

	result, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDurableCommandNotSupported)
	assert.False(t, result.DidWork)
	assert.Equal(t, candidate.MessageID, result.MessageID)
	assert.Equal(t, candidate.Sequence, result.Sequence)
	assert.Equal(t, repository.LiveChatInboxPending, result.Status)
	assert.Equal(t, 1, processor.preflightCalls)
	assert.Zero(t, repo.claimCalls, "migration gaps must not consume AttemptCount")
	assert.Zero(t, repo.failCalls)
	assert.Zero(t, processor.calls)
}

func TestLiveChatInboxWorkerSurfacesPreflightErrorBeforeClaim(t *testing.T) {
	candidate := testInboxCandidate()
	preflightErr := errors.New("configs unavailable")
	repo := &fakeLiveChatInboxWorkerRepository{items: []repository.LiveChatInboxDoc{candidate}}
	processor := &fakeLiveChatInboxMessageProcessor{preflightErr: preflightErr}
	worker := NewLiveChatInboxWorker(repo, processor)
	worker.now = time.Now

	_, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, preflightErr)
	assert.Zero(t, repo.claimCalls)
	assert.Zero(t, processor.calls)
}

func TestLiveChatInboxWorkerSurfacesDeadLetterOnExhaustedClaim(t *testing.T) {
	candidate := testInboxCandidate()
	repo := &fakeLiveChatInboxWorkerRepository{
		items:    []repository.LiveChatInboxDoc{candidate},
		claimErr: repository.ErrLiveChatInboxRetryExhausted,
	}
	worker := NewLiveChatInboxWorker(repo, &fakeLiveChatInboxMessageProcessor{})
	worker.now = time.Now

	result, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxRetryExhausted)
	assert.True(t, result.DidWork)
	assert.Equal(t, repository.LiveChatInboxDeadLettered, result.Status)
}

func TestLiveChatInboxWorkerPreservesProcessingAndFailureRecordErrors(t *testing.T) {
	candidate := testInboxCandidate()
	claimed := candidate
	claimed.Status = repository.LiveChatInboxProcessing
	processingErr := errors.New("domain failed")
	recordErr := errors.New("firestore failed")
	repo := &fakeLiveChatInboxWorkerRepository{
		items:       []repository.LiveChatInboxDoc{candidate},
		claimResult: claimed,
		failErr:     recordErr,
	}
	worker := NewLiveChatInboxWorker(repo, &fakeLiveChatInboxMessageProcessor{err: processingErr})
	worker.now = time.Now

	_, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", testInboxWorkerOptions())
	require.Error(t, err)
	assert.ErrorIs(t, err, processingErr)
	assert.ErrorIs(t, err, recordErr)
}

func TestLiveChatInboxWorkerRejectsInvalidOptionsBeforeRepositoryAccess(t *testing.T) {
	repo := &fakeLiveChatInboxWorkerRepository{}
	worker := NewLiveChatInboxWorker(repo, &fakeLiveChatInboxMessageProcessor{})
	worker.now = time.Now

	_, err := worker.ProcessNext(context.Background(), "chat-1", "worker-a", LiveChatInboxWorkerOptions{})
	require.Error(t, err)
	assert.Zero(t, repo.listLimit)
}
