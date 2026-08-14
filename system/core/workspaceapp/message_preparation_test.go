package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"app.modules/core/i18n"
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/utils"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
)

func TestPrepareMessage_SelfBotSkipsWithoutRepository(t *testing.T) {
	t.Parallel()

	app := WorkspaceApp{
		Configs: &Configs{LiveChatBotChannelID: "bot-channel"},
	}

	prepared, err := app.prepareMessage(
		context.Background(),
		NGWordConfig{},
		"!info",
		"bot-channel",
		"Bot",
		"",
		false,
		false,
		false,
	)
	require.NoError(t, err)
	assert.True(t, prepared.SkipExecution)
	assert.Empty(t, prepared.ImmediateReply)
	assert.Nil(t, prepared.CommandDetails)
}

func TestPrepareMessage_ReturnsCommandWithoutPostingReply(t *testing.T) {
	ctrl := gomock.NewController(t)
	app := newPreparationTestApp(t, ctrl, true)
	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	app.LiveChatBot = mockLiveChatBot

	prepared, err := app.prepareMessage(
		context.Background(),
		NGWordConfig{},
		"!info",
		"user-1",
		"User One",
		"https://example.com/profile.png",
		true,
		false,
		true,
	)
	require.NoError(t, err)
	assert.False(t, prepared.SkipExecution)
	assert.Empty(t, prepared.ImmediateReply)
	require.NotNil(t, prepared.CommandDetails)
	assert.Equal(t, utils.Info, prepared.CommandDetails.CommandType)
	assert.Equal(t, "user-1", app.ProcessedUserID)
	assert.Equal(t, "User One", app.ProcessedUserDisplayName)
	assert.True(t, app.ProcessedUserIsModeratorOrOwner)
	assert.True(t, app.ProcessedUserIsMember)
}

func TestPrepareMessage_DisabledMembershipNormalizesActor(t *testing.T) {
	ctrl := gomock.NewController(t)
	app := newPreparationTestApp(t, ctrl, false)
	app.LiveChatBot = mock_youtubebot.NewMockLiveChatBot(ctrl)

	prepared, err := app.prepareMessage(
		context.Background(),
		NGWordConfig{},
		"!info",
		"user-1",
		"User One",
		"",
		true,
		false,
		true,
	)
	require.NoError(t, err)
	require.NotNil(t, prepared.CommandDetails)
	assert.Equal(t, utils.Info, prepared.CommandDetails.CommandType)
	assert.False(t, app.ProcessedUserIsMember)
}

func TestPrepareMessage_ReturnsValidationReplyWithoutPosting(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	ctrl := gomock.NewController(t)
	app := newPreparationTestApp(t, ctrl, true)
	app.LiveChatBot = mock_youtubebot.NewMockLiveChatBot(ctrl)

	prepared, err := app.prepareMessage(
		context.Background(),
		NGWordConfig{},
		"!more 0",
		"user-1",
		"User One",
		"",
		true,
		false,
		false,
	)
	require.NoError(t, err)
	assert.True(t, prepared.SkipExecution)
	assert.Nil(t, prepared.CommandDetails)
	assert.Equal(
		t,
		i18nmsg.CommonSir("User One")+i18nmsg.ValidateNonOneOrMoreExtendedTime(),
		prepared.ImmediateReply,
	)
}

func TestProcessMessage_PreservesImmediateReplyDelivery(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	ctrl := gomock.NewController(t)
	app := newPreparationTestApp(t, ctrl, true)
	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	expected := i18nmsg.CommonSir("User One") + i18nmsg.ValidateNonOneOrMoreExtendedTime()
	mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), expected).Return(nil).Times(1)
	app.LiveChatBot = mockLiveChatBot

	err := app.ProcessMessage(
		context.Background(),
		NGWordConfig{},
		"!more 0",
		"user-1",
		"User One",
		"",
		true,
		false,
		false,
	)
	require.NoError(t, err)
}

func TestProcessMessage_PreservesUserInitializationFailureReply(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockFirestoreClient := mock_repository.NewMockDBClient(ctrl)
	failure := errors.New("database unavailable")
	mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).Times(1)
	mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).Return(failure).Times(1)

	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), i18nmsg.CommandError("User One")).Return(nil).Times(1)
	app := WorkspaceApp{
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{YoutubeMembershipEnabled: true},
		},
		Repository:         mockDB,
		LiveChatBot:        mockLiveChatBot,
		alertOwnerBot:      moderatorbot.DummyMessageBot{},
		alertModeratorsBot: moderatorbot.DummyMessageBot{},
		logModeratorsBot:   moderatorbot.DummyMessageBot{},
	}

	err := app.ProcessMessage(
		context.Background(),
		NGWordConfig{},
		"!info",
		"user-1",
		"User One",
		"",
		true,
		false,
		false,
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, failure.Error())
}

func newPreparationTestApp(t *testing.T, ctrl *gomock.Controller, membershipEnabled bool) *WorkspaceApp {
	t.Helper()
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockFirestoreClient := mock_repository.NewMockDBClient(ctrl)
	mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
	mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
			return f(ctx, &firestore.Transaction{})
		},
	).AnyTimes()
	mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), gomock.Any()).Return(repository.UserDoc{}, nil).AnyTimes()

	return &WorkspaceApp{
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{YoutubeMembershipEnabled: membershipEnabled},
		},
		Repository:         mockDB,
		alertOwnerBot:      moderatorbot.DummyMessageBot{},
		alertModeratorsBot: moderatorbot.DummyMessageBot{},
		logModeratorsBot:   moderatorbot.DummyMessageBot{},
	}
}

func loadWorkspaceAppTestI18n(t *testing.T) {
	t.Helper()
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		t.Fatal(fmt.Errorf("load locale folder: %w", err))
	}
}
