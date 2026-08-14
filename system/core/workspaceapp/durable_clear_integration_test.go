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

func TestProcessClaimedDurableInboxMessage_ClearWorkCommitsSegmentExactlyOnce(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-clear-chat"
	userID := "author-clear"
	now := time.Date(2026, 8, 16, 2, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  8,
		UserID:                  userID,
		SessionID:               "session-clear",
		UserDisplayName:         "Clear User",
		WorkName:                "Writing",
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
		"message-clear",
		userID,
		"Clear User",
		"!clear",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-clear",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-clear",
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
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-clear"))

	updatedSeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	assert.Empty(t, updatedSeat.WorkName)
	assert.True(t, updatedSeat.CurrentSegmentStartedAt.Equal(now))

	segments, err := controller.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	assert.Equal(t, "Writing", segments[0].WorkName)
	assert.Equal(t, repository.WorkState, segments[0].SegmentType)
	assert.True(t, segments[0].StartedAt.Equal(now.Add(-10*time.Minute)))
	assert.True(t, segments[0].EndedAt.Equal(now))
	assert.Equal(t, 10*60, segments[0].DurationSec)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	var expected usecase.Result
	expected.Add(usecase.ClearWork{SeatID: seat.SeatID, IsMemberSeat: false})
	assert.Equal(t, presenter.BuildClearMessage(expected, "Clear User"), outbox.Message)

	// The WorkSegment log and Seat reset must not be duplicated on a replay.
	err = app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-clear")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxAlreadyProcessed)
	segmentsAfterRetry, err := controller.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID)
	require.NoError(t, err)
	assert.Len(t, segmentsAfterRetry, 1)
}
