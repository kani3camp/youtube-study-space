package workspaceapp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/studyspaceerror"
	"app.modules/core/timeutil"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
)

func newSingleAttemptDBClient(ctrl *gomock.Controller) *mock_repository.MockDBClient {
	client := mock_repository.NewMockDBClient(ctrl)
	client.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
			return f(ctx, &firestore.Transaction{})
		}).AnyTimes()
	return client
}

// 自動空席探索の上限は、一般席では max_seats、メンバー席では member_max_seats とする。
func TestMinAvailableSeatIDForUser_UsesRoomSpecificMaxSeats(t *testing.T) {
	const userID = "seat-search-user"
	constants := repository.ConstantsConfigDoc{MaxSeats: 5, MemberMaxSeats: 2}
	tests := []struct {
		name         string
		isMemberSeat bool
		usedSeatIDs  []int
		wantSeatID   int
		wantErr      error
	}{
		{name: "一般席は max_seats まで探索する", isMemberSeat: false, usedSeatIDs: []int{1, 2, 3}, wantSeatID: 4},
		{name: "メンバー席は member_max_seats 以内の空席を返す", isMemberSeat: true, usedSeatIDs: []int{1}, wantSeatID: 2},
		{name: "メンバー席は member_max_seats を超える席を返さない", isMemberSeat: true, usedSeatIDs: []int{1, 2}, wantErr: studyspaceerror.ErrNoSeatAvailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mock_repository.NewMockRepository(ctrl)
			var seats []repository.SeatDoc
			for _, id := range tt.usedSeatIDs {
				seats = append(seats, repository.SeatDoc{SeatID: id, UserID: "other-user"})
			}
			if tt.isMemberSeat {
				mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return(seats, nil)
			} else {
				mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return(seats, nil)
			}
			mockDB.EXPECT().ReadSystemConstantsConfig(gomock.Any(), gomock.Any()).Return(constants, nil)
			mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIDAndUserID(gomock.Any(), gomock.Any(), userID, tt.isMemberSeat).Return(nil, nil).AnyTimes()
			mockDB.EXPECT().ReadSeatLimitsBLACKListWithSeatIDAndUserID(gomock.Any(), gomock.Any(), userID, tt.isMemberSeat).Return(nil, nil).AnyTimes()
			mockDB.EXPECT().GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), userID, gomock.Any(), tt.isMemberSeat).Return(nil, nil).AnyTimes()
			mockDB.EXPECT().GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), userID, gomock.Any(), tt.isMemberSeat).Return(nil, nil).AnyTimes()

			fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
			app := WorkspaceApp{
				Configs:    &Configs{Constants: constants},
				Repository: mockDB,
				nowFunc:    func() time.Time { return fixedNow },
			}

			seatID, err := app.MinAvailableSeatIDForUser(context.Background(), nil, userID, tt.isMemberSeat)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSeatID, seatID)
		})
	}
}

// 座席数固定でメンバー席を減らすとき、消える席のユーザーはメンバーとして退室処理される。
func TestAdjustMemberSeats_FixedShrinkExitsUsersAsMembers(t *testing.T) {
	require.NoError(t, i18n.LoadLocaleFolderFS())
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	const userID = "member-seat-user"
	seat := repository.SeatDoc{
		SeatID:                  3,
		UserID:                  userID,
		UserDisplayName:         "メンバー",
		State:                   repository.WorkState,
		EnteredAt:               fixedNow.Add(-10 * time.Minute),
		CurrentStateStartedAt:   fixedNow.Add(-10 * time.Minute),
		CurrentSegmentStartedAt: fixedNow.Add(-10 * time.Minute),
		Until:                   fixedNow.Add(time.Hour),
	}
	constants := repository.ConstantsConfigDoc{
		FixedMaxSeatsEnabled:  true,
		MemberMaxSeats:        4,
		DesiredMemberMaxSeats: 2,
	}

	app := &WorkspaceApp{
		Configs:       &Configs{Constants: constants},
		alertOwnerBot: moderatorbot.DummyMessageBot{},
		nowFunc:       func() time.Time { return fixedNow },
	}
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockDB.EXPECT().FirestoreClient().Return(newSingleAttemptDBClient(ctrl)).AnyTimes()
	mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]repository.SeatDoc{seat}, nil)
	mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), userID).Return(repository.UserDoc{}, nil)
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, true).Return(seat, nil).AnyTimes()
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, false).Return(repository.SeatDoc{}, status.Error(codes.NotFound, "")).AnyTimes()
	mockDB.EXPECT().ReadWorkStateSegmentsBySessionID(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockDB.EXPECT().DeleteSeat(gomock.Any(), gomock.Any(), seat.SeatID, true).
		DoAndReturn(func(context.Context, *firestore.Transaction, int, bool) error {
			assert.Equal(t, userID, app.ProcessedUserID)
			assert.True(t, app.ProcessedUserIsMember, "メンバー席のユーザーをメンバーとして扱う")
			return nil
		})
	mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockDB.EXPECT().CreateWorkSegmentDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockDB.EXPECT().UpdateUserLastExitedDate(gomock.Any(), userID, gomock.Any()).Return(nil)
	mockDB.EXPECT().UpdateUserTotalTime(gomock.Any(), userID, gomock.Any(), gomock.Any()).Return(nil)
	mockDB.EXPECT().UpdateUserRankPoint(gomock.Any(), userID, gomock.Any()).Return(nil)
	mockDB.EXPECT().UpdateMemberMaxSeats(gomock.Any(), gomock.Any(), constants.DesiredMemberMaxSeats).Return(nil)
	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	app.Repository = mockDB
	app.LiveChatBot = mockLiveChatBot

	require.NoError(t, app.adjustMemberSeats(context.Background(), constants))
}

// メンバー席の削減を却下したときのログには、メンバー席の desired 値を記録する。
func TestAdjustMemberSeats_RejectedShrinkLogsDesiredMemberMaxSeats(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	ctrl := gomock.NewController(t)
	constants := repository.ConstantsConfigDoc{
		MaxSeats:              30,
		DesiredMaxSeats:       30,
		MemberMaxSeats:        4,
		DesiredMemberMaxSeats: 2,
		MinVacancyRate:        0.5,
	}
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockDB.EXPECT().ReadMemberSeats(gomock.Any()).Return([]repository.SeatDoc{{SeatID: 1}, {SeatID: 2}}, nil)
	mockDB.EXPECT().UpdateDesiredMemberMaxSeats(gomock.Any(), gomock.Any(), constants.MemberMaxSeats).Return(nil)
	app := &WorkspaceApp{Configs: &Configs{Constants: constants}, Repository: mockDB}

	require.NoError(t, app.adjustMemberSeats(context.Background(), constants))

	var record map[string]any
	found := false
	for _, line := range bytes.Split(bytes.TrimSpace(logs.Bytes()), []byte("\n")) {
		require.NoError(t, json.Unmarshal(line, &record))
		if _, ok := record["current member max seats"]; ok {
			found = true
			break
		}
	}
	require.True(t, found, "メンバー席削減の却下ログが出力されていない")
	assert.InDelta(t, float64(constants.DesiredMemberMaxSeats), record["desired"], 0)
}
