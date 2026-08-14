//go:build integration

package workspaceapp

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/core/workspaceapp/usecase"
	"app.modules/internal/integrationtest"
)

func TestProcessClaimedDurableInboxMessage_MoreExtendsSeatExactlyOnce(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-more-chat"
	userID := "author-more"
	now := time.Date(2026, 8, 15, 22, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  5,
		UserID:                  userID,
		SessionID:               "session-more",
		UserDisplayName:         "More User",
		EnteredAt:               now.Add(-30 * time.Minute),
		Until:                   now.Add(60 * time.Minute),
		State:                   repository.WorkState,
		CurrentStateStartedAt:   now.Add(-30 * time.Minute),
		CurrentStateUntil:       now.Add(60 * time.Minute),
		CurrentSegmentStartedAt: now.Add(-30 * time.Minute),
	}
	require.NoError(t, controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := controller.CreateUser(ctx, tx, userID, repository.UserDoc{RegistrationDate: now.Add(-24 * time.Hour)}); err != nil {
			return err
		}
		return controller.CreateSeat(tx, seat, false)
	}))

	history := durableSourceHistory(
		liveChatID,
		"message-more",
		userID,
		"More User",
		"!more 15",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-more",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-more",
		now.Add(-10*time.Second),
		time.Minute,
		3,
	)
	require.NoError(t, err)

	app := WorkspaceApp{
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{
				YoutubeMembershipEnabled: true,
				MaxWorkTimeMin:           180,
				MaxBreakDurationMin:      60,
			},
			LiveChatBotChannelID: "bot-channel",
		},
		Repository: controller,
		nowFunc:    func() time.Time { return now },
	}

	supported, err := app.CanProcessDurableInboxMessage(claimed)
	require.NoError(t, err)
	assert.True(t, supported)
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-more"))

	updatedSeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	assert.True(t, updatedSeat.Until.Equal(now.Add(75*time.Minute)))
	assert.True(t, updatedSeat.CurrentStateUntil.Equal(now.Add(75*time.Minute)))

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	assert.Equal(t, repository.LiveChatReplyOutboxPending, outbox.Status)
	var expected usecase.Result
	expected.Add(usecase.MoreWorkExtended{AddedMin: 15})
	expected.Add(usecase.MoreSummary{RealtimeEnteredMin: 30, RemainingUntilExitMin: 75})
	assert.Equal(t, presenter.BuildMoreMessage(expected, "More User"), outbox.Message)

	// The same source message cannot extend the seat twice after its atomic
	// domain+reply+Processed commit has succeeded.
	err = app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-more")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxAlreadyProcessed)

	afterRetrySeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	assert.True(t, afterRetrySeat.Until.Equal(now.Add(75*time.Minute)))
}
