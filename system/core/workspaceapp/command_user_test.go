package workspaceapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	mock_myfirestore "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TODO: 各ケースでちゃんとエラーがハンドリングされること（返されること、ハンドリングされること）

func TestSystem_ShowUserInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())

	var showUserInfoTestCases = []struct {
		name                 string
		constantsConfig      repository.ConstantsConfigDoc
		commandDetails       utils.CommandDetails
		userIsMember         bool
		currentSeatDoc       *repository.SeatDoc
		expectedReplyMessage string
	}{
		{
			name: "ユーザー情報表示（退室時）",
			commandDetails: utils.CommandDetails{
				CommandType: utils.Info,
			},
			userIsMember:         false,
			currentSeatDoc:       nil,
			expectedReplyMessage: "@テストユーザー さん ［⏱️本日の作業時間：0分] ［📊累計作業時間：0分]",
		},
		{
			name: "ユーザー情報表示（入室時）",
			commandDetails: utils.CommandDetails{
				CommandType: utils.Info,
			},
			userIsMember: false,
			currentSeatDoc: &repository.SeatDoc{
				SeatId:                  1,
				UserId:                  "test_user_id",
				State:                   repository.WorkState,
				EnteredAt:               fixedNow.Add(-10 * time.Minute),
				CurrentStateStartedAt:   fixedNow.Add(-10 * time.Minute),
				CurrentSegmentStartedAt: fixedNow.Add(-10 * time.Minute),
			},
			expectedReplyMessage: "@テストユーザー さん ［⏱️本日の作業時間：10分] ［📊累計作業時間：10分]",
		},
	}

	for _, tt := range showUserInfoTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockRepository(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockDBClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(repository.UserDoc{}, nil).AnyTimes()
			if tt.currentSeatDoc != nil {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			} else {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			}
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				LiveChatBot:              mockLiveChatBot,
				alertOwnerBot:            moderatorbot.DummyMessageBot{},
				ProcessedUserId:          "test_user_id",
				ProcessedUserDisplayName: "テストユーザー",
				nowFunc:                  func() time.Time { return fixedNow },
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.ShowUserInfo(context.Background(), &tt.commandDetails.InfoOption)

			assert.Nil(t, err)
		})
	}
}

func TestSystem_Rank(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var rankTestCases = []struct {
		name                 string
		constantsConfig      repository.ConstantsConfigDoc
		commandDetails       utils.CommandDetails
		userIsMember         bool
		currentUserDoc       repository.UserDoc
		expectedReplyMessage string
	}{
		{
			name: "ランク表示モード切り替え（オン）",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.Rank,
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				RankVisible: false,
			},
			expectedReplyMessage: "@テストユーザー さんのランク表示をオンにしました🎯",
		},
		{
			name: "ランク表示モード切り替え（オフ）",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.Rank,
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				RankVisible: true,
			},
			expectedReplyMessage: "@テストユーザー さんのランク表示をオフにしました🎯",
		},
	}

	for _, tt := range rankTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockRepository(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockDBClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(tt.currentUserDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserRankVisible(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				LiveChatBot:              mockLiveChatBot,
				alertOwnerBot:            moderatorbot.DummyMessageBot{},
				ProcessedUserId:          "test_user_id",
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.Rank(context.Background(), &tt.commandDetails)

			assert.Nil(t, err)
		})
	}
}

func TestSystem_My(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var myTestCases = []struct {
		name                 string
		constantsConfig      repository.ConstantsConfigDoc
		commandDetails       utils.CommandDetails
		userIsMember         bool
		currentUserDoc       repository.UserDoc
		expectedReplyMessage string
	}{
		{
			name: "ランク表示モードオン",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.My,
				MyOptions: []utils.MyOption{
					{
						Type:      utils.RankVisible,
						BoolValue: true,
					},
				},
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				RankVisible: false,
			},
			expectedReplyMessage: "@テストユーザー さん、ランク表示をオンにしました🎯",
		},
		{
			name: "ランク表示モードオフ",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.My,
				MyOptions: []utils.MyOption{
					{
						Type:      utils.RankVisible,
						BoolValue: false,
					},
				},
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				RankVisible: true,
			},
			expectedReplyMessage: "@テストユーザー さん、ランク表示をオフにしました🎯",
		},
		{
			name: "ランク表示モードオン（すでにオン）",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.My,
				MyOptions: []utils.MyOption{
					{
						Type:      utils.RankVisible,
						BoolValue: true,
					},
				},
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				RankVisible: true,
			},
			expectedReplyMessage: "@テストユーザー さん、ランク表示モードはすでにオンです🎯",
		},
		{
			name: "ランク表示モードオフ（すでにオフ）",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.My,
				MyOptions: []utils.MyOption{
					{
						Type:      utils.RankVisible,
						BoolValue: false,
					},
				},
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				RankVisible: false,
			},
			expectedReplyMessage: "@テストユーザー さん、ランク表示モードはすでにオフです🎯",
		},
		{
			name: "お気に入り作業時間設定",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.My,
				MyOptions: []utils.MyOption{
					{
						Type:     utils.DefaultStudyMin,
						IntValue: 60,
					},
				},
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				DefaultStudyMin: 30,
			},
			expectedReplyMessage: "@テストユーザー さん、デフォルトの作業時間を60分に設定しました⏱️",
		},
		{
			name: "お気に入りカラーを設定（まだ使用不可）",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.My,
				MyOptions: []utils.MyOption{
					{
						Type:        utils.FavoriteColor,
						StringValue: "ff0000",
					},
				},
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				FavoriteColor: "000000",
			},
			expectedReplyMessage: "@テストユーザー さん、お気に入りカラーを更新しました🎨（累計作業時間が1000時間を超えるとお気に入りカラーが使えるようになります）",
		},
		{
			name: "お気に入りカラー設定（使用可能）",
			constantsConfig: repository.ConstantsConfigDoc{
				MaxSeats: 10,
			},
			commandDetails: utils.CommandDetails{
				CommandType: utils.My,
				MyOptions: []utils.MyOption{
					{
						Type:        utils.FavoriteColor,
						StringValue: "",
					},
				},
			},
			userIsMember: false,
			currentUserDoc: repository.UserDoc{
				FavoriteColor: "",
				TotalStudySec: int(1000 * time.Hour),
			},
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
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(tt.currentUserDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
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
				ProcessedUserId:          "test_user_id",
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.My(context.Background(), tt.commandDetails.MyOptions)

			assert.Nil(t, err)
		})
	}
}
