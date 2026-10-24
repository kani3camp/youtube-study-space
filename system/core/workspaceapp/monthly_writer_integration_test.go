//go:build integration

package workspaceapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
	"app.modules/internal/integrationtest"
)

var errMoveRollback = errors.New("synthetic move rollback after source and month writes")

type moveRollbackRepository struct {
	*repository.FirestoreControllerImplements
	fail bool
}

func (r *moveRollbackRepository) UpdateUserLastEnteredDate(tx *firestore.Transaction, uid string, at time.Time) error {
	if r.fail {
		return errMoveRollback
	}
	return r.FirestoreControllerImplements.UpdateUserLastEnteredDate(tx, uid, at)
}

func TestMonthlyInOrderMoveCommitsAndRollsBackAllDocuments(t *testing.T) {
	integrationtest.ResetFirestore(t)
	repo := integrationtest.NewFirestoreController(t)
	require.NoError(t, i18n.LoadLocaleFolderFS())
	ctx := context.Background()
	uid := "UCsynthetic0000000000001"
	boundary := time.Date(2026, 10, 24, 8, 0, 0, 0, timeutil.JapanLocation())
	start := boundary.Add(-10 * time.Minute)
	constants := repository.ConstantsConfigDoc{MaxSeats: 2, MaxWorkTimeMin: 120, MaxDailyOrderCount: 10}
	_, err := repo.FirestoreClient().Collection(repository.CONFIG).Doc(repository.SystemConstantsConfigDocName).Set(ctx, constants)
	require.NoError(t, err)
	require.NoError(t, repo.CreateUser(ctx, nil, uid, repository.UserDoc{TotalStudySec: 100, IsContinuousActive: true, CurrentActivityStateStarted: start, LastRPProcessed: boundary}))
	seat := repository.SeatDoc{SeatID: 1, UserID: uid, SessionID: "synthetic-order-move", State: repository.WorkState, WorkName: "synthetic", EnteredAt: start, CurrentStateStartedAt: start, CurrentSegmentStartedAt: start, Until: boundary.Add(time.Hour), CurrentStateUntil: boundary.Add(time.Hour)}
	require.NoError(t, repo.FirestoreClient().RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error { return repo.CreateSeat(tx, seat, false) }))
	require.NoError(t, repo.CreateSeatLimitInWHITEList(ctx, 2, uid, start, boundary.Add(time.Hour), false))
	bot := mock_youtubebot.NewMockLiveChatBot(gomock.NewController(t))
	bot.EXPECT().PostMessage(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	wrapped := &moveRollbackRepository{FirestoreControllerImplements: repo, fail: true}
	app := &WorkspaceApp{Configs: &Configs{Constants: constants}, Repository: wrapped, LiveChatBot: bot, alertOwnerBot: moderatorbot.DummyMessageBot{}, SortedMenuItems: []repository.MenuDoc{{Code: "synthetic-menu", Name: "synthetic"}}, nowFunc: func() time.Time { return boundary }}
	app.SetProcessedUser(uid, "synthetic", "", false, false, false)
	for _, fail := range []bool{true, false} {
		wrapped.fail = fail
		option := utils.InOption{IsSeatIDSet: true, SeatID: 2, MinWorkOrderOption: &utils.MinWorkOrderOption{IsWorkNameSet: true, WorkName: "next synthetic", IsDurationMinSet: true, DurationMin: 60, IsOrderSet: true, OrderNum: 1}}
		err := app.In(ctx, &option)
		if fail {
			require.ErrorIs(t, err, errMoveRollback)
		} else {
			require.NoError(t, err)
		}
		for _, collection := range []string{repository.WorkSegments, repository.WorkMonths, repository.OrderHistory} {
			rows, err := repo.FirestoreClient().Collection(collection).Where(repository.UserIDDocProperty, "==", uid).Documents(ctx).GetAll()
			require.NoError(t, err)
			if fail {
				require.Empty(t, rows)
			} else {
				require.Len(t, rows, 1)
			}
		}
		if fail {
			original, err := repo.ReadSeat(ctx, nil, 1, false)
			require.NoError(t, err)
			require.Equal(t, seat.SessionID, original.SessionID)
		} else {
			moved, err := repo.ReadSeat(ctx, nil, 2, false)
			require.NoError(t, err)
			require.True(t, moved.CurrentSegmentStartedAt.Equal(boundary))
			require.Equal(t, "next synthetic", moved.WorkName)
		}
	}
}
