package workspaceapp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
)

// 入室制限の判定は、削除バッチが消す前の期限切れレコードを無視し、有効なレコードだけで重複・矛盾を検査する。
func TestCheckIfUserSittingTooMuchForSeat_IgnoresExpiredSeatLimits(t *testing.T) {
	const userID = "seat-limit-user"
	const seatID = 7
	now := time.Date(2026, time.October, 8, 10, 0, 0, 0, timeutil.JapanLocation())
	expired := repository.SeatLimitDoc{SeatID: seatID, UserID: userID, Until: now.Add(-time.Minute)}
	active := repository.SeatLimitDoc{SeatID: seatID, UserID: userID, Until: now.Add(time.Hour)}

	tests := []struct {
		name        string
		white       []repository.SeatLimitDoc
		black       []repository.SeatLimitDoc
		wantLimited bool
		wantErr     bool
	}{
		{name: "期限切れと再登録した有効なホワイトリスト", white: []repository.SeatLimitDoc{expired, active}},
		{name: "有効なホワイトリストと期限切れのブラックリスト", white: []repository.SeatLimitDoc{active}, black: []repository.SeatLimitDoc{expired}},
		{name: "期限切れのホワイトリストと有効なブラックリスト", white: []repository.SeatLimitDoc{expired}, black: []repository.SeatLimitDoc{active}, wantLimited: true},
		{name: "有効なホワイトリストの重複はエラー", white: []repository.SeatLimitDoc{active, active}, wantErr: true},
		{name: "有効なホワイトリストとブラックリストの両方はエラー", white: []repository.SeatLimitDoc{active}, black: []repository.SeatLimitDoc{active}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDB := mock_repository.NewMockRepository(ctrl)
			mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIDAndUserID(gomock.Any(), seatID, userID, false).Return(tt.white, nil)
			mockDB.EXPECT().ReadSeatLimitsBLACKListWithSeatIDAndUserID(gomock.Any(), seatID, userID, false).Return(tt.black, nil)
			// 有効なレコードがあれば入退室履歴は読まず、新しいレコードも登録しない（gomock が想定外の呼び出しを拒否する）。
			app := WorkspaceApp{
				Repository: mockDB,
				Configs:    &Configs{Constants: repository.ConstantsConfigDoc{RecentRangeMin: 30, RecentThresholdMin: 15}},
				nowFunc:    func() time.Time { return now },
			}

			limited, err := app.CheckIfUserSittingTooMuchForSeat(context.Background(), userID, seatID, false)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLimited, limited)
		})
	}
}
