package core

import (
	"app.modules/core/myfirestore"
	"app.modules/core/utils"
	"context"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"strings"
	"time"
)

func HasWorkNameOptionPrefix(str string) bool {
	return strings.HasPrefix(str, WorkNameOptionPrefix) ||
		strings.HasPrefix(str, WorkNameOptionShortPrefix) ||
		strings.HasPrefix(str, WorkNameOptionPrefixLegacy) ||
		strings.HasPrefix(str, WorkNameOptionShortPrefixLegacy)
}

func TrimWorkNameOptionPrefix(str string) string {
	if strings.HasPrefix(str, WorkNameOptionPrefix) {
		return strings.TrimPrefix(str, WorkNameOptionPrefix)
	} else if strings.HasPrefix(str, WorkNameOptionShortPrefix) {
		return strings.TrimPrefix(str, WorkNameOptionShortPrefix)
	} else if strings.HasPrefix(str, WorkNameOptionPrefixLegacy) {
		return strings.TrimPrefix(str, WorkNameOptionPrefixLegacy)
	} else if strings.HasPrefix(str, WorkNameOptionShortPrefixLegacy) {
		return strings.TrimPrefix(str, WorkNameOptionShortPrefixLegacy)
	}
	return str
}

func HasTimeOptionPrefix(str string) bool {
	return strings.HasPrefix(str, TimeOptionPrefix) ||
		strings.HasPrefix(str, TimeOptionShortPrefix) ||
		strings.HasPrefix(str, TimeOptionPrefixLegacy) ||
		strings.HasPrefix(str, TimeOptionShortPrefixLegacy)
}

func IsEmptyTimeOption(str string) bool {
	return str == TimeOptionPrefix ||
		str == TimeOptionShortPrefix ||
		str == TimeOptionPrefixLegacy ||
		str == TimeOptionShortPrefixLegacy
}

func TrimTimeOptionPrefix(str string) string {
	if strings.HasPrefix(str, TimeOptionPrefix) {
		return strings.TrimPrefix(str, TimeOptionPrefix)
	} else if strings.HasPrefix(str, TimeOptionShortPrefix) {
		return strings.TrimPrefix(str, TimeOptionShortPrefix)
	} else if strings.HasPrefix(str, TimeOptionPrefixLegacy) {
		return strings.TrimPrefix(str, TimeOptionPrefixLegacy)
	} else if strings.HasPrefix(str, TimeOptionShortPrefixLegacy) {
		return strings.TrimPrefix(str, TimeOptionShortPrefixLegacy)
	}
	return str
}

func CreateUpdatedSeatsSeatWorkName(seats []myfirestore.Seat, workName string, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].WorkName = workName
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatBreakWorkName(seats []myfirestore.Seat, breakWorkName string, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].BreakWorkName = breakWorkName
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatAppearance(seats []myfirestore.Seat, newAppearance myfirestore.SeatAppearance, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].Appearance = newAppearance
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatUntil(seats []myfirestore.Seat, newUntil time.Time, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].Until = newUntil
			if seat.State == myfirestore.WorkState {
				seats[i].CurrentStateUntil = newUntil
			}
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatCurrentStateUntil(seats []myfirestore.Seat, newUntil time.Time, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].CurrentStateUntil = newUntil
			if seat.State == myfirestore.WorkState {
				seats[i].Until = newUntil
			}
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatState(seats []myfirestore.Seat, userId string, state myfirestore.SeatState,
	currentStateStartedAt time.Time, currentStateUntil time.Time, cumulativeWorkSec int, dailyCumulativeWorkSec int,
	workName string,
) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].State = state
			seats[i].CurrentStateStartedAt = currentStateStartedAt
			seats[i].CurrentStateUntil = currentStateUntil
			seats[i].CumulativeWorkSec = cumulativeWorkSec
			seats[i].DailyCumulativeWorkSec = dailyCumulativeWorkSec
			switch state {
			case myfirestore.BreakState:
				seats[i].BreakWorkName = workName
			case myfirestore.WorkState:
				seats[i].WorkName = workName
			}
			break
		}
	}
	return seats
}

func GetGcpProjectId(ctx context.Context, clientOption option.ClientOption) (string, error) {
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return "", err
	}
	return creds.ProjectID, nil
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func RealTimeTotalStudyDurationOfSeat(seat myfirestore.Seat) (time.Duration, error) {
	jstNow := utils.JstNow()
	var duration time.Duration
	switch seat.State {
	case myfirestore.WorkState:
		duration = time.Duration(seat.CumulativeWorkSec)*time.Second + utils.NoNegativeDuration(jstNow.Sub(seat.CurrentStateStartedAt))
	case myfirestore.BreakState:
		duration = time.Duration(seat.CumulativeWorkSec) * time.Second
	default:
		return 0, errors.New("unknown seat.State: " + string(seat.State))
	}
	return duration, nil
}

func RealTimeDailyTotalStudyDurationOfSeat(seat myfirestore.Seat) (time.Duration, error) {
	jstNow := utils.JstNow()
	var duration time.Duration
	// 今のstateになってから日付が変っている可能性
	if utils.DateEqual(seat.CurrentStateStartedAt, jstNow) { // 日付変わってない
		switch seat.State {
		case myfirestore.WorkState:
			duration = time.Duration(seat.DailyCumulativeWorkSec)*time.Second + utils.NoNegativeDuration(jstNow.Sub(seat.CurrentStateStartedAt))
		case myfirestore.BreakState:
			duration = time.Duration(seat.DailyCumulativeWorkSec) * time.Second
		default:
			return 0, errors.New("unknown seat.State: " + string(seat.State))
		}
	} else { // 日付変わってる
		switch seat.State {
		case myfirestore.WorkState:
			duration = time.Duration(utils.SecondsOfDay(jstNow)) * time.Second
		case myfirestore.BreakState:
			duration = time.Duration(0)
		}
	}
	return duration, nil
}
