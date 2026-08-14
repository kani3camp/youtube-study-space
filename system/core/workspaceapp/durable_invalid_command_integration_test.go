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

func TestProcessClaimedDurableInboxMessage_InvalidCommandCreatesUserAndCompletesWithoutReply(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-invalid-command-chat"
	now := time.Date(2026, 8, 15, 19, 0, 0, 0, timeutil.JapanLocation())
	history := durableSourceHistory(
		liveChatID,
		"message-invalid",
		"author-invalid",
		"Invalid User",
		"!definitely-unknown-command",
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
		"worker-invalid",
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
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-invalid"))

	_, err = controller.ReadUser(ctx, nil, history.AuthorChannelID)
	require.NoError(t, err)

	messageKey, err := repository.LiveChatMessageKey(liveChatID, history.ID)
	require.NoError(t, err)
	snapshot, err := controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(messageKey).Get(ctx)
	require.NoError(t, err)
	var inbox repository.LiveChatInboxDoc
	require.NoError(t, snapshot.DataTo(&inbox))
	assert.Equal(t, repository.LiveChatInboxProcessed, inbox.Status)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	_, err = controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.Error(t, err, "legacy invalid commands have no reply")
}
