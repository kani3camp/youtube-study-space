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

func TestFirestoreRepository_LiveChatProcessingCutoverStartsAtZeroWithoutShadowState(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 2, 0, 0, 0, time.UTC)

	cutover, err := controller.EnsureLiveChatProcessingCutover(ctx, "cutover-empty-chat", now)
	require.NoError(t, err)
	assert.Equal(t, int64(0), cutover.ProcessFromSequence)
	assert.Equal(t, "cutover-empty-chat", cutover.LiveChatID)
	assert.True(t, cutover.InitializedAt.Equal(now))
}

func TestFirestoreRepository_LiveChatProcessingCutoverSkipsShadowBacklogAndIsImmutable(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "cutover-shadow-chat"
	baseTime := time.Date(2026, 8, 15, 2, 10, 0, 0, time.UTC)

	shadowMessages := []repository.LiveChatHistoryDoc{
		liveChatHistoryDoc(liveChatID, "shadow-1", "author-1", "legacy one", baseTime),
		liveChatHistoryDoc(liveChatID, "shadow-2", "author-2", "legacy two", baseTime.Add(time.Second)),
	}
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"",
		"cursor-shadow",
		shadowMessages,
		baseTime.Add(2*time.Second),
	))

	cutoverAt := baseTime.Add(3 * time.Second)
	cutover, err := controller.EnsureLiveChatProcessingCutover(ctx, liveChatID, cutoverAt)
	require.NoError(t, err)
	assert.Equal(t, int64(2), cutover.ProcessFromSequence)

	postCutoverMessage := liveChatHistoryDoc(
		liveChatID,
		"durable-1",
		"author-3",
		"!info",
		baseTime.Add(4*time.Second),
	)
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"cursor-shadow",
		"cursor-durable",
		[]repository.LiveChatHistoryDoc{postCutoverMessage},
		baseTime.Add(5*time.Second),
	))

	claimable, err := controller.ListClaimableLiveChatInboxMessagesFromSequence(
		ctx,
		liveChatID,
		cutover.ProcessFromSequence,
		baseTime.Add(6*time.Second),
		10,
	)
	require.NoError(t, err)
	require.Len(t, claimable, 1)
	assert.Equal(t, "durable-1", claimable[0].MessageID)
	assert.Equal(t, int64(2), claimable[0].Sequence)

	// The cutover boundary is immutable even after StreamState advances.
	again, err := controller.EnsureLiveChatProcessingCutover(ctx, liveChatID, baseTime.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, cutover.ProcessFromSequence, again.ProcessFromSequence)
	assert.True(t, again.InitializedAt.Equal(cutoverAt))
}

func TestFirestoreRepository_LiveChatProcessingCutoverQueryReclaimsOnlyPostCutoverExpiredLease(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "cutover-expired-lease-chat"
	baseTime := time.Date(2026, 8, 15, 2, 20, 0, 0, time.UTC)

	shadow := liveChatHistoryDoc(liveChatID, "shadow-1", "author-1", "legacy", baseTime)
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"",
		"cursor-shadow",
		[]repository.LiveChatHistoryDoc{shadow},
		baseTime.Add(time.Second),
	))
	cutover, err := controller.EnsureLiveChatProcessingCutover(ctx, liveChatID, baseTime.Add(2*time.Second))
	require.NoError(t, err)
	assert.Equal(t, int64(1), cutover.ProcessFromSequence)

	postCutover := liveChatHistoryDoc(liveChatID, "durable-1", "author-2", "!info", baseTime.Add(3*time.Second))
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"cursor-shadow",
		"cursor-durable",
		[]repository.LiveChatHistoryDoc{postCutover},
		baseTime.Add(4*time.Second),
	))

	_, err = controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		"durable-1",
		"worker-a",
		baseTime.Add(5*time.Second),
		time.Second,
		3,
	)
	require.NoError(t, err)

	claimable, err := controller.ListClaimableLiveChatInboxMessagesFromSequence(
		ctx,
		liveChatID,
		cutover.ProcessFromSequence,
		baseTime.Add(7*time.Second),
		10,
	)
	require.NoError(t, err)
	require.Len(t, claimable, 1)
	assert.Equal(t, "durable-1", claimable[0].MessageID)
	assert.Equal(t, repository.LiveChatInboxProcessing, claimable[0].Status)
}
