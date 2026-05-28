package mypage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return timeutil.JstNow()
}

type Service struct {
	store Store
	clock Clock
}

func NewService(store Store, clock Clock) *Service {
	if clock == nil {
		clock = realClock{}
	}

	return &Service{
		store: store,
		clock: clock,
	}
}

func (s *Service) GetMe(ctx context.Context, identity Identity) (Response, error) {
	if identity.YouTubeChannelID == "" {
		return Response{}, ErrInvalidIdentity
	}

	viewer := Viewer{
		YouTubeChannelID: identity.YouTubeChannelID,
		DisplayName:      identity.DisplayName,
		ProfileImageURL:  identity.ProfileImageURL,
	}

	user, err := s.store.ReadUser(ctx, identity.YouTubeChannelID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return Response{
				Status:      StatusNotRegistered,
				Viewer:      viewer,
				CurrentSeat: nil,
			}, nil
		}
		return Response{}, fmt.Errorf("read user: %w", err)
	}

	memberSeat, memberFound, err := s.readSeat(ctx, identity.YouTubeChannelID, true)
	if err != nil {
		return Response{}, err
	}

	generalSeat, generalFound, err := s.readSeat(ctx, identity.YouTubeChannelID, false)
	if err != nil {
		return Response{}, err
	}

	if memberFound && generalFound {
		return Response{}, errors.New("user is in both member and general seats")
	}

	totalSec := user.TotalStudySec
	dailySec := user.DailyTotalStudySec

	var currentSeat *CurrentSeat
	if memberFound || generalFound {
		seat := generalSeat
		isMemberSeat := false
		if memberFound {
			seat = memberSeat
			isMemberSeat = true
		}

		now := s.clock.Now()

		realtimeTotalDuration, err := utils.RealTimeTotalStudyDurationOfSeat(seat, now)
		if err != nil {
			return Response{}, fmt.Errorf("calculate realtime total study duration: %w", err)
		}

		realtimeDailyDuration, err := utils.RealTimeDailyTotalStudyDurationOfSeat(seat, now)
		if err != nil {
			return Response{}, fmt.Errorf("calculate realtime daily study duration: %w", err)
		}

		totalSec += int(realtimeTotalDuration.Seconds())
		dailySec += int(realtimeDailyDuration.Seconds())

		currentSeat = toCurrentSeat(seat, isMemberSeat)
	}

	return Response{
		Status: StatusOK,
		Viewer: viewer,
		Stats: &Stats{
			DailyWorkSec:      dailySec,
			CumulativeWorkSec: totalSec,
		},
		CurrentSeat: currentSeat,
	}, nil
}

func (s *Service) readSeat(ctx context.Context, youtubeChannelID string, isMemberSeat bool) (repository.SeatDoc, bool, error) {
	seat, err := s.store.ReadSeatByUserID(ctx, youtubeChannelID, isMemberSeat)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return repository.SeatDoc{}, false, nil
		}
		return repository.SeatDoc{}, false, fmt.Errorf("read seat by user id: isMemberSeat=%v: %w", isMemberSeat, err)
	}

	return seat, true, nil
}

func toCurrentSeat(seat repository.SeatDoc, isMemberSeat bool) *CurrentSeat {
	return &CurrentSeat{
		SeatID:        seat.SeatID,
		IsMemberSeat:  isMemberSeat,
		State:         string(seat.State),
		WorkName:      seat.WorkName,
		BreakWorkName: seat.BreakWorkName,
		StartedAt:     seat.CurrentStateStartedAt.Format(time.RFC3339),
		Until:         seat.CurrentStateUntil.Format(time.RFC3339),
	}
}
