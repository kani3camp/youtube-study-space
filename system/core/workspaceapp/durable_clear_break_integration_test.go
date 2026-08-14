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

func TestProcessClaimedDurableInboxMessage_ClearBreakCommitsSegment(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-clear-break-chat"
	userID := "author-clear-break"
	now := time.Date(2026, 8, 16, 3, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  9,
		UserID:                  userID,
		SessionID:               "session-clear-break",
		UserDisplayName:         "Clear Break User",
		WorkName:                "Coding",
		BreakWorkName:           "Coffee",
		EnteredAt:               now.Add(-40 * time.Minute),
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
		"message-clear-break",
		userID,
		"Clear Break User",
		"!clear",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-clear-break",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-clear-break",
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
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-clear-break"))

	updatedSeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	assert.Equal(t, "Coding", updatedSeat.WorkName)
	assert.Empty(t, updatedSeat.BreakWorkName)
	assert.True(t, updatedSeat.CurrentSegmentStartedAt.Equal(now))

	segments, err := controller.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	assert.Equal(t, "Coffee", segments[0].WorkName)
	assert.Equal(t, repository.BreakState, segments[0].SegmentType)
	assert.Equal(t, 5*60, segments[0].DurationSec)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	var expected usecase.Result
	expected.Add(usecase.ClearBreak{SeatID: seat.SeatID, IsMemberSeat: false})
	assert.Equal(t, presenter.BuildClearMessage(expected, "Clear Break User"), outbox.Message)
}
