package workspaceapp

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
)

func TestBuildUserInfoReplyTx_BuildsReplyWithoutExternalPost(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	tx := &firestore.Transaction{}
	userID := "user-1"
	userDoc := repository.UserDoc{
		DailyTotalStudySec: 60 * 60,
		TotalStudySec:      2 * 60 * 60,
		RankVisible:        true,
		RankPoint:          42,
	}

	// Offline user: realtime duration is zero, so only the persisted totals are
	// reflected in the reply. No LiveChatBot is configured, which also proves
	// this transaction body does not post externally.
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, true).Return(
		repository.SeatDoc{},
		status.Error(codes.NotFound, "not in member room"),
	).Times(1)
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, false).Return(
		repository.SeatDoc{},
		status.Error(codes.NotFound, "not in general room"),
	).Times(1)
	mockDB.EXPECT().ReadUser(gomock.Any(), tx, userID).Return(userDoc, nil).Times(2)

	app := WorkspaceApp{
		Repository:               mockDB,
		ProcessedUserID:          userID,
		ProcessedUserDisplayName: "User One",
	}

	reply, err := app.buildUserInfoReplyTx(context.Background(), tx, &utils.InfoOption{})
	require.NoError(t, err)
	expected := i18nmsg.CommandUserInfoBase(
		"User One",
		timeutil.DurationToString(time.Hour),
		timeutil.DurationToString(2*time.Hour),
	) + i18nmsg.CommandUserInfoRank(42)
	assert.Equal(t, expected, reply)
}

func TestBuildUserInfoReplyTx_PropagatesRealtimeReadFailure(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	tx := &firestore.Transaction{}
	readErr := status.Error(codes.Internal, "seat lookup failed")
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "user-1", true).Return(repository.SeatDoc{}, readErr).Times(1)

	app := WorkspaceApp{Repository: mockDB, ProcessedUserID: "user-1"}
	reply, err := app.buildUserInfoReplyTx(context.Background(), tx, &utils.InfoOption{})
	require.Error(t, err)
	assert.Empty(t, reply)
	assert.ErrorContains(t, err, "seat lookup failed")
}

func TestBuildUserInfoReply_FirstUseDetailsNeedNoRepositoryRead(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	now := time.Date(2026, 8, 15, 11, 30, 0, 0, timeutil.JapanLocation())
	app := WorkspaceApp{
		ProcessedUserDisplayName: "New User",
		nowFunc:                  func() time.Time { return now },
	}
	userDoc := repository.UserDoc{RegistrationDate: now}

	reply := app.buildUserInfoReply(userDoc, 0, 0, &utils.InfoOption{ShowDetails: true})

	expected := i18nmsg.CommandUserInfoBase(
		"New User",
		timeutil.DurationToString(0),
		timeutil.DurationToString(0),
	) + i18nmsg.CommandUserInfoRankOff() +
		i18nmsg.CommandUserInfoDefaultWorkOff() +
		i18nmsg.CommandUserInfoFavoriteColorOff() +
		i18nmsg.CommandUserInfoRegisterDate("2026年08月15日")
	assert.Equal(t, expected, reply)
}
