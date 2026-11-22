package workspaceapp

import (
	"context"
	"errors"
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
	mock_moderatorbot "app.modules/core/moderatorbot/mocks"
	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/studyspaceerror"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	mock_youtubebot "app.modules/core/youtubebot/mocks"
)

// retryingDBClient は Firestore のリトライを模擬する。クロージャを attempts 回実行し、
// 最後の試行の結果だけをトランザクションの結果とする（それ以前の試行はコミット時に競合で中断された扱い）。
// attempt には実行中の試行番号（1始まり）が入る。
func retryingDBClient(ctrl *gomock.Controller, attempts int, attempt *int) *mock_repository.MockDBClient {
	client := mock_repository.NewMockDBClient(ctrl)
	client.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
			var err error
			for i := 1; i <= attempts; i++ {
				if attempt != nil {
					*attempt = i
				}
				err = f(ctx, &firestore.Transaction{})
			}
			return err
		}).AnyTimes()
	return client
}

func TestRunTransaction_RunsAfterCommitHooksOnlyFromCommittedAttempt(t *testing.T) {
	ctrl := gomock.NewController(t)
	var attempt int
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockDB.EXPECT().FirestoreClient().Return(retryingDBClient(ctrl, 2, &attempt)).AnyTimes()
	app := &WorkspaceApp{Repository: mockDB}

	var executed []int
	err := app.RunTransaction(context.Background(), func(ctx context.Context, _ *firestore.Transaction) error {
		current := attempt
		runAfterCommit(ctx, func(context.Context) { executed = append(executed, current) })
		assert.Empty(t, executed, "コミット前に実行してはいけない")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int{2}, executed, "コミットされた最後の試行の登録分だけを1回実行する")
}

func TestRunTransaction_DiscardsAfterCommitHooksWhenTransactionFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockDB.EXPECT().FirestoreClient().Return(retryingDBClient(ctrl, 1, nil)).AnyTimes()
	app := &WorkspaceApp{Repository: mockDB}
	errSynthetic := errors.New("synthetic transaction failure")

	executed := false
	err := app.RunTransaction(context.Background(), func(ctx context.Context, _ *firestore.Transaction) error {
		runAfterCommit(ctx, func(context.Context) { executed = true })
		return errSynthetic
	})

	require.ErrorIs(t, err, errSynthetic)
	assert.False(t, executed)
}

func TestRunAfterCommit_RunsImmediatelyOutsideTransaction(t *testing.T) {
	executed := false
	runAfterCommit(context.Background(), func(context.Context) { executed = true })
	assert.True(t, executed)
}

// !change のトランザクションがリトライされても、返信は1回だけで、1回で成功したときと同じ内容になる。
func TestChange_RetriedTransactionRepliesOnceWithoutDuplicatedMessage(t *testing.T) {
	require.NoError(t, i18n.LoadLocaleFolderFS())
	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	const userID = "change-user"
	seat := repository.SeatDoc{
		SeatID:                  1,
		UserID:                  userID,
		State:                   repository.WorkState,
		WorkName:                "旧作業",
		EnteredAt:               fixedNow.Add(-30 * time.Minute),
		CurrentStateStartedAt:   fixedNow.Add(-30 * time.Minute),
		CurrentSegmentStartedAt: fixedNow.Add(-30 * time.Minute),
		Until:                   fixedNow.Add(30 * time.Minute),
	}
	run := func(t *testing.T, attempts int) []string {
		t.Helper()
		ctrl := gomock.NewController(t)
		mockDB := mock_repository.NewMockRepository(ctrl)
		mockDB.EXPECT().FirestoreClient().Return(retryingDBClient(ctrl, attempts, nil)).AnyTimes()
		mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, false).Return(seat, nil).AnyTimes()
		mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, true).Return(repository.SeatDoc{}, status.Error(codes.NotFound, "")).AnyTimes()
		mockDB.EXPECT().CreateWorkSegmentDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(attempts)
		mockDB.EXPECT().UpdateSeat(gomock.Any(), gomock.Any(), gomock.Any(), false).Return(nil).Times(attempts)
		var replies []string
		bot := mock_youtubebot.NewMockLiveChatBot(ctrl)
		bot.EXPECT().PostMessage(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, message string) error {
			replies = append(replies, message)
			return nil
		}).AnyTimes()
		app := &WorkspaceApp{
			Configs:                  &Configs{Constants: repository.ConstantsConfigDoc{MaxWorkTimeMin: 300}},
			Repository:               mockDB,
			LiveChatBot:              bot,
			alertOwnerBot:            moderatorbot.DummyMessageBot{},
			ProcessedUserID:          userID,
			ProcessedUserDisplayName: "テストユーザー",
			nowFunc:                  func() time.Time { return fixedNow },
		}
		option := utils.MinWorkOrderOption{IsWorkNameSet: true, WorkName: "新作業", IsDurationMinSet: true, DurationMin: 90}
		require.NoError(t, app.Change(context.Background(), &option))
		return replies
	}

	single := run(t, 1)
	retried := run(t, 2)

	require.Len(t, single, 1)
	assert.Equal(t, single, retried)
}

// 席番号0（最小番号の空席）での入室がリトライされた場合、席は試行ごとに選び直し、呼び出し元のオプションは書き換えない。
// 着席制限リストは即座に登録され、再試行の判定は登録済みのレコードを読むため、同じ座席へ重複登録しない。
func TestIn_RetriedTransactionReresolvesAutoSeatAndRegistersSeatLimitOnce(t *testing.T) {
	require.NoError(t, i18n.LoadLocaleFolderFS())
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	const userID = "auto-seat-user"
	constants := repository.ConstantsConfigDoc{
		MaxSeats:           3,
		RecentRangeMin:     180,
		RecentThresholdMin: 60,
		MinimumCheckLongTimeSittingIntervalMinutes: 10,
	}

	var attempt int
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockDB.EXPECT().FirestoreClient().Return(retryingDBClient(ctrl, 2, &attempt)).AnyTimes()
	mockDB.EXPECT().ReadSystemConstantsConfig(gomock.Any(), gomock.Any()).Return(constants, nil).AnyTimes()
	// 2回目の試行の時点では、1回目に選んだ1番席が他のユーザーに使われている
	mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).DoAndReturn(func(context.Context) ([]repository.SeatDoc, error) {
		if attempt == 1 {
			return nil, nil
		}
		return []repository.SeatDoc{{SeatID: 1, UserID: "other-user"}}, nil
	}).AnyTimes()
	mockDB.EXPECT().ReadSeat(gomock.Any(), gomock.Any(), 1, false).Return(repository.SeatDoc{SeatID: 1, UserID: "other-user"}, nil).AnyTimes()
	whiteList := map[int][]repository.SeatLimitDoc{}
	mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIDAndUserID(gomock.Any(), gomock.Any(), userID, false).DoAndReturn(func(_ context.Context, seatID int, _ string, _ bool) ([]repository.SeatLimitDoc, error) {
		return whiteList[seatID], nil
	}).AnyTimes()
	mockDB.EXPECT().ReadSeatLimitsBLACKListWithSeatIDAndUserID(gomock.Any(), gomock.Any(), userID, false).Return(nil, nil).AnyTimes()
	mockDB.EXPECT().GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), userID, gomock.Any(), false).Return(nil, nil).AnyTimes()
	mockDB.EXPECT().GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), userID, gomock.Any(), false).Return(nil, nil).AnyTimes()
	mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), userID).Return(repository.UserDoc{IsContinuousActive: true}, nil).AnyTimes()
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, gomock.Any()).Return(repository.SeatDoc{}, status.Error(codes.NotFound, "")).AnyTimes()
	var createdSeatIDs []int
	mockDB.EXPECT().CreateSeat(gomock.Any(), gomock.Any(), false).DoAndReturn(func(_ *firestore.Transaction, seat repository.SeatDoc, _ bool) error {
		createdSeatIDs = append(createdSeatIDs, seat.SeatID)
		return nil
	}).AnyTimes()
	mockDB.EXPECT().UpdateUserLastEnteredDate(gomock.Any(), userID, gomock.Any()).Return(nil).AnyTimes()
	mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockDB.EXPECT().CreateSeatLimitInWHITEList(gomock.Any(), gomock.Any(), userID, gomock.Any(), gomock.Any(), false).DoAndReturn(func(_ context.Context, seatID int, uid string, createdAt, until time.Time, _ bool) error {
		whiteList[seatID] = append(whiteList[seatID], repository.SeatLimitDoc{SeatID: seatID, UserID: uid, CreatedAt: createdAt, Until: until})
		return nil
	}).AnyTimes()

	bot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	bot.EXPECT().PostMessage(gomock.Any(), "@テストユーザー さんが作業を始めました🔥（作業内容：\"テスト作業\"、最大30分、2番席）").Return(nil).Times(1)

	app := &WorkspaceApp{
		Configs:                  &Configs{Constants: constants},
		Repository:               mockDB,
		LiveChatBot:              bot,
		alertOwnerBot:            moderatorbot.DummyMessageBot{},
		ProcessedUserID:          userID,
		ProcessedUserDisplayName: "テストユーザー",
		nowFunc:                  func() time.Time { return fixedNow },
	}
	option := utils.InOption{
		IsSeatIDSet:        true,
		SeatID:             0,
		MinWorkOrderOption: &utils.MinWorkOrderOption{IsWorkNameSet: true, WorkName: "テスト作業", IsDurationMinSet: true, DurationMin: 30},
	}

	require.NoError(t, app.In(context.Background(), &option))

	assert.Equal(t, []int{1, 2}, createdSeatIDs, "試行ごとに空席を選び直す")
	assert.Zero(t, option.SeatID, "呼び出し元のオプションを書き換えない")
	for seatID, docs := range whiteList {
		assert.Len(t, docs, 1, "座席%dのホワイトリストは1件だけ", seatID)
	}
	assert.Len(t, whiteList[2], 1, "入室した座席のホワイトリストが登録されている")
}

// 自動空席探索で全候補が長時間入室制限に該当し入室を拒否した（トランザクションがエラーで終わった）場合も、
// 判定で確定したブラックリスト登録は残る。
func TestIn_NoSeatAvailableBecauseOfSeatLimitStillSavesBlackList(t *testing.T) {
	require.NoError(t, i18n.LoadLocaleFolderFS())
	ctrl := gomock.NewController(t)
	fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
	const userID = "long-sitting-user"
	constants := repository.ConstantsConfigDoc{
		MaxSeats:                      1,
		RecentRangeMin:                180,
		RecentThresholdMin:            60,
		LongTimeSittingPenaltyMinutes: 30,
		MinimumCheckLongTimeSittingIntervalMinutes: 10,
	}

	mockDB := mock_repository.NewMockRepository(ctrl)
	mockDB.EXPECT().FirestoreClient().Return(retryingDBClient(ctrl, 1, nil)).AnyTimes()
	mockDB.EXPECT().ReadSystemConstantsConfig(gomock.Any(), gomock.Any()).Return(constants, nil).AnyTimes()
	mockDB.EXPECT().ReadGeneralSeats(gomock.Any()).Return(nil, nil).AnyTimes()
	mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIDAndUserID(gomock.Any(), 1, userID, false).Return(nil, nil).AnyTimes()
	mockDB.EXPECT().ReadSeatLimitsBLACKListWithSeatIDAndUserID(gomock.Any(), 1, userID, false).Return(nil, nil).AnyTimes()
	mockDB.EXPECT().GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), userID, 1, false).Return([]repository.UserActivityDoc{
		{ActivityType: repository.EnterRoomActivity, UserID: userID, SeatID: 1, TakenAt: fixedNow.Add(-120 * time.Minute)},
	}, nil).AnyTimes()
	mockDB.EXPECT().GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), userID, 1, false).Return([]repository.UserActivityDoc{
		{ActivityType: repository.ExitRoomActivity, UserID: userID, SeatID: 1, TakenAt: fixedNow.Add(-10 * time.Minute)},
	}, nil).AnyTimes()
	mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), userID).Return(repository.UserDoc{IsContinuousActive: true}, nil).AnyTimes()
	mockDB.EXPECT().ReadSeatWithUserID(gomock.Any(), userID, gomock.Any()).Return(repository.SeatDoc{}, status.Error(codes.NotFound, "")).AnyTimes()
	mockDB.EXPECT().CreateSeatLimitInBLACKList(gomock.Any(), 1, userID, gomock.Any(), gomock.Any(), false).Return(nil).Times(1)

	bot := mock_youtubebot.NewMockLiveChatBot(ctrl)
	bot.EXPECT().PostMessage(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	app := &WorkspaceApp{
		Configs:                  &Configs{Constants: constants},
		Repository:               mockDB,
		LiveChatBot:              bot,
		alertOwnerBot:            moderatorbot.DummyMessageBot{},
		ProcessedUserID:          userID,
		ProcessedUserDisplayName: "テストユーザー",
		nowFunc:                  func() time.Time { return fixedNow },
	}
	option := utils.InOption{
		IsSeatIDSet:        true,
		SeatID:             0,
		MinWorkOrderOption: &utils.MinWorkOrderOption{IsWorkNameSet: true, WorkName: "テスト作業", IsDurationMinSet: true, DurationMin: 30},
	}

	err := app.In(context.Background(), &option)
	require.ErrorIs(t, err, studyspaceerror.ErrNoSeatAvailable)
}

func blockTestSeat(now time.Time) repository.SeatDoc {
	return repository.SeatDoc{
		SeatID:                  4,
		UserID:                  "blocked-user",
		UserDisplayName:         "対象ユーザー",
		State:                   repository.WorkState,
		EnteredAt:               now.Add(-10 * time.Minute),
		CurrentStateStartedAt:   now.Add(-10 * time.Minute),
		CurrentSegmentStartedAt: now.Add(-10 * time.Minute),
		Until:                   now.Add(time.Hour),
	}
}

// !block のYouTubeブロックとモデレーターへのログは、退室がコミットされた後に1回だけ行う。
func TestBlock_BansAndLogsOnceAfterCommit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		exitErr error
		wantBan int
	}{
		{name: "リトライ後にコミット", wantBan: 1},
		{name: "退室のトランザクションが失敗", exitErr: errors.New("synthetic delete failure"), wantBan: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, i18n.LoadLocaleFolderFS())
			ctrl := gomock.NewController(t)
			fixedNow := time.Date(2026, time.January, 1, 10, 0, 0, 0, timeutil.JapanLocation())
			seat := blockTestSeat(fixedNow)
			attempts := 2
			if tc.exitErr != nil {
				attempts = 1
			}

			mockDB := mock_repository.NewMockRepository(ctrl)
			mockDB.EXPECT().FirestoreClient().Return(retryingDBClient(ctrl, attempts, nil)).AnyTimes()
			mockDB.EXPECT().ReadSeat(gomock.Any(), gomock.Any(), seat.SeatID, false).Return(seat, nil).AnyTimes()
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), seat.UserID).Return(repository.UserDoc{}, nil).AnyTimes()
			mockDB.EXPECT().ReadWorkStateSegmentsBySessionID(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mockDB.EXPECT().DeleteSeat(gomock.Any(), gomock.Any(), seat.SeatID, false).Return(tc.exitErr).AnyTimes()
			mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().CreateWorkSegmentDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserLastExitedDate(gomock.Any(), seat.UserID, gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserTotalTime(gomock.Any(), seat.UserID, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			mockDB.EXPECT().UpdateUserRankPoint(gomock.Any(), seat.UserID, gomock.Any()).Return(nil).AnyTimes()

			bot := mock_youtubebot.NewMockLiveChatBot(ctrl)
			bot.EXPECT().BanUser(gomock.Any(), seat.UserID).Return(nil).Times(tc.wantBan)
			bot.EXPECT().PostMessage(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			logBot := mock_moderatorbot.NewMockMessageBot(ctrl)
			logBot.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Return(nil).Times(tc.wantBan)

			app := &WorkspaceApp{
				Configs:                         &Configs{},
				Repository:                      mockDB,
				LiveChatBot:                     bot,
				alertOwnerBot:                   moderatorbot.DummyMessageBot{},
				logModeratorsBot:                logBot,
				ProcessedUserID:                 "moderator",
				ProcessedUserDisplayName:        "モデレーター",
				ProcessedUserIsModeratorOrOwner: true,
				nowFunc:                         func() time.Time { return fixedNow },
			}

			err := app.Block(context.Background(), &utils.BlockOption{SeatID: seat.SeatID})
			if tc.exitErr != nil {
				require.ErrorIs(t, err, tc.exitErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
