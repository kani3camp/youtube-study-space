package workspaceapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	mock_myfirestore "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
)

func TestSystem_RankRecomputesCanonicalV2SeatAppearance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  1,
		UserID:                  "test_user_id",
		State:                   repository.WorkState,
		EnteredAt:               fixedNow,
		CurrentStateStartedAt:   fixedNow,
		CurrentSegmentStartedAt: fixedNow,
		Appearance: repository.SeatAppearance{
			SchemaVersion: 2,
			TopBarColor:   utils.ColorHours0To5,
			Rank:          2,
			RankVisible:   false,
			NumStars:      0,
		},
	}
	user := repository.UserDoc{
		TotalStudySec: 3 * 60 * 60,
		RankVisible:   false,
		RankPoint:     15000,
	}

	mockDB := mock_myfirestore.NewMockRepository(ctrl)
	mockFirestoreClient := mock_myfirestore.NewMockDBClient(ctrl)
	mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
			return f(ctx, &firestore.Transaction{})
		},
	)
	mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient)
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", true).
		Return(repository.SeatDoc{}, status.Error(codes.NotFound, "")).AnyTimes()
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", false).
		Return(seat, nil).AnyTimes()
	mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(user, nil).AnyTimes()
	mockDB.EXPECT().UpdateUserRankVisible(gomock.Any(), "test_user_id", true).Return(nil)
	mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), false).
		DoAndReturn(func(_ context.Context, _ *firestore.Transaction, updated repository.SeatDoc, _ bool) error {
			assert.Equal(t, utils.SeatAppearanceSchemaVersion, updated.Appearance.SchemaVersion)
			assert.Equal(t, utils.ColorHours0To5, updated.Appearance.TopBarColor)
			assert.Equal(t, 2, updated.Appearance.Rank)
			assert.True(t, updated.Appearance.RankVisible)
			assert.Equal(t, 0, updated.Appearance.NumStars)
			return nil
		})

	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), "@テストユーザー さんのランク表示をオンにしました🎯").Return(nil)

	app := WorkspaceApp{
		Repository:               mockDB,
		LiveChatBot:              mockLiveChatBot,
		alertOwnerBot:            moderatorbot.DummyMessageBot{},
		ProcessedUserID:          "test_user_id",
		ProcessedUserDisplayName: "テストユーザー",
		nowFunc:                  func() time.Time { return fixedNow },
	}
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, app.Rank(context.Background(), &utils.CommandDetails{CommandType: utils.Rank}))
}

func TestSystem_MyFavoriteColorRecomputesCanonicalV2SeatAppearance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  1,
		UserID:                  "test_user_id",
		State:                   repository.WorkState,
		EnteredAt:               fixedNow,
		CurrentStateStartedAt:   fixedNow,
		CurrentSegmentStartedAt: fixedNow,
		Appearance: repository.SeatAppearance{
			SchemaVersion: 2,
			TopBarColor:   utils.ColorHoursFrom1000,
			Rank:          2,
			RankVisible:   false,
			NumStars:      1,
		},
	}
	user := repository.UserDoc{
		TotalStudySec: 1000 * 60 * 60,
		RankPoint:     15000,
	}

	mockDB := mock_myfirestore.NewMockRepository(ctrl)
	mockFirestoreClient := mock_myfirestore.NewMockDBClient(ctrl)
	mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
			return f(ctx, &firestore.Transaction{})
		},
	)
	mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient)
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", true).
		Return(repository.SeatDoc{}, status.Error(codes.NotFound, "")).AnyTimes()
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", false).
		Return(seat, nil).AnyTimes()
	mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]repository.SeatDoc{seat}, nil)
	mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(user, nil).AnyTimes()
	mockDB.EXPECT().UpdateUserFavoriteColor(gomock.Any(), "test_user_id", utils.ColorHours700To1000).Return(nil)
	mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), false).
		DoAndReturn(func(_ context.Context, _ *firestore.Transaction, updated repository.SeatDoc, _ bool) error {
			assert.Equal(t, utils.SeatAppearanceSchemaVersion, updated.Appearance.SchemaVersion)
			assert.Equal(t, utils.ColorHours700To1000, updated.Appearance.TopBarColor)
			assert.Equal(t, 2, updated.Appearance.Rank)
			assert.False(t, updated.Appearance.RankVisible)
			assert.Equal(t, 1, updated.Appearance.NumStars)
			return nil
		})

	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), "@テストユーザー さん、お気に入りカラーを更新しました🎨").Return(nil)

	app := WorkspaceApp{
		Repository:               mockDB,
		LiveChatBot:              mockLiveChatBot,
		alertOwnerBot:            moderatorbot.DummyMessageBot{},
		ProcessedUserID:          "test_user_id",
		ProcessedUserDisplayName: "テストユーザー",
		nowFunc:                  func() time.Time { return fixedNow },
	}
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		t.Fatal(err)
	}
	assert.NoError(t, app.My(context.Background(), []utils.MyOption{{
		Type:        utils.FavoriteColor,
		StringValue: utils.ColorName700To1000,
	}}))
}

func TestSystem_My(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	myTestCases := []struct {
		name                 string
		constantsConfig      repository.ConstantsConfigDoc
		commandDetails       utils.CommandDetails
		userIsMember         bool
		currentUserDoc       repository.UserDoc
		expectedReplyMessage string
	}{
		{
			name:                 "ランク表示モードオン",
			constantsConfig:      repository.ConstantsConfigDoc{MaxSeats: 10},
			commandDetails:       utils.CommandDetails{CommandType: utils.My, MyOptions: []utils.MyOption{{Type: utils.RankVisible, BoolValue: true}}},
			userIsMember:         false,
			currentUserDoc:       repository.UserDoc{RankVisible: false},
			expectedReplyMessage: "@テストユーザー さん、ランク表示をオンにしました🎯",
		},
		{
			name:                 "ランク表示モードオフ",
			constantsConfig:      repository.ConstantsConfigDoc{MaxSeats: 10},
			commandDetails:       utils.CommandDetails{CommandType: utils.My, MyOptions: []utils.MyOption{{Type: utils.RankVisible, BoolValue: false}}},
			userIsMember:         false,
			currentUserDoc:       repository.UserDoc{RankVisible: true},
			expectedReplyMessage: "@テストユーザー さん、ランク表示をオフにしました🎯",
		},
		{
			name:                 "ランク表示モードオン（すでにオン）",
			constantsConfig:      repository.ConstantsConfigDoc{MaxSeats: 10},
			commandDetails:       utils.CommandDetails{CommandType: utils.My, MyOptions: []utils.MyOption{{Type: utils.RankVisible, BoolValue: true}}},
			userIsMember:         false,
			currentUserDoc:       repository.UserDoc{RankVisible: true},
			expectedReplyMessage: "@テストユーザー さん、ランク表示モードはすでにオンです🎯",
		},
		{
			name:                 "ランク表示モードオフ（すでにオフ）",
			constantsConfig:      repository.ConstantsConfigDoc{MaxSeats: 10},
			commandDetails:       utils.CommandDetails{CommandType: utils.My, MyOptions: []utils.MyOption{{Type: utils.RankVisible, BoolValue: false}}},
			userIsMember:         false,
			currentUserDoc:       repository.UserDoc{RankVisible: false},
			expectedReplyMessage: "@テストユーザー さん、ランク表示モードはすでにオフです🎯",
		},
		{
			name:                 "お気に入り作業時間設定",
			constantsConfig:      repository.ConstantsConfigDoc{MaxSeats: 10},
			commandDetails:       utils.CommandDetails{CommandType: utils.My, MyOptions: []utils.MyOption{{Type: utils.DefaultStudyMin, IntValue: 60}}},
			userIsMember:         false,
			currentUserDoc:       repository.UserDoc{DefaultStudyMin: 30},
			expectedReplyMessage: "@テストユーザー さん、デフォルトの作業時間を60分に設定しました⏱️",
		},
		{
			name:                 "お気に入りカラーを設定（まだ使用不可）",
			constantsConfig:      repository.ConstantsConfigDoc{MaxSeats: 10},
			commandDetails:       utils.CommandDetails{CommandType: utils.My, MyOptions: []utils.MyOption{{Type: utils.FavoriteColor, StringValue: "ff0000"}}},
			userIsMember:         false,
			currentUserDoc:       repository.UserDoc{FavoriteColor: "000000"},
			expectedReplyMessage: "@テストユーザー さん、お気に入りカラーを更新しました🎨（累計作業時間が1000時間を超えるとお気に入りカラーが使えるようになります）",
		},
		{
			name:                 "お気に入りカラー設定（使用可能）",
			constantsConfig:      repository.ConstantsConfigDoc{MaxSeats: 10},
			commandDetails:       utils.CommandDetails{CommandType: utils.My, MyOptions: []utils.MyOption{{Type: utils.FavoriteColor, StringValue: ""}}},
			userIsMember:         false,
			currentUserDoc:       repository.UserDoc{FavoriteColor: "", TotalStudySec: int(1000 * time.Hour)},
			expectedReplyMessage: "@テストユーザー さん、お気に入りカラーを更新しました🎨",
		},
	}

	for _, tt := range myTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockRepository(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockDBClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						return f(ctx, &firestore.Transaction{})
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(tt.currentUserDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateUserRankVisible(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserDefaultStudyMin(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).MaxTimes(1)
			mockDB.EXPECT().UpdateUserFavoriteColor(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).MaxTimes(1)

			mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				LiveChatBot:              mockLiveChatBot,
				alertOwnerBot:            moderatorbot.DummyMessageBot{},
				ProcessedUserID:          "test_user_id",
				ProcessedUserDisplayName: "テストユーザー",
				Configs:                  &Configs{Constants: tt.constantsConfig},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			assert.Nil(t, app.My(context.Background(), tt.commandDetails.MyOptions))
		})
	}
}
