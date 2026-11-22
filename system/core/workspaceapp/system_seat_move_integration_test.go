//go:build integration

package workspaceapp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/i18n"
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/internal/integrationtest"
)

// recordingLiveChatBot はライブチャットへの投稿内容を記録する。
type recordingLiveChatBot struct {
	mu       sync.Mutex
	messages []string
}

func (b *recordingLiveChatBot) ListMessages(context.Context, string) ([]*youtube.LiveChatMessage, string, int, error) {
	return nil, "", 0, nil
}

func (b *recordingLiveChatBot) PostMessage(_ context.Context, message string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, message)
	return nil
}

func (b *recordingLiveChatBot) BanUser(context.Context, string) error { return nil }

func (b *recordingLiveChatBot) Messages() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.messages...)
}

type systemSeatMoveFixture struct {
	repo   *repository.FirestoreControllerImplements
	app    *WorkspaceApp
	bot    *recordingLiveChatBot
	userID string
	now    time.Time
}

func newSystemSeatMoveFixture(t *testing.T, constants repository.ConstantsConfigDoc, seat repository.SeatDoc) systemSeatMoveFixture {
	t.Helper()
	integrationtest.ResetFirestore(t)
	require.NoError(t, i18n.LoadLocaleFolderFS())
	repo := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, timeutil.JapanLocation())

	_, err := repo.FirestoreClient().Collection(repository.CONFIG).Doc(repository.SystemConstantsConfigDocName).Set(ctx, constants)
	require.NoError(t, err)
	require.NoError(t, repo.CreateUser(ctx, nil, seat.UserID, repository.UserDoc{IsContinuousActive: true, CurrentActivityStateStarted: seat.EnteredAt}))
	require.NoError(t, repo.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := repo.CreateSeat(tx, seat, false); err != nil {
			return fmt.Errorf("create fixture seat: %w", err)
		}
		if err := repo.CreateUserActivityDoc(ctx, tx, repository.UserActivityDoc{
			UserID:       seat.UserID,
			ActivityType: repository.EnterRoomActivity,
			SeatID:       seat.SeatID,
			TakenAt:      seat.EnteredAt,
		}); err != nil {
			return fmt.Errorf("create fixture enter activity: %w", err)
		}
		return nil
	}))

	bot := &recordingLiveChatBot{}
	app := &WorkspaceApp{
		Configs:       &Configs{Constants: constants},
		Repository:    repo,
		LiveChatBot:   bot,
		alertOwnerBot: moderatorbot.DummyMessageBot{},
		// 移動元のメニューコードは現在のメニュー一覧に存在しなくても引き継がれる。
		SortedMenuItems: []repository.MenuDoc{{Code: "another-menu", Name: "別メニュー"}},
		nowFunc:         func() time.Time { return now },
	}
	return systemSeatMoveFixture{repo: repo, app: app, bot: bot, userID: seat.UserID, now: now}
}

func (f systemSeatMoveFixture) requireMovedWithMenuAndNoOrder(t *testing.T, fromSeatID, wantSeatID int, wantMenuCode, wantWorkName string) {
	t.Helper()
	ctx := context.Background()
	_, err := f.repo.ReadSeat(ctx, nil, fromSeatID, false)
	require.Equal(t, codes.NotFound, status.Code(err), "移動元の座席が残っている")

	moved, err := f.repo.ReadSeatWithUserID(ctx, f.userID, false)
	require.NoError(t, err)
	assert.Equal(t, wantSeatID, moved.SeatID)
	assert.Equal(t, wantMenuCode, moved.MenuCode, "メニューを引き継ぐ")
	assert.Equal(t, wantWorkName, moved.WorkName)

	orders, err := f.repo.FirestoreClient().Collection(repository.OrderHistory).Where(repository.UserIDDocProperty, "==", f.userID).Documents(ctx).GetAll()
	require.NoError(t, err)
	assert.Empty(t, orders, "システム都合の席移動は注文履歴を作らない")
	count, err := f.repo.CountUserOrdersOfTheDay(ctx, f.userID, f.now)
	require.NoError(t, err)
	assert.Zero(t, count, "システム都合の席移動は当日の注文回数を増やさない")
}

func longSittingSeat(now time.Time, seatID int) repository.SeatDoc {
	enteredAt := now.Add(-100 * time.Minute)
	return repository.SeatDoc{
		SeatID:                  seatID,
		UserID:                  "UCsyntheticForceMove00001",
		SessionID:               "synthetic-force-move-session",
		UserDisplayName:         "長時間ユーザー",
		WorkName:                "読書",
		MenuCode:                "retired-menu",
		State:                   repository.WorkState,
		EnteredAt:               enteredAt,
		Until:                   now.Add(60 * time.Minute),
		CurrentStateStartedAt:   enteredAt,
		CurrentStateUntil:       now.Add(60 * time.Minute),
		CurrentSegmentStartedAt: enteredAt,
	}
}

// 長時間入室制限による強制席移動は、メニューを引き継ぎ、注文回数を増やさない。
func TestOrganizeDBForceMoveCarriesMenuWithoutCountingOrder(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, timeutil.JapanLocation())
	constants := repository.ConstantsConfigDoc{
		MaxSeats:                      3,
		MaxWorkTimeMin:                300,
		MaxDailyOrderCount:            1,
		RecentRangeMin:                180,
		RecentThresholdMin:            60,
		LongTimeSittingPenaltyMinutes: 30,
		MinimumCheckLongTimeSittingIntervalMinutes: 10,
	}
	seat := longSittingSeat(now, 1)
	f := newSystemSeatMoveFixture(t, constants, seat)
	ctx := context.Background()
	snapshot, err := f.repo.ReadSeat(ctx, nil, seat.SeatID, false)
	require.NoError(t, err)

	require.NoError(t, f.app.OrganizeDBForceMove(ctx, []repository.SeatDoc{snapshot}, false))

	f.requireMovedWithMenuAndNoOrder(t, seat.SeatID, 2, seat.MenuCode, seat.WorkName)
	blackList, err := f.repo.ReadSeatLimitsBLACKListWithSeatIDAndUserID(ctx, seat.SeatID, f.userID, false)
	require.NoError(t, err)
	assert.Len(t, blackList, 1)
}

// 座席数の削減に伴う席移動は、メニューを引き継ぎ、注文回数を増やさない。
func TestAdjustGeneralSeatsShrinkCarriesMenuWithoutCountingOrder(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, timeutil.JapanLocation())
	constants := repository.ConstantsConfigDoc{
		MaxSeats:           3,
		DesiredMaxSeats:    2,
		MaxWorkTimeMin:     300,
		MaxDailyOrderCount: 1,
		RecentRangeMin:     180,
		RecentThresholdMin: 600,
		MinimumCheckLongTimeSittingIntervalMinutes: 10,
	}
	seat := longSittingSeat(now, 3)
	f := newSystemSeatMoveFixture(t, constants, seat)
	ctx := context.Background()

	require.NoError(t, f.app.adjustGeneralSeats(ctx, constants))

	f.requireMovedWithMenuAndNoOrder(t, seat.SeatID, 1, seat.MenuCode, seat.WorkName)
	updated, err := f.repo.ReadSystemConstantsConfig(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, constants.DesiredMaxSeats, updated.MaxSeats)
}
