package core_test

import (
	"app.modules/core"
	"app.modules/core/i18n"
	"app.modules/core/myfirestore"
	mock_myfirestore "app.modules/core/myfirestore/mocks"
	"app.modules/core/utils"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
	"cloud.google.com/go/firestore"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

// TODO: 各ケースでちゃんとエラーがハンドリングされること（返されること、ハンドリングされること）

var inTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	targetSeatDoc        *myfirestore.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "一般席入室",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet: true,
				SeatId:      1,
				MinutesAndWorkName: &utils.MinutesAndWorkNameOption{
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
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet: true,
				SeatId:      1,
				MinutesAndWorkName: &utils.MinutesAndWorkNameOption{
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
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet: true,
				SeatId:      1,
				MinutesAndWorkName: &utils.MinutesAndWorkNameOption{
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
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 1,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet:        false,
				MinutesAndWorkName: &utils.MinutesAndWorkNameOption{},
				IsMemberSeat:       false,
			},
		},
		userIsMember:         false,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさんが作業を始めました🔥（最大100分、1番席）",
	},
	{
		name: "一般席：指定した座席が空いていない",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		targetSeatDoc: &myfirestore.SeatDoc{
			SeatId: 1,
			UserId: "test_user_id",
		},
		expectedReplyMessage: "@テストユーザーさん、その番号の席は今は使えません。他の空いている席を選ぶか、「!in」で席を指定せずに入室してください",
	},
	{
		name: "一般席：座席が存在しない",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet:        true,
				MinutesAndWorkName: &utils.MinutesAndWorkNameOption{},
				SeatId:             999,
			},
		},
		userIsMember:         false,
		targetSeatDoc:        nil,
		expectedReplyMessage: "@テストユーザーさん、その番号の席は今は使えません。他の空いている席を選ぶか、「!in」で席を指定せずに入室してください",
	},
	{
		name: "メンバー席：座席指定なし",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           1,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.In,
			InOption: utils.InOption{
				IsSeatIdSet:        false,
				MinutesAndWorkName: &utils.MinutesAndWorkNameOption{},
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
				var seatDoc myfirestore.SeatDoc
				var seatErr error
				if tt.targetSeatDoc != nil {
					seatDoc = *tt.targetSeatDoc
					seatErr = nil
				} else {
					seatDoc = myfirestore.SeatDoc{}
					seatErr = status.Errorf(codes.NotFound, "")
				}
				mockDB.EXPECT().ReadSeat(gomock.Any(), gomock.Any(), tt.commandDetails.InOption.SeatId, gomock.Any()).Return(seatDoc, seatErr).AnyTimes()
			}
			mockDB.EXPECT().ReadSystemConstantsConfig(gomock.Any(), gomock.Any()).Return(tt.constantsConfig, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIdAndUserId(gomock.Any(), gomock.Any(), "test_user_id", gomock.Any()).
				Return([]myfirestore.SeatLimitDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatLimitsBLACKListWithSeatIdAndUserId(gomock.Any(), gomock.Any(), "test_user_id", gomock.Any()).
				Return([]myfirestore.SeatLimitDoc{}, nil).AnyTimes()
			mockDB.EXPECT().GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]myfirestore.UserActivityDoc{}, nil).AnyTimes()
			mockDB.EXPECT().GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]myfirestore.UserActivityDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").
				Return(myfirestore.UserDoc{
					DefaultStudyMin:    100,
					RankVisible:        false,
					IsContinuousActive: false,
				}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", true).
				Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", false).
				Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateSeat(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserLastEnteredDate(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(gomock.Any(), gomock.Any(), "test_user_id", true, gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserLastPenaltyImposedDays(gomock.Any(), gomock.Any(), "test_user_id", 0).Return(nil).AnyTimes()
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]myfirestore.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]myfirestore.SeatDoc{}, nil).AnyTimes()
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

			system := core.System{
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
				FirestoreController:      mockDB,
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserId:          "test_user_id",
				ProcessedUserDisplayName: "テストユーザー",
				ProcessedUserIsMember:    tt.userIsMember,
			}
			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.In(context.Background(), &tt.commandDetails)

			assert.Nil(t, err)
		})
	}
}

var outTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	expectedReplyMessage string
}{
	{
		name: "一般席退室",
		commandDetails: utils.CommandDetails{
			CommandType: utils.Out,
		},
		expectedReplyMessage: "@テストユーザーさんが退室しました🚶🚪 （+ 0分、1番席）",
	},
	{
		name: "メンバー席退室",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Out,
		},
		userIsMember:         true,
		expectedReplyMessage: "@テストユーザーさんが退室しました🚶🚪 （+ 0分、VIP1番席）",
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
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(myfirestore.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(myfirestore.SeatDoc{
				SeatId: 1,
				UserId: "test_user_id",
			}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().DeleteSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserLastExitedDate(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserTotalTime(gomock.Any(), "test_user_id", gomock.Any(), gomock.Any()).Return(nil).Times(1)
			mockDB.EXPECT().UpdateUserRankPoint(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).Times(1)

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.Out(&tt.commandDetails, context.Background())

			assert.Nil(t, err)
		})
	}
}

var showUserInfoTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *myfirestore.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "ユーザー情報表示（退室時）",
		commandDetails: utils.CommandDetails{
			CommandType: utils.Info,
		},
		userIsMember:         false,
		currentSeatDoc:       nil,
		expectedReplyMessage: "@テストユーザーさん ［本日の作業時間：0分] ［累計作業時間：0分]",
	},
	{
		name: "ユーザー情報表示（入室時）",
		commandDetails: utils.CommandDetails{
			CommandType: utils.Info,
		},
		userIsMember: false,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                1,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん ［本日の作業時間：10分] ［累計作業時間：10分]",
	},
}

func TestSystem_ShowUserInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range showUserInfoTestCases {
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
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(myfirestore.UserDoc{}, nil).AnyTimes()
			if tt.currentSeatDoc != nil {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			} else {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			}
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.ShowUserInfo(&tt.commandDetails, context.Background())

			assert.Nil(t, err)
		})
	}
}

var showSeatInfoTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *myfirestore.SeatDoc
	generalSeats         []myfirestore.SeatDoc
	memberSeats          []myfirestore.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "座席表示（退室時）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Seat,
		},
		userIsMember:         false,
		currentSeatDoc:       nil,
		generalSeats:         []myfirestore.SeatDoc{},
		memberSeats:          []myfirestore.SeatDoc{},
		expectedReplyMessage: "@テストユーザーさんは入室していません。「!in」コマンドで入室しましょう！",
	},
	{
		name: "座席表示（一般席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Seat,
		},
		userIsMember: false,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                3,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		generalSeats: []myfirestore.SeatDoc{
			{
				SeatId: 3,
				UserId: "test_user_id",
				State:  myfirestore.WorkState,
			},
		},
		memberSeats:          []myfirestore.SeatDoc{},
		expectedReplyMessage: "@テストユーザーさんは3番の席で作業中です💪現在10分入室中、作業時間は10分、自動退室まで残り89分です。",
	},
	{
		name: "座席表示（メンバー席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Seat,
		},
		userIsMember: true,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                3,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		generalSeats: []myfirestore.SeatDoc{},
		memberSeats: []myfirestore.SeatDoc{
			{
				SeatId: 3,
				UserId: "test_user_id",
				State:  myfirestore.WorkState,
			},
		},
		expectedReplyMessage: "@テストユーザーさんはVIP3番の席で作業中です💪現在10分入室中、作業時間は10分、自動退室まで残り89分です。",
	},
	{
		name: "座席表示（一般席：詳細あり）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                3,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		generalSeats: []myfirestore.SeatDoc{
			{
				SeatId: 3,
				UserId: "test_user_id",
				State:  myfirestore.WorkState,
			},
		},
		memberSeats:          []myfirestore.SeatDoc{},
		expectedReplyMessage: "@テストユーザーさんは3番の席で作業中です💪現在10分入室中、作業時間は10分、自動退室まで残り89分です。過去1440分以内に3番席に合計0分着席しています🪑",
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
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(myfirestore.UserDoc{}, nil).AnyTimes()
			if tt.currentSeatDoc != nil {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			} else {
				mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			}
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]myfirestore.UserActivityDoc{}, nil).AnyTimes()
			mockDB.EXPECT().GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return([]myfirestore.UserActivityDoc{}, nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.ShowSeatInfo(&tt.commandDetails, context.Background())

			assert.Nil(t, err)
		})
	}
}

var changeTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *myfirestore.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "作業内容・入室時間変更（一般席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats:       10,
			MinWorkTimeMin: 5,
			MaxWorkTimeMin: 360,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Change,
			ChangeOption: utils.MinutesAndWorkNameOption{
				IsWorkNameSet:    true,
				IsDurationMinSet: true,
				WorkName:         "テスト作業",
				DurationMin:      360,
			},
		},
		userIsMember: false,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、作業内容を更新しました（5番席）。入室時間を360分に変更しました。現在10分入室中。自動退室まで残り349分です。",
	},
	{
		name: "作業内容・入室時間変更（メンバー席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
			MinWorkTimeMin:           5,
			MaxWorkTimeMin:           360,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Change,
			ChangeOption: utils.MinutesAndWorkNameOption{
				IsWorkNameSet:    true,
				IsDurationMinSet: true,
				WorkName:         "テスト作業",
				DurationMin:      360,
			},
		},
		userIsMember: true,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、作業内容を更新しました（VIP7番席）。入室時間を360分に変更しました。現在10分入室中。自動退室まで残り349分です。",
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
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(myfirestore.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat myfirestore.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, tt.commandDetails.ChangeOption.DurationMin, int(seat.Until.Sub(seat.EnteredAt).Minutes()))
				assert.Equal(t, tt.commandDetails.ChangeOption.WorkName, seat.WorkName)
				return nil
			}).Times(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.Change(&tt.commandDetails, context.Background())

			assert.Nil(t, err)
		})
	}
}

var moreTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *myfirestore.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "作業時間延長（一般席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、自動退室までの時間を30分延長しました。現在10分入室中。自動退室まで残り119分です",
	},
	{
		name: "作業時間延長（メンバー席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、自動退室までの時間を30分延長しました。現在10分入室中。自動退室まで残り119分です",
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
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(myfirestore.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat myfirestore.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, tt.currentSeatDoc.Until.Add(30*time.Minute), seat.Until)
				assert.Equal(t, tt.currentSeatDoc.WorkName, seat.WorkName)
				return nil
			}).Times(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.More(&tt.commandDetails, context.Background())

			assert.Nil(t, err)
		})
	}
}

var breakTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *myfirestore.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "休憩開始（一般席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats:                10,
			DefaultBreakDurationMin: 30,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Break,
		},
		userIsMember: false,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが休憩します（最大30分、5番席）",
	},
	{
		name: "休憩開始（メンバー席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
			DefaultBreakDurationMin:  30,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Break,
		},
		userIsMember: true,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが休憩します（最大30分、VIP7番席）",
	},
	{
		name: "休憩開始（一般席：休憩中）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats:                10,
			DefaultBreakDurationMin: 30,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Break,
		},
		userIsMember: false,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 myfirestore.BreakState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、作業中のみ使えるコマンドです。",
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
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(myfirestore.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat myfirestore.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, myfirestore.BreakState, seat.State)
				assert.Equal(t, tt.currentSeatDoc.WorkName, seat.WorkName)
				return nil
			}).MaxTimes(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.Break(context.Background(), &tt.commandDetails)

			assert.Nil(t, err)
		})
	}
}

var resumeTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentSeatDoc       *myfirestore.SeatDoc
	expectedReplyMessage string
}{
	{
		name: "作業再開（一般席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Resume,
		},
		userIsMember: false,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 myfirestore.BreakState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが作業を再開します（5番席、自動退室まで89分）",
	},
	{
		name: "作業再開（メンバー席）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			YoutubeMembershipEnabled: true,
			MemberMaxSeats:           10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Resume,
		},
		userIsMember: true,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                7,
			UserId:                "test_user_id",
			State:                 myfirestore.BreakState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさんが作業を再開します（VIP7番席、自動退室まで89分）",
	},
	{
		name: "作業再開（一般席：作業中）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Resume,
		},
		userIsMember: false,
		currentSeatDoc: &myfirestore.SeatDoc{
			SeatId:                5,
			UserId:                "test_user_id",
			State:                 myfirestore.WorkState,
			CurrentStateStartedAt: time.Now().Add(-10 * time.Minute),
			EnteredAt:             time.Now().Add(-10 * time.Minute),
			Until:                 time.Now().Add(90 * time.Minute),
		},
		expectedReplyMessage: "@テストユーザーさん、座席で休憩中のみ使えるコマンドです。",
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
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(myfirestore.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(*tt.currentSeatDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), tt.userIsMember).DoAndReturn(func(ctx context.Context, tx *firestore.Transaction, seat myfirestore.SeatDoc, isMemberSeat bool) error {
				assert.Equal(t, tt.currentSeatDoc.SeatId, seat.SeatId)
				assert.Equal(t, tt.currentSeatDoc.UserId, seat.UserId)
				assert.Equal(t, myfirestore.WorkState, seat.State)
				assert.Equal(t, tt.currentSeatDoc.WorkName, seat.WorkName)
				return nil
			}).MaxTimes(1)
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.Resume(context.Background(), &tt.commandDetails)

			assert.Nil(t, err)
		})
	}
}

var rankTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentUserDoc       myfirestore.UserDoc
	expectedReplyMessage string
}{
	{
		name: "ランク表示モード切り替え（オン）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Rank,
		},
		userIsMember: false,
		currentUserDoc: myfirestore.UserDoc{
			RankVisible: false,
		},
		expectedReplyMessage: "@テストユーザーさんのランク表示をオンにしました",
	},
	{
		name: "ランク表示モード切り替え（オフ）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
			MaxSeats: 10,
		},
		commandDetails: utils.CommandDetails{
			CommandType: utils.Rank,
		},
		userIsMember: false,
		currentUserDoc: myfirestore.UserDoc{
			RankVisible: true,
		},
		expectedReplyMessage: "@テストユーザーさんのランク表示をオフにしました",
	},
}

func TestSystem_Rank(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range rankTestCases {
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
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]myfirestore.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]myfirestore.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(tt.currentUserDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserRankVisible(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.Rank(&tt.commandDetails, context.Background())

			assert.Nil(t, err)
		})
	}
}

var myTestCases = []struct {
	name                 string
	constantsConfig      myfirestore.ConstantsConfigDoc
	commandDetails       utils.CommandDetails
	userIsMember         bool
	currentUserDoc       myfirestore.UserDoc
	expectedReplyMessage string
}{
	{
		name: "ランク表示モードオン",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentUserDoc: myfirestore.UserDoc{
			RankVisible: false,
		},
		expectedReplyMessage: "@テストユーザーさん、ランク表示をオンにしました。",
	},
	{
		name: "ランク表示モードオフ",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentUserDoc: myfirestore.UserDoc{
			RankVisible: true,
		},
		expectedReplyMessage: "@テストユーザーさん、ランク表示をオフにしました。",
	},
	{
		name: "ランク表示モードオン（すでにオン）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentUserDoc: myfirestore.UserDoc{
			RankVisible: true,
		},
		expectedReplyMessage: "@テストユーザーさん、ランク表示モードはすでにオンです。",
	},
	{
		name: "ランク表示モードオフ（すでにオフ）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentUserDoc: myfirestore.UserDoc{
			RankVisible: false,
		},
		expectedReplyMessage: "@テストユーザーさん、ランク表示モードはすでにオフです。",
	},
	{
		name: "お気に入り作業時間設定",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentUserDoc: myfirestore.UserDoc{
			DefaultStudyMin: 30,
		},
		expectedReplyMessage: "@テストユーザーさん、デフォルトの作業時間を60分に設定しました。",
	},
	{
		name: "お気に入りカラーを設定（まだ使用不可）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentUserDoc: myfirestore.UserDoc{
			FavoriteColor: "000000",
		},
		expectedReplyMessage: "@テストユーザーさん、お気に入りカラーを更新しました。（累計作業時間が1000時間を超えるとお気に入りカラーが使えるようになります）",
	},
	{
		name: "お気に入りカラー設定（使用可能）",
		constantsConfig: myfirestore.ConstantsConfigDoc{
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
		currentUserDoc: myfirestore.UserDoc{
			FavoriteColor: "",
			TotalStudySec: int(1000 * time.Hour),
		},
		expectedReplyMessage: "@テストユーザーさん、お気に入りカラーを更新しました。",
	},
}

func TestSystem_My(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	for _, tt := range myTestCases {
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
			mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return([]myfirestore.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]myfirestore.SeatDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), "test_user_id").Return(tt.currentUserDoc, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().ReadSeatWithUserId(gomock.Any(), "test_user_id", !tt.userIsMember).Return(myfirestore.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
			mockDB.EXPECT().UpdateUserRankVisible(gomock.Any(), "test_user_id", gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserDefaultStudyMin(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).MaxTimes(1)
			mockDB.EXPECT().UpdateUserFavoriteColor(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).MaxTimes(1)

			mockLiveChatBot := mock_youtubebot.NewMockYoutubeLiveChatBotInterface(ctrl)
			mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), tt.expectedReplyMessage).Return(nil).Times(1)

			system := core.System{
				FirestoreController:      mockDB,
				ProcessedUserId:          "test_user_id",
				LiveChatBot:              mockLiveChatBot,
				ProcessedUserDisplayName: "テストユーザー",
				Configs: &core.SystemConfigs{
					Constants: tt.constantsConfig,
				},
			}

			if err := i18n.LoadLocaleFolderFS(); err != nil {
				panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
			}

			// テスト対象の関数を実行
			err := system.My(&tt.commandDetails, context.Background())

			assert.Nil(t, err)
		})
	}
}
