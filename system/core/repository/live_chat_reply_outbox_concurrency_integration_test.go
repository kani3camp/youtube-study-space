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

func TestFirestoreRepository_ReplyOutboxConcurrentClaimHasSingleWinner(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 6, 0, 0, 0, time.UTC)
	intent := testReplyIntent(now, 9, "message-concurrent", "author-1", "reply")
	createReplyIntent(t, controller, intent)

	claimAt := now.Add(time.Minute)
	workers := []string{"worker-a", "worker-b"}
	type claimResult struct {
		worker string
		item   repository.LiveChatReplyOutboxDoc
		err    error
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
			claimed, err := controller.ClaimLiveChatReply(
				ctx,
				intent.LiveChatID,
				intent.SourceMessageID,
				intent.IntentSlot,
				worker,
				claimAt,
				time.Minute,
				3,
			)
			results <- claimResult{worker: worker, item: claimed, err: err}
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
		if assert.ErrorIs(t, result.err, repository.ErrLiveChatReplyNotClaimable) {
			rejected = append(rejected, result)
		}
	}

	require.Len(t, successes, 1)
	require.Len(t, rejected, 1)
	assert.Equal(t, repository.LiveChatReplyOutboxDelivering, successes[0].item.Status)
	assert.Equal(t, 1, successes[0].item.AttemptCount)

	stored := readReplyIntent(t, controller, intent)
	assert.Equal(t, successes[0].worker, stored.LeaseOwner)
	assert.Equal(t, 1, stored.AttemptCount)
}
