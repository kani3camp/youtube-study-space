//go:build integration

package workspaceapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/internal/integrationtest"
)

func durableSourceHistory(liveChatID, messageID, authorID, displayName, text string, publishedAt time.Time) repository.LiveChatHistoryDoc {
	return repository.LiveChatHistoryDoc{
		AuthorChannelID:       authorID,
		AuthorDisplayName:     displayName,
		AuthorProfileImageURL: "https://example.com/profile.png",
		AuthorIsChatModerator: false,
		ID:                    messageID,
		LiveChatID:            liveChatID,
		MessageText:           text,
		PublishedAt:           publishedAt,
		Type:                  "textMessageEvent",
	}
}

func TestProcessClaimedDurableInboxMessage_FirstUseInfoCommitsUserReplyAndProcessed(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-info-chat"
	now := time.Date(2026, 8, 15, 15, 0, 0, 0, timeutil.JapanLocation())
	history := durableSourceHistory(liveChatID, "message-info", "author-1", "@New User", "!info", now.Add(-time.Minute))
	source := repository.LiveChatIngestMessage{
		History:             history,
		AuthorIsChatOwner:   true,
		AuthorIsChatSponsor: false,
	}
	require.NoError(t, controller.IngestLiveChatSourcePage(
		ctx,
		liveChatID,
		"",
		"cursor-1",
		[]repository.LiveChatIngestMessage{source},
		now.Add(-30*time.Second),
	))

	claimed, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, history.ID, "worker-a", now.Add(-10*time.Second), time.Minute, 3)
	require.NoError(t, err)
	assert.Equal(t, "New User", claimed.AuthorDisplayName)
	assert.True(t, claimed.AuthorIsChatOwner)
	assert.True(t, claimed.AuthorIsChatMember)

	app := WorkspaceApp{
		Configs: &Configs{
			Constants:            repository.ConstantsConfigDoc{YoutubeMembershipEnabled: true},
			LiveChatBotChannelID: "bot-channel",
		},
		Repository: controller,
		nowFunc:    func() time.Time { return now },
	}

	// No LiveChatBot is configured. Any accidental external reply would fail the
	// test; durable processing must only create an Outbox intent.
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-a"))
	assert.True(t, app.ProcessedUserIsModeratorOrOwner)
	assert.True(t, app.ProcessedUserIsMember)

	user, err := controller.ReadUser(ctx, nil, history.AuthorChannelID)
	require.NoError(t, err)
	assert.True(t, user.RegistrationDate.Equal(now), "Firestore normalizes timestamps to UTC")
	assert.Zero(t, user.DailyTotalStudySec)
	assert.Zero(t, user.TotalStudySec)

	messageKey, err := repository.LiveChatMessageKey(liveChatID, history.ID)
	require.NoError(t, err)
	inboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(messageKey).Get(ctx)
	require.NoError(t, err)
	var inbox repository.LiveChatInboxDoc
	require.NoError(t, inboxSnapshot.DataTo(&inbox))
	assert.Equal(t, repository.LiveChatInboxProcessed, inbox.Status)
	require.NotNil(t, inbox.ProcessedAt)
	assert.True(t, inbox.ProcessedAt.Equal(now), "Firestore normalizes timestamps to UTC")
	assert.Empty(t, inbox.LeaseOwner)
	assert.Nil(t, inbox.LeaseUntil)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	outboxSnapshot, err := controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.NoError(t, err)
	var outbox repository.LiveChatReplyOutboxDoc
	require.NoError(t, outboxSnapshot.DataTo(&outbox))
	assert.Equal(t, repository.LiveChatReplyOutboxPending, outbox.Status)
	assert.Equal(t, history.AuthorChannelID, outbox.SourceAuthorChannelID)
	assert.Equal(t, i18nmsg.CommandUserInfoBase(
		"New User",
		timeutil.DurationToString(0),
		timeutil.DurationToString(0),
	), outbox.Message)

	// The Inbox itself is the effect ledger: a retry cannot recreate the User or
	// reply intent after a successful commit.
	err = app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-a")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrLiveChatInboxAlreadyProcessed)
}

func TestProcessClaimedDurableInboxMessage_NonCommandCreatesUserWithoutReply(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-non-command-chat"
	now := time.Date(2026, 8, 15, 16, 0, 0, 0, timeutil.JapanLocation())
	history := durableSourceHistory(liveChatID, "message-text", "author-2", "Viewer", "hello study room", now.Add(-time.Minute))
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{history}, now.Add(-30*time.Second)))
	claimed, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, history.ID, "worker-a", now.Add(-10*time.Second), time.Minute, 3)
	require.NoError(t, err)

	app := WorkspaceApp{
		Configs:    &Configs{Constants: repository.ConstantsConfigDoc{YoutubeMembershipEnabled: true}},
		Repository: controller,
		nowFunc:    func() time.Time { return now },
	}
	require.NoError(t, app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-a"))
	_, err = controller.ReadUser(ctx, nil, history.AuthorChannelID)
	require.NoError(t, err)

	outboxKey, err := repository.LiveChatReplyOutboxKey(liveChatID, history.ID, durablePrimaryReplyIntentSlot)
	require.NoError(t, err)
	_, err = controller.FirestoreClient().Collection(repository.LiveChatReplyOutbox).Doc(outboxKey).Get(ctx)
	require.Error(t, err)
}

func TestProcessClaimedDurableInboxMessage_UnsupportedCommandDoesNotCommit(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	liveChatID := "durable-unsupported-chat"
	now := time.Date(2026, 8, 15, 17, 0, 0, 0, timeutil.JapanLocation())
	history := durableSourceHistory(liveChatID, "message-out", "author-3", "Viewer", "!out", now.Add(-time.Minute))
	require.NoError(t, controller.IngestLiveChatPage(ctx, liveChatID, "", "cursor-1", []repository.LiveChatHistoryDoc{history}, now.Add(-30*time.Second)))
	claimed, err := controller.ClaimLiveChatInboxMessage(ctx, liveChatID, history.ID, "worker-a", now.Add(-10*time.Second), time.Minute, 3)
	require.NoError(t, err)

	app := WorkspaceApp{
		Configs:    &Configs{Constants: repository.ConstantsConfigDoc{YoutubeMembershipEnabled: true}},
		Repository: controller,
		nowFunc:    func() time.Time { return now },
	}
	err = app.ProcessClaimedDurableInboxMessage(ctx, claimed, "worker-a")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDurableCommandNotSupported)

	messageKey, err := repository.LiveChatMessageKey(liveChatID, history.ID)
	require.NoError(t, err)
	snapshot, err := controller.FirestoreClient().Collection(repository.LiveChatInbox).Doc(messageKey).Get(ctx)
	require.NoError(t, err)
	var inbox repository.LiveChatInboxDoc
	require.NoError(t, snapshot.DataTo(&inbox))
	assert.Equal(t, repository.LiveChatInboxProcessing, inbox.Status)
	assert.Equal(t, "worker-a", inbox.LeaseOwner)

	_, err = controller.ReadUser(ctx, nil, history.AuthorChannelID)
	require.Error(t, err)
}
