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
	"app.modules/core/utils"
	"app.modules/internal/integrationtest"
)

func TestProcessClaimedDurableInboxMessage_RankUpdatesSeatedAppearanceAtomically(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-rank-seat-chat"
	userID := "author-rank-seat"
	now := time.Date(2026, 8, 16, 1, 0, 0, 0, timeutil.JapanLocation())
	user := repository.UserDoc{
		RegistrationDate: now.Add(-7 * 24 * time.Hour),
		TotalStudySec:    2 * 60 * 60,
		RankVisible:      false,
		RankPoint:        0,
		FavoriteColor:    "",
	}
	seat := repository.SeatDoc{
		SeatID:                  7,
		UserID:                  userID,
		SessionID:               "session-rank-seat",
		UserDisplayName:         "Rank Seat User",
		EnteredAt:               now.Add(-30 * time.Minute),
		Until:                   now.Add(60 * time.Minute),
		State:                   repository.WorkState,
		CurrentStateStartedAt:   now.Add(-30 * time.Minute),
		CurrentStateUntil:       now.Add(60 * time.Minute),
		CurrentSegmentStartedAt: now.Add(-30 * time.Minute),
	}
	require.NoError(t, controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := controller.CreateUser(ctx, tx, userID, user); err != nil {
			return err
		}
		return controller.CreateSeat(tx, seat, false)
	}))

	history := durableSourceHistory(
		liveChatID,
		"message-rank-seat",
		userID,
		"Rank Seat User",
		"!rank",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-rank-seat",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-rank-seat",
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
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-rank-seat"))

	updatedUser, err := controller.ReadUser(ctx, nil, userID)
	require.NoError(t, err)
	assert.True(t, updatedUser.RankVisible)

	updatedSeat, err := controller.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)
	realtimeSeatDuration, err := utils.RealTimeTotalStudyDurationOfSeat(seat, now)
	require.NoError(t, err)
	expectedAppearance, err := utils.GetSeatAppearance(
		user.TotalStudySec+int(realtimeSeatDuration.Seconds()),
		true,
		user.RankPoint,
		user.FavoriteColor,
	)
	require.NoError(t, err)
	assert.Equal(t, expectedAppearance, updatedSeat.Appearance)
}
