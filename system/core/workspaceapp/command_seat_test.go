package workspaceapp

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"app.modules/core/i18n"
	"app.modules/core/repository"
	mock_myfirestore "app.modules/core/repository/mocks"
	"app.modules/core/utils"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var inTestCases = []struct {
	name                 string
	constantsConfig      repository.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	targetSeatDoc        *repository.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "一般席入室",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet: true,
				SeatId:      1,
				MinutesAndWorkName: &utils.MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					DurationMin:      30,
					WorkName:         "テスト作業",
				},
				IsMemberSeat: false,
			},
		},
		userIsMember:         false,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさんが作業を始めました🔥（最大30分、1番席）",
	},
	{
		name: "メンバー席入室",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet: true,
				SeatId:      1,
				MinutesAndWorkName: &utils.MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					DurationMin:      30,
					WorkName:         "テスト作業",
				},
				IsMemberSeat: true,
			},
		},
		userIsMember:         true,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさんが作業を始めました🔥（最大30分、VIP1番席）",
	},
	{
		name: "メンバー以外がメンバー席入室",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet: true,
				SeatId:      1,
				MinutesAndWorkName: &utils.MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					DurationMin:      30,
					WorkName:         "テスト作業",
				},
				IsMemberSeat: true,
			},
		},
		userIsMember:         false,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさん、メンバー限定席に座るには、メンバー登録が必要です🍀",
	},
	{
		name: "一般席：座席指定なし",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 1,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet:        false,
				MinutesAndWorkName: &utils.MinWorkOrderOption{},
				IsMemberSeat:       false,
			},
		},
		userIsMember:         false,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさんが作業を始めました🔥（最大100分、1番席）",
	},
	{
		name: "一般席：指定した座席が空いていない",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet: true,
				SeatId:      1,
			},
		},
		userIsMember: false,
		targetSeatDoc: &repository.SeatDoc{
			SeatId: 1,
			UserId: "test_user_id",
		},
		expectedReplyMessage: "@テストユーザーさん、その番号の席は今は使えません。他の空いている席を選ぶか、「!in」で席を指定せずに入室してください🪑",
	},
	{
		name: "一般席：座席が存在しない",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet:        true,
				MinutesAndWorkName: &utils.MinWorkOrderOption{},
				SeatId:             999,
			},
		},
		userIsMember:         false,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさん、その番号の席は今は使えません。他の空いている席を選ぶか、「!in」で席を指定せずに入室してください🪑",
	},
	{
		name: "メンバー席：座席指定なし",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           1,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet:        false,
				MinutesAndWorkName: &utils.MinWorkOrderOption{},
				IsMemberSeat:       true,
			},
		},
		userIsMember:         true,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさんが作業を始めました🔥（最大100分、VIP1番席）",
	},
}

func TestSystem_In(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range inTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			if tt.commandDetails.InOption.IsSeatIdSet {
				var seatDoc repository.SeatDoc
				var seatErr error
				if tt.targetSeatDoc != nil {
					seatDoc = *tt.targetSeatDoc
					seatErr = nil
				} else {
					seatDoc = repository.SeatDoc{}
					seatErr = status.Errorf(codes.NotFound, "")
				}
				mockDB.EXPECT().ReadSeat(gomock.Any(), gomock.Any(), tt.commandDetails.InOption.SeatId, gomock.Any()).Return(seatDoc, seatErr).AnyTimes()
			}
			mockDB.EXPECT().ReadSystemConstantsConfig(gomock.Any(), gomock.Any()).Return(tt.constantsConfig, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIdAndUserId(gomock.Any(), gomock.Any(), "test_user_id", gomock.Any()).
				Return([]repository.SeatLimitDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatLimitsBLACKListWithSeatIdAndUserId(gomock.Any(), gomock.Any(), "test_user_id", gomock.Any()).
				Return([]repository.SeatLimitDoc{}, nil).AnyTimes()
			mockDB.EXPECT().GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]repository.UserActivityDoc{}, nil).AnyTimes()
			mockDB.EXPECT().GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]repository.UserActivityDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").
				Return(repository.UserDoc{
					DefaultStudyMin:    100,
					RankVisible:        false,
					IsContinuousActive: false,
				}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", true).
				Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", false).
				Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateSeat(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserLastEnteredDate(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(gomock.Any(), gomock.Any(), "test_user_id", true, gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserLastPenaltyImposedDays(gomock.Any(), gomock.Any(), "test_user_id", 0).Return(nil).AnyTimes()
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]repository.SeatDoc{}, nil).AnyTimes()
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
				Repository:               mockDB,
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserId:          "test_user_id",
				ProcessedUserDisplayName: "テストユーザー",
				ProcessedUserIsMember:    tt.userIsMember,
			}
			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.In(context.Background(), &tt.commandDetails.InOption)

			assert.Nil(t, err)
		})
	}
}

var outTestCases = []struct {
	name                 string
	constantsConfig      repository.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	expectedReplyMessage string
}{
	{
		name: "一般席退室",
		commandDetails: utils.CommandDetails{
			CommandType: utils.Out,
		},
		expectedReplyMessage: "@テストユーザーさんが退室しました🚪 （+ 0分、1番席）",
	},
	{
		name: "メンバー席退室",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Out,
		},
		userIsMember:         true,
		expectedReplyMessage: "@テストユーザーさんが退室しました🚪 （+ 0分、VIP1番席）",
	},
}

func TestSystem_Out(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range outTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(repository.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(repository.SeatDoc{
				SeatId: 1,
				UserId: "test_user_id",
			}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().DeleteSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserLastExitedDate(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserTotalTime(gomock.Any(), "test_user_id", gomock.Any(), gomock.Any()).Return(nil).Times(1)
			mockDB.EXPECT().UpdateUserRankPoint(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).Times(1)

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.Out(context.Background(), &tt.commandDetails)

			assert.Nil(t, err)
		})
	}
}

var showSeatInfoTestCases = []struct {
	name                 string
	constantsConfig      repository.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *repository.SeatDoc
	generalSeats         []repository.SeatDoc
	memberSeats          []repository.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "座席表示（退室時）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Seat,
		},
		userIsMember:         false,
		currentSeatDoc:       nil,
		generalSeats:         []repository.SeatDoc{},
		memberSeats:          []repository.SeatDoc{},
		expectedReplyMessage: "@テストユーザーさんは入室していません。「!in」コマンドで入室しましょう！📝",
	},
	{
		name: "座席表示（一般席）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Seat,
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                3,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		generalSeats: []repository.SeatDoc{
			{
				SeatId: 3,
				UserId: "test_user_id",
				State:  repository.WorkState,
			},
		},
		memberSeats:          []repository.SeatDoc{},
		expectedReplyMessage: "@テストユーザーさんは3番の席で作業中です💪現在10分入室中、作業時間は10分、自動退室まで残り89分です📊",
	},
	{
		name: "座席表示（メンバー席）",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Seat,
		},
		userIsMember: true,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                3,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		generalSeats: []repository.SeatDoc{},
		memberSeats: []repository.SeatDoc{
			{
				SeatId: 3,
				UserId: "test_user_id",
				State:  repository.WorkState,
			},
		},
		expectedReplyMessage: "@テストユーザーさんはVIP3番の席で作業中です💪現在10分入室中、作業時間は10分、自動退室まで残り89分です📊",
	},
	{
		name: "座席表示（一般席：詳細あり）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats:       10,
			RecentRangeMin: 1440,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Seat,
			SeatOption: utils.SeatOption{
				ShowDetails: true,
			},
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                3,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		generalSeats: []repository.SeatDoc{
			{
				SeatId: 3,
				UserId: "test_user_id",
				State:  repository.WorkState,
			},
		},
		memberSeats:          []repository.SeatDoc{},
		expectedReplyMessage: "@テストユーザーさんは3番の席で作業中です💪現在10分入室中、作業時間は10分、自動退室まで残り89分です📊過去1440分以内に3番席に合計0分着席しています🪑",
	},
}

func TestSystem_ShowSeatInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range showSeatInfoTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return(tt.generalSeats, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return(tt.memberSeats, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(repository.UserDoc{}, nil).AnyTimes()
			if tt.currentSeatDoc != nil {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			} else {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			}
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]repository.UserActivityDoc{}, nil).AnyTimes()
			mockDB.EXPECT().GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]repository.UserActivityDoc{}, nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.ShowSeatInfo(context.Background(), &tt.commandDetails.SeatOption)

			assert.Nil(t, err)
		})
	}
}

var changeTestCases = []struct {
	name                 string
	constantsConfig      repository.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *repository.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "作業内容・入室時間変更（一般席）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats:       10,
			MinWorkTimeMin: 5,
			MaxWorkTimeMin: 360,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Change,
			ChangeOption: utils.MinWorkOrderOption{
				IsWorkNameSet:    true,
				IsDurationMinSet: true,
				WorkName:         "テスト作業",
				DurationMin:      360,
			},
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、作業内容を更新しました✍️（5番席）入室時間を360分に変更しました。現在10分入室中。自動退室まで残り349分です⏱️",
	},
	{
		name: "作業内容・入室時間変更（メンバー席）",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
			MinWorkTimeMin:           5,
			MaxWorkTimeMin:           360,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Change,
			ChangeOption: utils.MinWorkOrderOption{
				IsWorkNameSet:    true,
				IsDurationMinSet: true,
				WorkName:         "テスト作業",
				DurationMin:      360,
			},
		},
		userIsMember: true,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、作業内容を更新しました✍️（VIP7番席）入室時間を360分に変更しました。現在10分入室中。自動退室まで残り349分です⏱️",
	},
}

func TestSystem_Change(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range changeTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(repository.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat repository.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, tt.commandDetails.ChangeOption.DurationMin, int(seat.Until.Sub(seat.EnteredAt).Minutes()))
				assert.Equal(t, tt.commandDetails.ChangeOption.WorkName, seat.WorkName)
				return nil
			}).Times(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.Change(context.Background(), &tt.commandDetails.ChangeOption)

			assert.Nil(t, err)
		})
	}
}

var moreTestCases = []struct {
	name                 string
	constantsConfig      repository.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *repository.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "作業時間延長（一般席）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats:       10,
			MinWorkTimeMin: 5,
			MaxWorkTimeMin: 360,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.More,
			MoreOption: utils.MoreOption{
				DurationMin: 30,
			},
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、自動退室までの時間を30分延長しました⏱️現在10分入室中。自動退室まで残り119分です⏳",
	},
	{
		name: "作業時間延長（メンバー席）",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
			MinWorkTimeMin:           5,
			MaxWorkTimeMin:           360,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.More,
			MoreOption: utils.MoreOption{
				DurationMin: 30,
			},
		},
		userIsMember: true,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、自動退室までの時間を30分延長しました⏱️現在10分入室中。自動退室まで残り119分です⏳",
	},
}

func TestSystem_More(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range moreTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(repository.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat repository.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, tt.currentSeatDoc.Until.Add(30*time.Minute), seat.Until)
				assert.Equal(t, tt.currentSeatDoc.WorkName, seat.WorkName)
				return nil
			}).Times(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.More(context.Background(), &tt.commandDetails.MoreOption)

			assert.Nil(t, err)
		})
	}
}

var breakTestCases = []struct {
	name                 string
	constantsConfig      repository.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *repository.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "休憩開始（一般席）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats:                10,
			DefaultBreakDurationMin: 30,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Break,
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが休憩します☕（最大30分、5番席）",
	},
	{
		name: "休憩開始（メンバー席）",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
			DefaultBreakDurationMin:  30,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Break,
		},
		userIsMember: true,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが休憩します☕（最大30分、VIP7番席）",
	},
	{
		name: "休憩開始（一般席：休憩中）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats:                10,
			DefaultBreakDurationMin: 30,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Break,
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 repository.BreakState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、作業中のみ使えるコマンドです🙏",
	},
}

func TestSystem_Break(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range breakTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(repository.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat repository.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, repository.BreakState, seat.State)
				assert.Equal(t, tt.currentSeatDoc.WorkName, seat.WorkName)
				return nil
			}).MaxTimes(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.Break(context.Background(), &tt.commandDetails.BreakOption)

			assert.Nil(t, err)
		})
	}
}

var resumeTestCases = []struct {
	name                 string
	constantsConfig      repository.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *repository.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "作業再開（一般席）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Resume,
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 repository.BreakState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが作業を再開します🔥（5番席、自動退室まで89分）",
	},
	{
		name: "作業再開（メンバー席）",
		constantsConfig: repository.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Resume,
		},
		userIsMember: true,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 repository.BreakState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが作業を再開します🔥（VIP7番席、自動退室まで89分）",
	},
	{
		name: "作業再開（一般席：作業中）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Resume,
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 repository.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、座席で休憩中のみ使えるコマンドです🙏",
	},
}

func TestSystem_Resume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range resumeTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
			mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
				DoAndReturn(
					func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) error {
						tx := &firestore.Transaction{}
						return f(ctx, tx)
					},
				).AnyTimes()
			mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(repository.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat repository.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, repository.WorkState, seat.State)
				assert.Equal(t, tt.currentSeatDoc.WorkName, seat.WorkName)
				return nil
			}).MaxTimes(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.Resume(context.Background(), &tt.commandDetails.ResumeOption)

			assert.Nil(t, err)
		})
	}
}

var orderTestCases = []struct {
	name                     string
	constantsConfig          repository.ConstantsConfigDoc
	commandDetails           utils.CommandDetails
	userIsMember             bool
	currentSeatDoc           *repository.SeatDoc
	alreadyOrderedCountToday int64
	newOrderHistory          *repository.OrderHistoryDoc
	expectedReplyMessage     string
}{
	{
		name: "メニュー注文（一般席）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxDailyOrderCount: 5,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Order,
			OrderOption: utils.OrderOption{
				IntValue:  1,
				ClearFlag: false,
			},
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:   1,
			UserId:   "test_user_id",
			MenuCode: "",
		},
		alreadyOrderedCountToday: 0,
		newOrderHistory: &repository.OrderHistoryDoc{
			UserId:   "test_user_id",
			MenuCode: "black-tea",
		},
		expectedReplyMessage: "@テストユーザーさん、紅茶の注文を受け付けました🍽（本日1回目）",
	},
	{
		name: "メニュー注文（メンバー席）",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxDailyOrderCount: 5,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Order,
			OrderOption: utils.OrderOption{
				IntValue:  1,
				ClearFlag: false,
			},
		},
		userIsMember: true,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:   1,
			UserId:   "test_user_id",
			MenuCode: "",
		},
		alreadyOrderedCountToday: 0,
		newOrderHistory: &repository.OrderHistoryDoc{
			UserId:   "test_user_id",
			MenuCode: "black-tea",
		},
		expectedReplyMessage: "@テストユーザーさん、紅茶の注文を受け付けました🍽（本日1回目）",
	},
	{
		name: "入室してないなら注文できない",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxDailyOrderCount: 5,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Order,
			OrderOption: utils.OrderOption{
				IntValue:  1,
				ClearFlag: false,
			},
		},
		userIsMember:         false,
		currentSeatDoc:       nil,
		expectedReplyMessage: "@テストユーザーさん、入室中のみ使えるコマンドです🚪",
	},
	{
		name: "非メンバーは注文回数に上限あり",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxDailyOrderCount: 5,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Order,
			OrderOption: utils.OrderOption{
				IntValue:  1,
				ClearFlag: false,
			},
		},
		userIsMember: false,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:   1,
			UserId:   "test_user_id",
			MenuCode: "",
		},
		alreadyOrderedCountToday: 5,
		expectedReplyMessage:     "@テストユーザーさん、本日の注文回数が上限(5回)に達しています😔",
	},
	{
		name: "メンバーは注文回数に上限なし",
		constantsConfig: repository.ConstantsConfigDoc{
			MaxDailyOrderCount: 5,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Order,
			OrderOption: utils.OrderOption{
				IntValue:  1,
				ClearFlag: false,
			},
		},
		userIsMember: true,
		currentSeatDoc: &repository.SeatDoc{
			SeatId:   1,
			UserId:   "test_user_id",
			MenuCode: "",
		},
		alreadyOrderedCountToday: 5,
		newOrderHistory: &repository.OrderHistoryDoc{
			UserId:   "test_user_id",
			MenuCode: "black-tea",
		},
		expectedReplyMessage: "@テストユーザーさん、紅茶の注文を受け付けました🍽（本日6回目）",
	},
}

func TestSystem_Order(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	menuDocs := []repository.MenuDoc{
		{
			Code: "black-tea",
			Name: "紅茶",
		},
		{
			Code: "coffee",
			Name: "コーヒー",
		},
	}
	// メニューコードで昇順ソート
	sort.Slice(menuDocs, func(i, j int) bool {
		return menuDocs[i].Code < menuDocs[j].Code
	})

	for _, tt := range orderTestCases {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := mock_myfirestore.NewMockFirestoreController(ctrl)
			mockFirestoreClient := mock_myfirestore.NewMockFirestoreClient(ctrl)
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

			if tt.currentSeatDoc != nil {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			} else {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			}
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()

			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().ReadAllMenuDocsOrderByCode(gomock.Any()).Return(menuDocs, nil).AnyTimes()
			mockDB.EXPECT().CountUserOrdersOfTheDay(gomock.Any(), "test_user_id", gomock.Any()).Return(tt.alreadyOrderedCountToday, nil).AnyTimes()
			mockDB.EXPECT().CreateOrderHistoryDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat repository.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.NotNil(t, tt.currentSeatDoc.MenuCode)
				return nil
			}).MaxTimes(1)

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			app := WorkspaceApp{
				Repository:               mockDB,
				ProcessedUserId:          "test_user_id",
				ProcessedUserIsMember:    tt.userIsMember,
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &Configs{
					Constants: tt.constantsConfig,
				},
				SortedMenuItems: menuDocs,
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := app.Order(context.Background(), &tt.commandDetails.OrderOption)

			assert.Nil(t, err)
		})
	}
}
