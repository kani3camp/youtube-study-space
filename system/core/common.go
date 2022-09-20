package core

import (
	"app.modules/core/myfirestore"
	"app.modules/core/utils"
	"context"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"regexp"
	"sort"
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

func GetSeatByUserId(seats []myfirestore.SeatDoc, userId string) (myfirestore.SeatDoc, error) {
	for _, seat := range seats {
		if seat.UserId == userId {
			return seat, nil
		}
	}
	return myfirestore.SeatDoc{}, errors.New("no seat found with user id = " + userId)
}

func GetGcpProjectId(ctx context.Context, clientOption option.ClientOption) (string, error) {
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return "", err
	}
	return creds.ProjectID, nil
}

func containsInt(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func containsString(s []string, e string) bool {
	contains, _ := containsStringWithFoundIndex(s, e)
	return contains
}

func containsStringWithFoundIndex(s []string, e string) (bool, int) {
	for i, a := range s {
		if a == e {
			return true, i
		}
	}
	return false, 0
}

func containsRegexWithFoundIndex(s []string, e string) (bool, int, error) {
	for i, a := range s {
		r, err := regexp.Compile(a)
		if err != nil {
			return false, 0, err
		}
		if r.MatchString(e) {
			return true, i, nil
		}
	}
	return false, 0, nil
}

func RealTimeTotalStudyDurationOfSeat(seat myfirestore.SeatDoc) (time.Duration, error) {
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

func RealTimeDailyTotalStudyDurationOfSeat(seat myfirestore.SeatDoc) (time.Duration, error) {
	jstNow := utils.JstNow()
	var duration time.Duration
	// 今のstateになってから日付が変っている可能性
	if utils.DateEqualJST(seat.CurrentStateStartedAt, jstNow) { // 日付変わってない
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

func SortUserActivityByTakenAtAscending(docs []myfirestore.UserActivityDoc) {
	sort.Slice(docs, func(i, j int) bool { return docs[i].TakenAt.Before(docs[j].TakenAt) })
}

// CheckEnterExitActivityOrder 入室と退室が交互に並んでいるか確認する。
func CheckEnterExitActivityOrder(activityDocs []myfirestore.UserActivityDoc) bool {
	var lastActivityType myfirestore.UserActivityType
	for i, activity := range activityDocs {
		if i == 0 {
			lastActivityType = activity.ActivityType
			continue
		}
		if activity.ActivityType == lastActivityType {
			return false
		}
		lastActivityType = activity.ActivityType
	}
	return true
}
