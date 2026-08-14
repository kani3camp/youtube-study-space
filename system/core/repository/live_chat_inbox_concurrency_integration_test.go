//go:build integration

package repository_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"
)

func TestFirestoreRepository_LiveChatInboxConcurrentClaimHasSingleWinner(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "live-chat-concurrent-claim"
	ingestedAt := time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC)
	message := liveChatHistoryDoc(liveChatID, "message-1", "author-1", "!in", ingestedAt)
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{message}, ingestedAt))

	claimAt := ingestedAt.Add(time.Minute)
	workers := []string{"worker-a", "worker-b"}
	type claimResult struct {
		worker  string
		message repository.LiveChatInboxDoc
		err     error
	}
	results := make(chan claimResult, len(workers))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, worker := range workers {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, err := controller.ClaimLiveChatInboxMessage(
				ctx,
				liveChatID,
				message.ID,
				worker,
				claimAt,
				time.Minute,
				3,
			)
			results <- claimResult{worker: worker, message: claimed, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var successes []claimResult
	var rejected []claimResult
	for result := range results {
		if result.err == nil {
			successes = append(successes, result)
			continue
		}
		if assert.ErrorIs(t, result.err, repository.ErrLiveChatInboxNotClaimable) {
			rejected = append(rejected, result)
		}
	}

	require.Len(t, successes, 1)
	require.Len(t, rejected, 1)
	assert.Equal(t, repository.LiveChatInboxProcessing, successes[0].message.Status)
	assert.Equal(t, 1, successes[0].message.AttemptCount)

	stored := readInboxMessage(t, controller, liveChatID, message.ID)
	assert.Equal(t, successes[0].worker, stored.LeaseOwner)
	assert.Equal(t, 1, stored.AttemptCount)
}
