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
