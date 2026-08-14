//go:build integration

package workspaceapp

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp/presenter"
	"app.modules/internal/integrationtest"
)

func TestProcessClaimedDurableInboxMessage_SeatQueryFirstUseCommitsReplyAndProcessed(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-seat-query-chat"
	now := time.Date(2026, 8, 15, 20, 0, 0, 0, timeutil.JapanLocation())
	history := durableSourceHistory(
		liveChatID,
		"message-seat",
		"author-seat",
		"@Seat User",
		"!seat",
		now.Add(-time.Minute),
	)
	source := repository.LiveChatIngestMessage{History: history}
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-seat",
		[]repository.LiveChatIngestMessage{source},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-seat",
		now.Add(-10*time.Second),
		time.Minute,
		3,
	)
	require.NoError(t, err)
	assert.Equal(t, "Seat User", claimed.AuthorDisplayName)

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
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-seat"))

	_, err = controller.ReadUser(ctx, nil, history.AuthorChannelID)
	require.NoError(t, err)

	messageKey, err := repository.LiveChatMessageKey(liveChatID, history.ID)
	require.NoError(t, err)
	inboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(messageKey).Get(ctx)
	require.NoError(t, err)
	var inbox repository.LiveChatInboxDoc
	require.NoError(t, inboxSnapshot.DataTo(&inbox))
	assert.Equal(t, repository.LiveChatInboxProcessed, inbox.Status)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	assert.Equal(t, repository.LiveChatReplyOutboxPending, outbox.Status)
	assert.Equal(t, i18nmsg.CommandNotEnter("Seat User", utils.InCommand), outbox.Message)
}

func TestProcessClaimedDurableInboxMessage_SeatQueryReportsCurrentWorkSeat(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-seat-query-current-chat"
	userID := "author-seated"
	now := time.Date(2026, 8, 15, 21, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  4,
		UserID:                  userID,
		SessionID:               "session-seat-query",
		UserDisplayName:         "Seated User",
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
		"message-current-seat",
		userID,
		"Seated User",
		"!seat",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-current-seat",
		[]repository.LiveChatIngestMessage{{History: history}},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-current-seat",
		now.Add(-10*time.Second),
		time.Minute,
		3,
	)
	require.NoError(t, err)

	app := WorkspaceApp{
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{
				YoutubeMembershipEnabled: true,
				RecentRangeMin:           60,
			},
			LiveChatBotChannelID: "bot-channel",
		},
		Repository: controller,
		nowFunc:    func() time.Time { return now },
	}
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-current-seat"))

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	assert.Equal(t, repository.LiveChatReplyOutboxPending, outbox.Status)
	assert.Equal(t, i18nmsg.CommandSeatInfoBase(
		"Seated User",
		presenter.SeatIDStr(seat.SeatID, false),
		i18nmsg.CommonWork(),
		30,
		30,
		60,
		"",
	), outbox.Message)
}
