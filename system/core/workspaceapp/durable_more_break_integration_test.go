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

func TestProcessClaimedDurableInboxMessage_MoreExtendsBreakAtomically(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-more-break-chat"
	userID := "author-more-break"
	now := time.Date(2026, 8, 15, 23, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  6,
		UserID:                  userID,
		SessionID:               "session-more-break",
		UserDisplayName:         "Break User",
		EnteredAt:               now.Add(-30 * time.Minute),
		Until:                   now.Add(60 * time.Minute),
		State:                   repository.BreakState,
		CurrentStateStartedAt:   now.Add(-5 * time.Minute),
		CurrentStateUntil:       now.Add(10 * time.Minute),
		CurrentSegmentStartedAt: now.Add(-5 * time.Minute),
	}
	require.NoError(t, controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := controller.CreateUser(ctx, tx, userID, repository.UserDoc{RegistrationDate: now.Add(-24 * time.Hour)}); err != nil {
			return err
		}
		return controller.CreateSeat(tx, seat, false)
	}))

	history := durableSourceHistory(
		liveChatID,
		"message-more-break",
		userID,
		"Break User",
		"!more 15",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-more-break",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-more-break",
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
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-more-break"))

	updatedSeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	assert.True(t, updatedSeat.CurrentStateUntil.Equal(now.Add(25*time.Minute)))
	assert.True(t, updatedSeat.Until.Equal(now.Add(60*time.Minute)))

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	var expected usecase.Result
	expected.Add(usecase.MoreBreakExtended{AddedMin: 15, RemainingBreakMin: 25})
	expected.Add(usecase.MoreSummary{RealtimeEnteredMin: 30, RemainingUntilExitMin: 60})
	assert.Equal(t, presenter.BuildMoreMessage(expected, "Break User"), outbox.Message)
}
