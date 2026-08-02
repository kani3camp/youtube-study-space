//go:build integration

package repository_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"
)

const testTimeZoneOffset = 9 * 60 * 60

func newTestRepository(t *testing.T) *repository.FirestoreControllerImplements {
	t.Helper()
	return integrationtest.NewFirestoreController(t)
}

func runTransaction(t *testing.T, controller *repository.FirestoreControllerImplements, fn func(context.Context, *firestore.Transaction) error) {
	t.Helper()
	err := controller.FirestoreClient().RunTransaction(context.Background(), fn)
	require.NoError(t, err)
}

func newSeatDoc(seatID int, userID string, sessionID string) repository.SeatDoc {
	jst := time.FixedZone("JST", testTimeZoneOffset)
	return repository.SeatDoc{
		SeatID:          seatID,
		UserID:          userID,
		SessionID:       sessionID,
		UserDisplayName: "テストユーザー",
		WorkName:        "初期作業",
		BreakWorkName:   "初期休憩作業",
		EnteredAt:       time.Date(2026, 8, 2, 9, 0, 0, 0, jst),
		Until:           time.Date(2026, 8, 2, 18, 0, 0, 0, jst),
		Appearance: repository.SeatAppearance{
			ColorCode1:           "#112233",
			ColorCode2:           "#445566",
			NumStars:             4,
			ColorGradientEnabled: true,
		},
		MenuCode:                "menu-initial",
		State:                   repository.WorkState,
		CurrentStateStartedAt:   time.Date(2026, 8, 2, 9, 0, 0, 0, jst),
		CurrentStateUntil:       time.Date(2026, 8, 2, 18, 0, 0, 0, jst),
		CurrentSegmentStartedAt: time.Date(2026, 8, 2, 9, 0, 0, 0, jst),
		CumulativeWorkSec:       3600,
		DailyCumulativeWorkSec:  1800,
		UserProfileImageURL:     "https://example.com/profile.png",
	}
}

func TestFirestoreRepository_SeatCreateAndRead(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := newTestRepository(t)
	want := newSeatDoc(3, "seat-create-user", "session-create")

	runTransaction(t, controller, func(_ context.Context, tx *firestore.Transaction) error {
		return controller.CreateSeat(tx, want, false)
	})

	got, err := controller.ReadSeat(context.Background(), nil, want.SeatID, false)
	require.NoError(t, err)
	assert.Equal(t, want.SeatID, got.SeatID)
	assert.Equal(t, want.UserID, got.UserID)
	assert.Equal(t, want.SessionID, got.SessionID)
	assert.Equal(t, want.UserDisplayName, got.UserDisplayName)
	assert.Equal(t, want.WorkName, got.WorkName)
	assert.Equal(t, want.BreakWorkName, got.BreakWorkName)
	assert.Equal(t, want.Appearance, got.Appearance)
	assert.Equal(t, want.State, got.State)
	assert.Equal(t, want.CumulativeWorkSec, got.CumulativeWorkSec)
	assert.Equal(t, want.DailyCumulativeWorkSec, got.DailyCumulativeWorkSec)
	assert.Equal(t, want.UserProfileImageURL, got.UserProfileImageURL)
	assert.Equal(t, want.EnteredAt.UTC(), got.EnteredAt)
	assert.Equal(t, want.Until.UTC(), got.Until)
	assert.Equal(t, want.CurrentStateStartedAt.UTC(), got.CurrentStateStartedAt)
	assert.Equal(t, want.CurrentStateUntil.UTC(), got.CurrentStateUntil)
	assert.Equal(t, want.CurrentSegmentStartedAt.UTC(), got.CurrentSegmentStartedAt)
}

func TestFirestoreRepository_SeatCollectionsAreSeparated(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := newTestRepository(t)
	generalSeat := newSeatDoc(5, "general-user", "general-session")
	memberSeat := newSeatDoc(5, "member-user", "member-session")
	memberSeat.WorkName = "メンバー作業"
	memberSeat.Appearance.ColorCode1 = "#abcdef"

	runTransaction(t, controller, func(_ context.Context, tx *firestore.Transaction) error {
		if err := controller.CreateSeat(tx, generalSeat, false); err != nil {
			return err
		}
		return controller.CreateSeat(tx, memberSeat, true)
	})

	gotGeneral, err := controller.ReadSeat(context.Background(), nil, generalSeat.SeatID, false)
	require.NoError(t, err)
	gotMember, err := controller.ReadSeat(context.Background(), nil, memberSeat.SeatID, true)
	require.NoError(t, err)

	assert.Equal(t, generalSeat.UserID, gotGeneral.UserID)
	assert.Equal(t, generalSeat.SessionID, gotGeneral.SessionID)
	assert.Equal(t, generalSeat.WorkName, gotGeneral.WorkName)
	assert.Equal(t, generalSeat.Appearance, gotGeneral.Appearance)
	assert.Equal(t, memberSeat.UserID, gotMember.UserID)
	assert.Equal(t, memberSeat.SessionID, gotMember.SessionID)
	assert.Equal(t, memberSeat.WorkName, gotMember.WorkName)
	assert.Equal(t, memberSeat.Appearance, gotMember.Appearance)
	assert.NotEqual(t, gotGeneral.UserID, gotMember.UserID)
	assert.NotEqual(t, gotGeneral.SessionID, gotMember.SessionID)
}

func TestFirestoreRepository_UpdateSeat(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := newTestRepository(t)
	original := newSeatDoc(7, "seat-update-user", "session-update")

	runTransaction(t, controller, func(_ context.Context, tx *firestore.Transaction) error {
		return controller.CreateSeat(tx, original, false)
	})

	updated := original
	updated.WorkName = "更新後の作業"
	updated.BreakWorkName = "更新後の休憩作業"
	updated.State = repository.BreakState
	updated.CurrentStateStartedAt = time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	updated.CurrentStateUntil = time.Date(2026, 8, 2, 12, 30, 0, 0, time.UTC)
	updated.CurrentSegmentStartedAt = time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	updated.CumulativeWorkSec = 7200
	updated.DailyCumulativeWorkSec = 5400
	updated.MenuCode = "menu-updated"

	runTransaction(t, controller, func(ctx context.Context, tx *firestore.Transaction) error {
		return controller.UpdateSeat(ctx, tx, updated, false)
	})

	got, err := controller.ReadSeat(context.Background(), nil, updated.SeatID, false)
	require.NoError(t, err)
	assert.Equal(t, updated.WorkName, got.WorkName)
	assert.Equal(t, updated.BreakWorkName, got.BreakWorkName)
	assert.Equal(t, updated.State, got.State)
	assert.Equal(t, updated.CurrentStateStartedAt, got.CurrentStateStartedAt)
	assert.Equal(t, updated.CurrentStateUntil, got.CurrentStateUntil)
	assert.Equal(t, updated.CurrentSegmentStartedAt, got.CurrentSegmentStartedAt)
	assert.Equal(t, updated.CumulativeWorkSec, got.CumulativeWorkSec)
	assert.Equal(t, updated.DailyCumulativeWorkSec, got.DailyCumulativeWorkSec)
	assert.Equal(t, updated.MenuCode, got.MenuCode)
	assert.Equal(t, original.SeatID, got.SeatID)
	assert.Equal(t, original.UserID, got.UserID)
	assert.Equal(t, original.SessionID, got.SessionID)
	assert.Equal(t, original.UserDisplayName, got.UserDisplayName)
	assert.Equal(t, original.EnteredAt.UTC(), got.EnteredAt)
	assert.Equal(t, original.Until.UTC(), got.Until)
	assert.Equal(t, original.Appearance, got.Appearance)
	assert.Equal(t, original.UserProfileImageURL, got.UserProfileImageURL)
}

func TestFirestoreRepository_DeleteSeat(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := newTestRepository(t)
	seat := newSeatDoc(9, "seat-delete-user", "session-delete")

	runTransaction(t, controller, func(_ context.Context, tx *firestore.Transaction) error {
		return controller.CreateSeat(tx, seat, false)
	})

	require.NoError(t, controller.DeleteSeat(context.Background(), nil, seat.SeatID, false))
	_, err := controller.ReadSeat(context.Background(), nil, seat.SeatID, false)
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))

	// Firestore delete is idempotent in the current repository implementation.
	assert.NoError(t, controller.DeleteSeat(context.Background(), nil, seat.SeatID, false))
}

func TestFirestoreRepository_UserCreateAndRead(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := newTestRepository(t)
	jst := time.FixedZone("JST", testTimeZoneOffset)
	userID := "user-create"
	want := repository.UserDoc{
		DailyTotalStudySec:          7200,
		TotalStudySec:               86400,
		RegistrationDate:            time.Date(2026, 7, 1, 9, 0, 0, 0, jst),
		StatusMessage:               "集中中",
		LastEntered:                 time.Date(2026, 8, 2, 8, 30, 0, 0, jst),
		LastExited:                  time.Date(2026, 8, 1, 18, 0, 0, 0, jst),
		RankVisible:                 true,
		DefaultStudyMin:             50,
		RankPoint:                   123,
		LastRPProcessed:             time.Date(2026, 8, 1, 0, 0, 0, 0, jst),
		LastPenaltyImposedDays:      2,
		IsContinuousActive:          true,
		CurrentActivityStateStarted: time.Date(2026, 7, 25, 0, 0, 0, 0, jst),
		FavoriteColor:               "#123456",
	}

	require.NoError(t, controller.CreateUser(context.Background(), nil, userID, want))
	got, err := controller.ReadUser(context.Background(), nil, userID)
	require.NoError(t, err)

	assert.Equal(t, want.DailyTotalStudySec, got.DailyTotalStudySec)
	assert.Equal(t, want.TotalStudySec, got.TotalStudySec)
	assert.Equal(t, want.StatusMessage, got.StatusMessage)
	assert.Equal(t, want.RankVisible, got.RankVisible)
	assert.Equal(t, want.DefaultStudyMin, got.DefaultStudyMin)
	assert.Equal(t, want.RankPoint, got.RankPoint)
	assert.Equal(t, want.LastPenaltyImposedDays, got.LastPenaltyImposedDays)
	assert.Equal(t, want.IsContinuousActive, got.IsContinuousActive)
	assert.Equal(t, want.FavoriteColor, got.FavoriteColor)
	assert.Equal(t, want.RegistrationDate.UTC(), got.RegistrationDate)
	assert.Equal(t, want.LastEntered.UTC(), got.LastEntered)
	assert.Equal(t, want.LastExited.UTC(), got.LastExited)
	assert.Equal(t, want.LastRPProcessed.UTC(), got.LastRPProcessed)
	assert.Equal(t, want.CurrentActivityStateStarted.UTC(), got.CurrentActivityStateStarted)
}

func TestFirestoreRepository_UserUpdates(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := newTestRepository(t)
	userID := "user-update"
	original := repository.UserDoc{
		DailyTotalStudySec:          100,
		TotalStudySec:               1000,
		RegistrationDate:            time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		StatusMessage:               "維持する設定",
		LastEntered:                 time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		LastExited:                  time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		RankVisible:                 false,
		DefaultStudyMin:             25,
		RankPoint:                   10,
		LastRPProcessed:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		LastPenaltyImposedDays:      1,
		IsContinuousActive:          true,
		CurrentActivityStateStarted: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		FavoriteColor:               "#abcdef",
	}
	require.NoError(t, controller.CreateUser(context.Background(), nil, userID, original))

	updatedEntered := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	updatedExited := time.Date(2026, 8, 1, 18, 0, 0, 0, time.UTC)
	runTransaction(t, controller, func(_ context.Context, tx *firestore.Transaction) error {
		if err := controller.UpdateUserTotalTime(tx, userID, 7200, 3600); err != nil {
			return err
		}
		if err := controller.UpdateUserLastEnteredDate(tx, userID, updatedEntered); err != nil {
			return err
		}
		if err := controller.UpdateUserLastExitedDate(tx, userID, updatedExited); err != nil {
			return err
		}
		if err := controller.UpdateUserRankVisible(tx, userID, true); err != nil {
			return err
		}
		if err := controller.UpdateUserDefaultStudyMin(tx, userID, 45); err != nil {
			return err
		}
		return controller.UpdateUserRankPoint(tx, userID, 99)
	})

	got, err := controller.ReadUser(context.Background(), nil, userID)
	require.NoError(t, err)
	assert.Equal(t, 7200, got.TotalStudySec)
	assert.Equal(t, 3600, got.DailyTotalStudySec)
	assert.Equal(t, updatedEntered, got.LastEntered)
	assert.Equal(t, updatedExited, got.LastExited)
	assert.True(t, got.RankVisible)
	assert.Equal(t, 45, got.DefaultStudyMin)
	assert.Equal(t, 99, got.RankPoint)
	assert.Equal(t, original.RegistrationDate, got.RegistrationDate)
	assert.Equal(t, original.StatusMessage, got.StatusMessage)
	assert.Equal(t, original.LastRPProcessed, got.LastRPProcessed)
	assert.Equal(t, original.LastPenaltyImposedDays, got.LastPenaltyImposedDays)
	assert.Equal(t, original.IsContinuousActive, got.IsContinuousActive)
	assert.Equal(t, original.CurrentActivityStateStarted, got.CurrentActivityStateStarted)
	assert.Equal(t, original.FavoriteColor, got.FavoriteColor)
}

func TestFirestoreRepository_UserActivityQuery(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := newTestRepository(t)
	jst := time.FixedZone("JST", testTimeZoneOffset)
	from := time.Date(2026, 8, 2, 10, 0, 0, 0, jst)
	targetUserID := "activity-target"
	activities := []repository.UserActivityDoc{
		{UserID: targetUserID, ActivityType: repository.EnterRoomActivity, SeatID: 4, IsMemberSeat: false, TakenAt: from.Add(20 * time.Minute)},
		{UserID: targetUserID, ActivityType: repository.EnterRoomActivity, SeatID: 4, IsMemberSeat: false, TakenAt: from.Add(10 * time.Minute)},
		{UserID: targetUserID, ActivityType: repository.ExitRoomActivity, SeatID: 4, IsMemberSeat: false, TakenAt: from.Add(30 * time.Minute)},
		{UserID: "another-user", ActivityType: repository.EnterRoomActivity, SeatID: 4, IsMemberSeat: false, TakenAt: from.Add(40 * time.Minute)},
		{UserID: targetUserID, ActivityType: repository.EnterRoomActivity, SeatID: 5, IsMemberSeat: false, TakenAt: from.Add(50 * time.Minute)},
		{UserID: targetUserID, ActivityType: repository.EnterRoomActivity, SeatID: 4, IsMemberSeat: true, TakenAt: from.Add(60 * time.Minute)},
		{UserID: targetUserID, ActivityType: repository.EnterRoomActivity, SeatID: 4, IsMemberSeat: false, TakenAt: from.Add(-time.Minute)},
	}
	for _, activity := range activities {
		require.NoError(t, controller.CreateUserActivityDoc(context.Background(), nil, activity))
	}

	gotEnters, err := controller.GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat(
		context.Background(), from, targetUserID, 4, false,
	)
	require.NoError(t, err)
	require.Len(t, gotEnters, 2)
	assert.Equal(t, activities[1].UserID, gotEnters[0].UserID)
	assert.Equal(t, activities[1].ActivityType, gotEnters[0].ActivityType)
	assert.Equal(t, activities[1].SeatID, gotEnters[0].SeatID)
	assert.Equal(t, activities[1].IsMemberSeat, gotEnters[0].IsMemberSeat)
	assert.Equal(t, activities[1].TakenAt.UTC(), gotEnters[0].TakenAt)
	assert.Equal(t, activities[0].TakenAt.UTC(), gotEnters[1].TakenAt)

	gotExits, err := controller.GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat(
		context.Background(), from, targetUserID, 4, false,
	)
	require.NoError(t, err)
	require.Len(t, gotExits, 1)
	assert.Equal(t, activities[2].ActivityType, gotExits[0].ActivityType)
	assert.Equal(t, activities[2].TakenAt.UTC(), gotExits[0].TakenAt)
}
