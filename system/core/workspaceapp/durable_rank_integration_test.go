//go:build integration

package workspaceapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/internal/integrationtest"
)

func TestProcessClaimedDurableInboxMessage_RankFirstUseTogglesExactlyOnce(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-rank-chat"
	userID := "author-rank"
	now := time.Date(2026, 8, 16, 0, 0, 0, 0, timeutil.JapanLocation())
	history := durableSourceHistory(
		liveChatID,
		"message-rank",
		userID,
		"Rank User",
		"!rank",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-rank",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-rank",
		now.Add(-10*time.Second),
		time.Minute,
		3,
	)
	require.NoError(t, err)

	app := WorkspaceApp{
		Configs: &Configs{
			Constants:            repository.ConstantsConfigDoc{YoutubeMembershipEnabled: true},
			LiveChatBotChannelID: "bot-channel",
		},
		Repository: controller,
		nowFunc:    func() time.Time { return now },
	}

	supported, err := app.CanProcessDurableInboxMessage(claimed)
	require.NoError(t, err)
	assert.True(t, supported)
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-rank"))

	user, err := controller.ReadUser(ctx, nil, userID)
	require.NoError(t, err)
	assert.True(t, user.RankVisible)
	assert.True(t, user.RegistrationDate.Equal(now))

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	assert.Equal(t, i18nmsg.CommandRank("Rank User", i18nmsg.CommonOn()), outbox.Message)

	// A retry after the atomic commit must not toggle the setting back off.
	err = app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-rank")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxAlreadyProcessed)
	userAfterRetry, err := controller.ReadUser(ctx, nil, userID)
	require.NoError(t, err)
	assert.True(t, userAfterRetry.RankVisible)
}
