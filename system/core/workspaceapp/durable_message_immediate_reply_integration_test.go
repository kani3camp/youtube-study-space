//go:build integration

package workspaceapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/internal/integrationtest"
)

func TestProcessClaimedDurableInboxMessage_ValidationReplyCommitsUserOutboxAndProcessed(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-validation-reply-chat"
	now := time.Date(2026, 8, 15, 18, 0, 0, 0, timeutil.JapanLocation())
	history := durableSourceHistory(
		liveChatID,
		"message-validation",
		"author-validation",
		"@Validation User",
		"!more 0",
		now.Add(-time.Minute),
	)
	require.NoError(t, controller.IngestLiveChatPage(
		ctx,
		liveChatID,
		"",
		"cursor-1",
		[]repository.LiveChatHistoryDoc{history},
		now.Add(-30*time.Second),
	))
	claimed, err := controller.ClaimLiveChatInboxMessage(
		ctx,
		liveChatID,
		history.ID,
		"worker-validation",
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

	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-validation"))

	user, err := controller.ReadUser(ctx, nil, history.AuthorChannelID)
	require.NoError(t, err)
	assert.True(t, user.RegistrationDate.Equal(now))

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

	// parseAndValidateMessage is side-effect free. Re-running it after the
	// processor has populated the actor context gives the exact legacy reply the
	// durable outbox must preserve.
	prepared := app.parseAndValidateMessage(history.MessageText, false)
	require.NotEmpty(t, prepared.ImmediateReply)
	assert.Equal(t, prepared.ImmediateReply, outbox.Message)
	assert.Contains(t, outbox.Message, "Validation User")
}
