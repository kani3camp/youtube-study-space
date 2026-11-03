package workspaceapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"go.uber.org/mock/gomock"

	"app.modules/core/repository"
	mock_repository "app.modules/core/repository/mocks"
	"app.modules/core/timeutil"
)

func captureLegacyProcessLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logs
}

func requirePrivateValuesAbsentFromLogs(t *testing.T, logs string, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(logs, value) {
			t.Fatalf("process log contains synthetic private value: %s", logs)
		}
	}
}

func TestUpdateUserRPProcessLogsExcludeChannelAndWrappedError(t *testing.T) {
	const channelID = "PRIVATE_CHANNEL_ID_541"
	const credential = "PRIVATE_CREDENTIAL_751"
	previousErr := errors.New("read sentinel")
	readErr := fmt.Errorf("user=%s credential=%s: %w", channelID, credential, previousErr)
	now := time.Date(2026, time.October, 8, 10, 0, 0, 0, timeutil.JapanLocation())
	for _, tc := range []struct {
		name       string
		readUser   repository.UserDoc
		readError  error
		wantLog    string
		wantError  bool
		wantUpdate bool
	}{
		{name: "already processed", readUser: repository.UserDoc{LastRPProcessed: now}, wantLog: `"reason":"already_processed_today"`},
		{name: "updated", readUser: repository.UserDoc{
			LastRPProcessed: now.AddDate(0, 0, -1), LastEntered: now, LastExited: now,
			IsContinuousActive: true, CurrentActivityStateStarted: now.AddDate(0, 0, -1),
		}, wantLog: "RP update started", wantUpdate: true},
		{name: "read failed", readError: readErr, wantLog: "RP update started", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLegacyProcessLogs(t)
			ctrl := gomock.NewController(t)
			mockDB := mock_repository.NewMockRepository(ctrl)
			mockClient := mock_repository.NewMockDBClient(ctrl)
			mockDB.EXPECT().FirestoreClient().Return(mockClient)
			mockClient.EXPECT().RunTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, f func(context.Context, *firestore.Transaction) error, _ ...firestore.TransactionOption) error {
					return f(ctx, &firestore.Transaction{})
				},
			)
			mockDB.EXPECT().ReadUser(gomock.Any(), gomock.Any(), channelID).Return(tc.readUser, tc.readError)
			if tc.wantUpdate {
				mockDB.EXPECT().UpdateUserLastRPProcessed(gomock.Any(), channelID, now).Return(nil)
			}

			app := WorkspaceApp{Repository: mockDB}
			err := app.UpdateUserRP(context.Background(), channelID, now)
			if tc.wantError {
				if !errors.Is(err, previousErr) {
					t.Fatalf("read error identity lost: %v", err)
				}
			} else if err != nil {
				t.Fatalf("duplicate RP processing should skip: %v", err)
			}
			if !strings.Contains(logs.String(), tc.wantLog) {
				t.Fatalf("missing safe RP status: %s", logs.String())
			}
			if strings.Contains(logs.String(), `"userID"`) {
				t.Fatalf("RP log contains user identifier field: %s", logs.String())
			}
			requirePrivateValuesAbsentFromLogs(t, logs.String(), channelID, credential)
		})
	}
}

func TestSeatLimitProcessLogsExcludeChannelAndPrivateDurations(t *testing.T) {
	const channelID = "PRIVATE_CHANNEL_ID_542"
	now := time.Date(2026, time.October, 8, 10, 0, 0, 0, timeutil.JapanLocation())
	for _, tc := range []struct {
		name        string
		white       []repository.SeatLimitDoc
		black       []repository.SeatLimitDoc
		enter       []repository.UserActivityDoc
		wantLimited bool
		wantLog     string
		writeList   string
	}{
		{name: "active white list", white: []repository.SeatLimitDoc{{Until: now.Add(time.Hour)}}, wantLog: `"reason":"white_list_active"`},
		{name: "active black list", black: []repository.SeatLimitDoc{{Until: now.Add(time.Hour)}}, wantLimited: true, wantLog: `"reason":"black_list_active"`},
		{name: "new white list", wantLog: `"list":"white"`, writeList: "white"},
		{name: "new black list", enter: []repository.UserActivityDoc{{ActivityType: repository.EnterRoomActivity, TakenAt: now.Add(-20 * time.Minute)}}, wantLimited: true, wantLog: `"list":"black"`, writeList: "black"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLegacyProcessLogs(t)
			ctrl := gomock.NewController(t)
			mockDB := mock_repository.NewMockRepository(ctrl)
			mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIDAndUserID(gomock.Any(), 7, channelID, false).Return(tc.white, nil)
			mockDB.EXPECT().ReadSeatLimitsBLACKListWithSeatIDAndUserID(gomock.Any(), 7, channelID, false).Return(tc.black, nil)
			if tc.writeList != "" {
				mockDB.EXPECT().GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), channelID, 7, false).Return(tc.enter, nil)
				mockDB.EXPECT().GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat(gomock.Any(), gomock.Any(), channelID, 7, false).Return(nil, nil)
				if tc.writeList == "white" {
					mockDB.EXPECT().CreateSeatLimitInWHITEList(gomock.Any(), 7, channelID, gomock.Any(), gomock.Any(), false).Return(nil)
				} else {
					mockDB.EXPECT().CreateSeatLimitInBLACKList(gomock.Any(), 7, channelID, gomock.Any(), gomock.Any(), false).Return(nil)
				}
			}
			app := WorkspaceApp{
				Repository: mockDB,
				Configs: &Configs{Constants: repository.ConstantsConfigDoc{
					RecentRangeMin: 30, RecentThresholdMin: 15,
					MinimumCheckLongTimeSittingIntervalMinutes: 1, LongTimeSittingPenaltyMinutes: 5,
				}},
				nowFunc: func() time.Time { return now },
			}
			limited, err := app.CheckIfUserSittingTooMuchForSeat(context.Background(), channelID, 7, false)
			if err != nil || limited != tc.wantLimited {
				t.Fatalf("seat limit decision changed: limited=%v err=%v", limited, err)
			}
			if !strings.Contains(logs.String(), tc.wantLog) {
				t.Fatalf("missing safe seat limit status: %s", logs.String())
			}
			for _, field := range []string{`"userID"`, `"seatID"`, "過去何分", "合計何分"} {
				if strings.Contains(logs.String(), field) {
					t.Fatalf("seat limit log contains private detail: %s", logs.String())
				}
			}
			requirePrivateValuesAbsentFromLogs(t, logs.String(), channelID)
		})
	}
}

func TestSeatLimitFailureDoesNotLogWrappedPrivateError(t *testing.T) {
	const channelID = "PRIVATE_CHANNEL_ID_544"
	const credential = "PRIVATE_CREDENTIAL_754"
	cause := errors.New("repository sentinel")
	logs := captureLegacyProcessLogs(t)
	ctrl := gomock.NewController(t)
	mockDB := mock_repository.NewMockRepository(ctrl)
	mockDB.EXPECT().ReadSeatLimitsWHITEListWithSeatIDAndUserID(gomock.Any(), 7, channelID, false).
		Return(nil, fmt.Errorf("user=%s credential=%s: %w", channelID, credential, cause))
	app := WorkspaceApp{Repository: mockDB, nowFunc: func() time.Time { return time.Now() }}
	_, err := app.CheckIfUserSittingTooMuchForSeat(context.Background(), channelID, 7, false)
	if !errors.Is(err, cause) {
		t.Fatalf("repository error identity lost: %v", err)
	}
	requirePrivateValuesAbsentFromLogs(t, logs.String(), channelID, credential)
}

func TestExitRoomProcessLogsExcludeChannelWorkContentAndPrivateError(t *testing.T) {
	const channelID = "PRIVATE_CHANNEL_ID_545"
	const workContent = "PRIVATE_WORK_CONTENT_965"
	const credential = "PRIVATE_CREDENTIAL_755"
	deleteCause := errors.New("delete sentinel")
	now := time.Date(2026, time.October, 8, 10, 0, 0, 0, timeutil.JapanLocation())
	seat := repository.SeatDoc{
		SeatID: 7, UserID: channelID, UserDisplayName: "PRIVATE_DISPLAY_NAME_766", WorkName: workContent,
		State: repository.BreakState, CurrentStateStartedAt: now.Add(-time.Minute),
		CurrentSegmentStartedAt: now.Add(-time.Minute),
	}
	for _, tc := range []struct {
		name      string
		deleteErr error
	}{
		{name: "completed"},
		{name: "delete failed", deleteErr: fmt.Errorf("credential=%s: %w", credential, deleteCause)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLegacyProcessLogs(t)
			ctrl := gomock.NewController(t)
			mockDB := mock_repository.NewMockRepository(ctrl)
			mockDB.EXPECT().DeleteSeat(gomock.Any(), gomock.Any(), 7, false).Return(tc.deleteErr)
			if tc.deleteErr == nil {
				mockDB.EXPECT().CreateUserActivityDoc(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockDB.EXPECT().CreateWorkSegmentDoc(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ *firestore.Transaction, segment repository.WorkSegmentDoc) error {
						if segment.UserID != channelID || segment.WorkName != workContent {
							t.Fatalf("work segment content changed: %#v", segment)
						}
						return nil
					},
				)
				mockDB.EXPECT().UpdateUserLastExitedDate(gomock.Any(), channelID, now).Return(nil)
				mockDB.EXPECT().UpdateUserTotalTime(gomock.Any(), channelID, 0, 0).Return(nil)
				mockDB.EXPECT().UpdateUserRankPoint(gomock.Any(), channelID, 0).Return(nil)
			}
			app := WorkspaceApp{Repository: mockDB}
			_, _, err := app.exitRoomAt(context.Background(), &firestore.Transaction{}, false, seat, &repository.UserDoc{}, nil, now)
			if tc.deleteErr == nil {
				if err != nil || !strings.Contains(logs.String(), "room exit completed") || strings.Count(logs.String(), "work duration validation passed") != 2 {
					t.Fatalf("expected completed exit and safe validation events: err=%v logs=%s", err, logs.String())
				}
			} else if !errors.Is(err, deleteCause) {
				t.Fatalf("repository error identity lost on failed exit: %v", err)
			}
			for _, field := range []string{`"userID"`, `"seatID"`, `"addedWorkedTimeSec"`, `"newRP"`} {
				if strings.Contains(logs.String(), field) {
					t.Fatalf("room exit log contains private detail: %s", logs.String())
				}
			}
			requirePrivateValuesAbsentFromLogs(t, logs.String(), channelID, workContent, "PRIVATE_DISPLAY_NAME_766", credential)
		})
	}
}
