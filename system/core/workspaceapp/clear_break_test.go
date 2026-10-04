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
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
)

func TestClearDuringBreakClearsRegularWorkName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID:                  5,
		UserID:                  "test_user_id",
		WorkName:                "資格勉強",
		State:                   repository.BreakState,
		EnteredAt:               fixedNow.Add(-30 * time.Minute),
		Until:                   fixedNow.Add(90 * time.Minute),
		CurrentStateStartedAt:   fixedNow.Add(-10 * time.Minute),
		CurrentStateUntil:       fixedNow.Add(20 * time.Minute),
		CurrentSegmentStartedAt: fixedNow.Add(-10 * time.Minute),
	}

	mockDB := mock_repository.NewMockRepository(ctrl)
	mockFirestoreClient := mock_repository.NewMockDBClient(ctrl)
	mockFirestoreClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
			return f(ctx, &firestore.Transaction{})
		},
	).Times(1)
	mockDB.EXPECT().FirestoreClient().Return(mockFirestoreClient).AnyTimes()
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", false).Return(seat, nil).AnyTimes()
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), "test_user_id", true).Return(repository.SeatDoc{}, status.Errorf(codes.NotFound, "")).AnyTimes()
	mockDB.EXPECT().CreateWorkSegmentDoc(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *firestore.Transaction, segment repository.WorkSegmentDoc) error {
			assert.Equal(t, repository.BreakState, segment.SegmentType)
			assert.Equal(t, "資格勉強", segment.WorkName)
			return nil
		},
	).Times(1)
	mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), false).DoAndReturn(
		func(_ context.Context, _ *firestore.Transaction, updated repository.SeatDoc, _ bool) error {
			assert.Equal(t, repository.BreakState, updated.State)
			assert.Empty(t, updated.WorkName)
			assert.Equal(t, fixedNow, updated.CurrentSegmentStartedAt)
			return nil
		},
	).Times(1)

	mockLiveChatBot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	mockLiveChatBot.EXPECT().PostMessage(gomock.Any(), "@テストユーザー さん、作業内容をリセットしました🧹(5番席)").Return(nil).Times(1)

	app := WorkspaceApp{
		Repository:               mockDB,
		LiveChatBot:              mockLiveChatBot,
		alertOwnerBot:            moderatorbot.DummyMessageBot{},
		ProcessedUserID:          "test_user_id",
		ProcessedUserDisplayName: "テストユーザー",
		nowFunc:                  func() time.Time { return fixedNow },
	}

	if err := i18n.LoadLocaleFolderFS(); err != nil {
		panic(fmt.Errorf("in LoadLocaleFolderFS(): %w", err))
	}

	assert.NoError(t, app.Clear(context.Background()))
}
