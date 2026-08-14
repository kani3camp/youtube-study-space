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

func TestProcessClaimedDurableInboxMessage_BreakTransitionsExactlyOnce(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-break-chat"
	userID := "author-break"
	now := time.Date(2026, 8, 16, 4, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  10,
		UserID:                  userID,
		SessionID:               "session-break",
		UserDisplayName:         "Break User",
		WorkName:                "Coding",
		BreakWorkName:           "Previous Break",
		EnteredAt:               now.Add(-30 * time.Minute),
		Until:                   now.Add(60 * time.Minute),
		State:                   repository.WorkState,
		CurrentStateStartedAt:   now.Add(-30 * time.Minute),
		CurrentStateUntil:       now.Add(60 * time.Minute),
		CurrentSegmentStartedAt: now.Add(-10 * time.Minute),
	}
	require.NoError(t, controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := controller.CreateUser(ctx, tx, userID, repository.UserDoc{RegistrationDate: now.Add(-24 * time.Hour)}); err != nil {
			return err
		}
		return controller.CreateSeat(tx, seat, false)
	}))

	history := durableSourceHistory(
		liveChatID,
		"message-break",
		userID,
		"Break User",
		"!break min=15 work=Coffee",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-break",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-break",
		now.Add(-10*time.Second),
		time.Minute,
		3,
	)
	require.NoError(t, err)

	app := WorkspaceApp{
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{
				YoutubeMembershipEnabled: true,
				MinBreakIntervalMin:      25,
				DefaultBreakDurationMin:  10,
			},
			LiveChatBotChannelID: "bot-channel",
		},
		Repository: controller,
		nowFunc:    func() time.Time { return now },
	}

	supported, err := app.CanProcessDurableInboxMessage(claimed)
	require.NoError(t, err)
	assert.True(t, supported)
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-break"))

	updatedSeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	assert.Equal(t, repository.BreakState, updatedSeat.State)
	assert.Equal(t, "Coffee", updatedSeat.BreakWorkName)
	assert.True(t, updatedSeat.CurrentStateStartedAt.Equal(now))
	assert.True(t, updatedSeat.CurrentStateUntil.Equal(now.Add(15*time.Minute)))
	assert.True(t, updatedSeat.CurrentSegmentStartedAt.Equal(now))
	assert.Equal(t, 30*60, updatedSeat.CumulativeWorkSec)

	segments, err := controller.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	assert.Equal(t, "Coding", segments[0].WorkName)
	assert.Equal(t, repository.WorkState, segments[0].SegmentType)
	assert.Equal(t, 10*60, segments[0].DurationSec)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	var expected usecase.Result
	expected.Add(usecase.BreakStarted{
		SeatID:       seat.SeatID,
		IsMemberSeat: false,
		WorkName:     "Coffee",
		DurationMin:  15,
	})
	assert.Equal(t, presenter.BuildBreakMessage(expected, "Break User"), outbox.Message)

	// The same source message must not create a second WorkSegment or apply the
	// Work->Break transition twice after Processed committed.
	err = app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-break")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxAlreadyProcessed)
	segmentsAfterRetry, err := controller.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID)
	require.NoError(t, err)
	assert.Len(t, segmentsAfterRetry, 1)
}
