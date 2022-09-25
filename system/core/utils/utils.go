package utils

import (
	"app.modules/core/myfirestore"
	"context"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"image/color"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func JapanLocation() *time.Location {
	return time.FixedZone("Asia/Tokyo", 9*60*60)
}

// JstNow 日本時間におけるtime.Now()を返す。
func JstNow() time.Time {
	return time.Now().UTC().In(JapanLocation())
}

// SecondsOfDay tの0時0分からの経過時間（秒）
func SecondsOfDay(t time.Time) int {
	return t.Second() + int(time.Minute.Seconds())*t.Minute() + int(time.Hour.Seconds())*t.Hour()
}

func Get7daysBeforeJust0AM(date time.Time) time.Time {
	date7daysBefore := Get7daysBefore(date)
	return time.Date(
		date7daysBefore.Year(),
		date7daysBefore.Month(),
		date7daysBefore.Day(),
		0, 0, 0, 0,
		JapanLocation())
}

func Get7daysBefore(date time.Time) time.Time {
	return date.AddDate(0, 0, -7)
}

// LoadEnv TODO さらに上の階層に書くべき
func LoadEnv(relativeEnvPath string) {
	err := godotenv.Load(relativeEnvPath)
	if err != nil {
		log.Println(err.Error())
		log.Fatal("Error loading .env file")
	}
}

// SecondsToHours 秒を時間に換算。切り捨て。
func SecondsToHours(seconds int) int {
	duration := time.Duration(seconds) * time.Second
	return int(duration.Hours())
}

func IsColorCode(str string) bool {
	_, err := ParseHexColor(str)
	return err == nil
}

// ParseHexColor from https://stackoverflow.com/questions/54197913/parse-hex-string-to-image-color
func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		_, err = fmt.Sscanf(s, "#%01x%01x%01x", &c.R, &c.G, &c.B)
		// Double the hex digits:
		c.R *= 17
		c.G *= 17
		c.B *= 17
	default:
		err = fmt.Errorf("invalid length, must be 7 or 4")
	}
	return
}

// NumTrue from https://stackoverflow.com/questions/57983764/how-to-get-sum-of-true-bools
func NumTrue(b ...bool) int {
	n := 0
	for _, v := range b {
		if v {
			n++
		}
	}
	return n
}

// DateEqualJST from https://stackoverflow.com/questions/21053427/check-if-two-time-objects-are-on-the-same-date-in-go
func DateEqualJST(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.In(JapanLocation()).Date()
	y2, m2, d2 := date2.In(JapanLocation()).Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// DurationToString for Japanese.
func DurationToString(duration time.Duration) string {
	if duration < time.Hour {
		return strconv.Itoa(int(duration.Minutes())) + "分"
	} else {
		return strconv.Itoa(int(duration.Hours())) + "時間" + strconv.Itoa(int(duration.Minutes())%60) + "分"
	}
}

// NoNegativeDuration 負の値であれば0に修正する。
func NoNegativeDuration(duration time.Duration) time.Duration {
	if duration < 0 {
		return time.Duration(0)
	}
	return duration
}

func Ftoa(f float64) string {
	return strconv.Itoa(int(f))
}

func DivideStringEqually(batchSize int, values []string) [][]string {
	batchList := make([][]string, batchSize)
	for i, value := range values {
		index := i % batchSize
		batchList[index] = append(batchList[index], value)
	}
	return batchList
}

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

func ContainsString(s []string, e string) bool {
	contains, _ := ContainsStringWithFoundIndex(s, e)
	return contains
}

func ContainsStringWithFoundIndex(s []string, e string) (bool, int) {
	for i, a := range s {
		if a == e {
			return true, i
		}
	}
	return false, 0
}

func ContainsRegexWithFoundIndex(s []string, e string) (bool, int, error) {
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
	jstNow := JstNow()
	var duration time.Duration
	switch seat.State {
	case myfirestore.WorkState:
		duration = time.Duration(seat.CumulativeWorkSec)*time.Second + NoNegativeDuration(jstNow.Sub(seat.CurrentStateStartedAt))
	case myfirestore.BreakState:
		duration = time.Duration(seat.CumulativeWorkSec) * time.Second
	default:
		return 0, errors.New("unknown seat.State: " + string(seat.State))
	}
	return duration, nil
}

func RealTimeDailyTotalStudyDurationOfSeat(seat myfirestore.SeatDoc) (time.Duration, error) {
	jstNow := JstNow()
	var duration time.Duration
	// 今のstateになってから日付が変っている可能性
	if DateEqualJST(seat.CurrentStateStartedAt, jstNow) { // 日付変わってない
		switch seat.State {
		case myfirestore.WorkState:
			duration = time.Duration(seat.DailyCumulativeWorkSec)*time.Second + NoNegativeDuration(jstNow.Sub(seat.CurrentStateStartedAt))
		case myfirestore.BreakState:
			duration = time.Duration(seat.DailyCumulativeWorkSec) * time.Second
		default:
			return 0, errors.New("unknown seat.State: " + string(seat.State))
		}
	} else { // 日付変わってる
		switch seat.State {
		case myfirestore.WorkState:
			duration = time.Duration(SecondsOfDay(jstNow)) * time.Second
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
