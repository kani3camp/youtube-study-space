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

func TestProcessClaimedDurableInboxMessage_ResumeTransitionsExactlyOnce(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-resume-chat"
	userID := "author-resume"
	now := time.Date(2026, 8, 16, 5, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  11,
		UserID:                  userID,
		SessionID:               "session-resume",
		UserDisplayName:         "Resume User",
		WorkName:                "Before Break",
		BreakWorkName:           "Coffee",
		EnteredAt:               now.Add(-40 * time.Minute),
		Until:                   now.Add(60 * time.Minute),
		State:                   repository.BreakState,
		CurrentStateStartedAt:   now.Add(-5 * time.Minute),
		CurrentStateUntil:       now.Add(10 * time.Minute),
		CurrentSegmentStartedAt: now.Add(-5 * time.Minute),
		CumulativeWorkSec:       35 * 60,
		DailyCumulativeWorkSec:  35 * 60,
	}
	require.NoError(t, controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := controller.CreateUser(ctx, tx, userID, repository.UserDoc{RegistrationDate: now.Add(-24 * time.Hour)}); err != nil {
			return err
		}
		return controller.CreateSeat(tx, seat, false)
	}))

	history := durableSourceHistory(
		liveChatID,
		"message-resume",
		userID,
		"Resume User",
		"!resume work=Focus",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-resume",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-resume",
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
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-resume"))

	updatedSeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	assert.Equal(t, repository.WorkState, updatedSeat.State)
	assert.Equal(t, "Focus", updatedSeat.WorkName)
	assert.True(t, updatedSeat.CurrentStateStartedAt.Equal(now))
	assert.True(t, updatedSeat.CurrentStateUntil.Equal(seat.Until))
	assert.True(t, updatedSeat.CurrentSegmentStartedAt.Equal(now))
	assert.Equal(t, seat.CumulativeWorkSec, updatedSeat.CumulativeWorkSec)
	assert.Equal(t, seat.DailyCumulativeWorkSec, updatedSeat.DailyCumulativeWorkSec)

	// Read the Break segment explicitly because the repository convenience
	// method intentionally returns Work-state segments only.
	segmentDocs, err := controller.FirestoreClient().Collection(repository.WorkSegments).
		Where(repository.SessionIDDocProperty, "==", seat.SessionID).
		Where(repository.SegmentTypeDocProperty, "==", repository.BreakState).
		Documents(ctx).
		GetAll()
	require.NoError(t, err)
	require.Len(t, segmentDocs, 1)
	var segment repository.WorkSegmentDoc
	require.NoError(t, segmentDocs[0].DataTo(&segment))
	assert.Equal(t, "Coffee", segment.WorkName)
	assert.Equal(t, 5*60, segment.DurationSec)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	var expected usecase.Result
	expected.Add(usecase.ResumeStarted{
		SeatID:                seat.SeatID,
		IsMemberSeat:          false,
		RemainingUntilExitMin: 60,
	})
	assert.Equal(t, presenter.BuildResumeMessage(expected, "Resume User"), outbox.Message)

	// Replaying the same source message must not create another Break segment or
	// reapply the Break->Work transition.
	err = app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-resume")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxAlreadyProcessed)
	segmentDocsAfterRetry, err := controller.FirestoreClient().Collection(repository.WorkSegments).
		Where(repository.SessionIDDocProperty, "==", seat.SessionID).
		Where(repository.SegmentTypeDocProperty, "==", repository.BreakState).
		Documents(ctx).
		GetAll()
	require.NoError(t, err)
	assert.Len(t, segmentDocsAfterRetry, 1)
}
