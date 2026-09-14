package workspaceapp

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
)

// TestSystem_OrganizeDBResumePreservesV2SeatAppearance は、休憩復帰時の full Set
// （document 全体の更新）で V2 canonical field が削り戻されないことを保証する。
// 古い youtube_organize_database Lambda が残ると V1 へ downgrade する経路になるため、
// Release 1 の writer 互換性の根幹として固定する。
func TestSystem_OrganizeDBResumePreservesV2SeatAppearance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	v2Seat := repository.SeatDoc{
		SeatID:                  3,
		UserID:                  "user-id",
		UserDisplayName:         "テストユーザー",
		WorkName:                "勉強",
		State:                   repository.BreakState,
		CurrentStateStartedAt:   fixedNow.Add(-30 * time.Minute),
		CurrentStateUntil:       fixedNow.Add(-1 * time.Minute),
		CurrentSegmentStartedAt: fixedNow.Add(-30 * time.Minute),
		Until:                   fixedNow.Add(60 * time.Minute),
		Appearance: repository.SeatAppearance{
			SchemaVersion:        2,
			TopBarColor:          "#ABCDEF",
			Rank:                 7,
			RankVisible:          true,
			ColorCode1:           "#111111",
			ColorCode2:           "#222222",
			NumStars:             3,
			ColorGradientEnabled: true,
		},
	}

	mockDB := mock_repository.NewMockRepository(ctrl)
	mockFirestoreClient := mock_repository.NewMockDBClient(ctrl)
	mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
				return f(ctx, &firestore.Transaction{})
			},
		)
	mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient)
	mockDB.EXPECT().ReadSeatsExpiredBreakUntil(gomock.Any(), fixedNow, false).
		Return([]repository.SeatDoc{v2Seat}, nil)
	mockDB.EXPECT().ReadSeat(gomock.Any(), gomock.Any(), v2Seat.SeatID, false).
		Return(v2Seat, nil)
	mockDB.EXPECT().CreateWorkSegmentDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), false).
		DoAndReturn(func(_ context.Context, _ *firestore.Transaction, seat repository.SeatDoc, _ bool) error {
			assert.Equal(t, 2, seat.Appearance.SchemaVersion)
			assert.Equal(t, v2Seat.Appearance, seat.Appearance)
			return nil
		})
	mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), gomock.Any()).Return(nil)

	app := WorkspaceApp{
		Repository:    mockDB,
		LiveChatBot:   mockLiveChatBot,
		alertOwnerBot: moderatorbot.DummyMessageBot{},
		nowFunc:       func() time.Time { return fixedNow },
	}

	if err := i18n.LoadLocaleFolderFS(); err != nil {
		t.Fatal(err)
	}

	assert.NoError(t, app.OrganizeDBResume(context.Background(), false))
}
