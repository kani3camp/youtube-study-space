package mypage

import (
	"context"
	"errors"
	"testing"
	"time"

	"app.modules/core/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeStore struct {
	user    repository.UserDoc
	userErr error

	memberSeat    repository.SeatDoc
	memberSeatErr error

	generalSeat    repository.SeatDoc
	generalSeatErr error
}

func (s *fakeStore) ReadUser(_ context.Context, _ string) (repository.UserDoc, error) {
	return s.user, s.userErr
}

func (s *fakeStore) ReadSeatByUserID(_ context.Context, _ string, isMemberSeat bool) (repository.SeatDoc, error) {
	if isMemberSeat {
		return s.memberSeat, s.memberSeatErr
	}
	return s.generalSeat, s.generalSeatErr
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

func TestService_GetMe_NotRegistered(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeStore{
		userErr:        status.Error(codes.NotFound, "user not found"),
		memberSeatErr:  status.Error(codes.NotFound, "member seat not found"),
		generalSeatErr: status.Error(codes.NotFound, "general seat not found"),
	}, fakeClock{now: fixedJSTTime(2026, 5, 12, 10, 0, 0)})

	resp, err := svc.GetMe(context.Background(), testIdentity())

	require.NoError(t, err)
	assert.Equal(t, StatusNotRegistered, resp.Status)
	assert.Equal(t, "UCxxxxxxxxxxxxxxxxxxxxxx", resp.Viewer.YouTubeChannelID)
	assert.Nil(t, resp.Stats)
	assert.Nil(t, resp.CurrentSeat)
}

func TestService_GetMe_RegisteredAndNotInRoom(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeStore{
		user: repository.UserDoc{
			DailyTotalStudySec: 120,
			TotalStudySec:      3600,
		},
		memberSeatErr:  status.Error(codes.NotFound, "member seat not found"),
		generalSeatErr: status.Error(codes.NotFound, "general seat not found"),
	}, fakeClock{now: fixedJSTTime(2026, 5, 12, 10, 0, 0)})

	resp, err := svc.GetMe(context.Background(), testIdentity())

	require.NoError(t, err)
	assert.Equal(t, StatusOK, resp.Status)
	require.NotNil(t, resp.Stats)
	assert.Equal(t, 120, resp.Stats.DailyWorkSec)
	assert.Equal(t, 3600, resp.Stats.CumulativeWorkSec)
	assert.Nil(t, resp.CurrentSeat)
}

func TestService_GetMe_RegisteredAndWorkingInGeneralSeat(t *testing.T) {
	t.Parallel()

	startedAt := fixedJSTTime(2026, 5, 12, 10, 0, 0)
	now := fixedJSTTime(2026, 5, 12, 10, 10, 0)

	svc := NewService(&fakeStore{
		user: repository.UserDoc{
			DailyTotalStudySec: 100,
			TotalStudySec:      1000,
		},
		memberSeatErr: status.Error(codes.NotFound, "member seat not found"),
		generalSeat: repository.SeatDoc{
			SeatID:                  12,
			UserID:                  "UCxxxxxxxxxxxxxxxxxxxxxx",
			State:                   repository.WorkState,
			WorkName:                "Go API実装",
			BreakWorkName:           "",
			CurrentStateStartedAt:   startedAt,
			CurrentStateUntil:       fixedJSTTime(2026, 5, 12, 11, 0, 0),
			CumulativeWorkSec:       60,
			DailyCumulativeWorkSec:  30,
			CurrentSegmentStartedAt: startedAt,
		},
	}, fakeClock{now: now})

	resp, err := svc.GetMe(context.Background(), testIdentity())

	require.NoError(t, err)
	assert.Equal(t, StatusOK, resp.Status)
	require.NotNil(t, resp.Stats)

	// UserDoc.TotalStudySec 1000 + seat.CumulativeWorkSec 60 + 10min
	assert.Equal(t, 1660, resp.Stats.CumulativeWorkSec)

	// UserDoc.DailyTotalStudySec 100 + seat.DailyCumulativeWorkSec 30 + 10min
	assert.Equal(t, 730, resp.Stats.DailyWorkSec)

	require.NotNil(t, resp.CurrentSeat)
	assert.Equal(t, 12, resp.CurrentSeat.SeatID)
	assert.False(t, resp.CurrentSeat.IsMemberSeat)
	assert.Equal(t, "work", resp.CurrentSeat.State)
	assert.Equal(t, "Go API実装", resp.CurrentSeat.WorkName)
	assert.Equal(t, startedAt.Format(time.RFC3339), resp.CurrentSeat.StartedAt)
	assert.Equal(t, fixedJSTTime(2026, 5, 12, 11, 0, 0).Format(time.RFC3339), resp.CurrentSeat.Until)
}

func TestService_GetMe_RegisteredAndBreakingInMemberSeat(t *testing.T) {
	t.Parallel()

	startedAt := fixedJSTTime(2026, 5, 12, 10, 0, 0)
	now := fixedJSTTime(2026, 5, 12, 10, 10, 0)

	svc := NewService(&fakeStore{
		user: repository.UserDoc{
			DailyTotalStudySec: 100,
			TotalStudySec:      1000,
		},
		memberSeat: repository.SeatDoc{
			SeatID:                 3,
			UserID:                 "UCxxxxxxxxxxxxxxxxxxxxxx",
			State:                  repository.BreakState,
			WorkName:               "Go API実装",
			BreakWorkName:          "休憩",
			CurrentStateStartedAt:  startedAt,
			CurrentStateUntil:      fixedJSTTime(2026, 5, 12, 10, 30, 0),
			CumulativeWorkSec:      500,
			DailyCumulativeWorkSec: 50,
		},
		generalSeatErr: status.Error(codes.NotFound, "general seat not found"),
	}, fakeClock{now: now})

	resp, err := svc.GetMe(context.Background(), testIdentity())

	require.NoError(t, err)
	require.NotNil(t, resp.Stats)

	// break 中なので now-startedAt は足されない。
	assert.Equal(t, 1500, resp.Stats.CumulativeWorkSec)
	assert.Equal(t, 150, resp.Stats.DailyWorkSec)

	require.NotNil(t, resp.CurrentSeat)
	assert.Equal(t, 3, resp.CurrentSeat.SeatID)
	assert.True(t, resp.CurrentSeat.IsMemberSeat)
	assert.Equal(t, "break", resp.CurrentSeat.State)
	assert.Equal(t, "Go API実装", resp.CurrentSeat.WorkName)
	assert.Equal(t, "休憩", resp.CurrentSeat.BreakWorkName)
}

func TestService_GetMe_ReturnsErrorWhenUserIsInBothRooms(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeStore{
		user: repository.UserDoc{},
		memberSeat: repository.SeatDoc{
			SeatID: 1,
			UserID: "UCxxxxxxxxxxxxxxxxxxxxxx",
			State:  repository.WorkState,
		},
		generalSeat: repository.SeatDoc{
			SeatID: 2,
			UserID: "UCxxxxxxxxxxxxxxxxxxxxxx",
			State:  repository.WorkState,
		},
	}, fakeClock{now: fixedJSTTime(2026, 5, 12, 10, 0, 0)})

	_, err := svc.GetMe(context.Background(), testIdentity())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "both member and general seats")
}

func TestService_GetMe_ReturnsErrorWhenSeatStateIsUnknown(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeStore{
		user:          repository.UserDoc{},
		memberSeatErr: status.Error(codes.NotFound, "member seat not found"),
		generalSeat: repository.SeatDoc{
			SeatID: 1,
			UserID: "UCxxxxxxxxxxxxxxxxxxxxxx",
			State:  repository.SeatState("unknown"),
		},
	}, fakeClock{now: fixedJSTTime(2026, 5, 12, 10, 0, 0)})

	_, err := svc.GetMe(context.Background(), testIdentity())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown seat.State")
}

func TestService_GetMe_ReturnsErrorWhenIdentityDoesNotHaveYouTubeChannelID(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeStore{}, fakeClock{now: fixedJSTTime(2026, 5, 12, 10, 0, 0)})

	_, err := svc.GetMe(context.Background(), Identity{})

	require.ErrorIs(t, err, ErrInvalidIdentity)
}

func TestService_GetMe_ReturnsErrorWhenReadUserFails(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("firestore unavailable")
	svc := NewService(&fakeStore{
		userErr: expectedErr,
	}, fakeClock{now: fixedJSTTime(2026, 5, 12, 10, 0, 0)})

	_, err := svc.GetMe(context.Background(), testIdentity())

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func testIdentity() Identity {
	return Identity{
		FirebaseUID:      "firebase-user-id",
		YouTubeChannelID: "UCxxxxxxxxxxxxxxxxxxxxxx",
		DisplayName:      "テストユーザー",
		ProfileImageURL:  "https://example.com/profile.png",
	}
}

func fixedJSTTime(year int, month time.Month, day int, hour int, min int, sec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, 0, time.FixedZone("JST", 9*60*60))
}
