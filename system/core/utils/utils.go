package utils

import (
	"context"
	"log/slog"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"errors"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

// LoadEnv TODO さらに上の階層に書くべき
func LoadEnv(relativeEnvPath string) {
	if err := godotenv.Load(relativeEnvPath); err != nil {
		slog.Error("Error loading .env file")
		panic(err)
	}
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

func DivideStringEqually(batchSize int, values []string) [][]string {
	batchList := make([][]string, batchSize)
	for i, value := range values {
		index := i % batchSize
		batchList[index] = append(batchList[index], value)
	}
	return batchList
}

func GetSeatByUserID(seats []repository.SeatDoc, userID string) (repository.SeatDoc, error) {
	for _, seat := range seats {
		if seat.UserID == userID {
			return seat, nil
		}
	}
	return repository.SeatDoc{}, errors.New("no seat found with userID = " + userID)
}

func GetGcpProjectID(ctx context.Context, clientOption option.ClientOption) (string, error) {
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return "", err
	}
	return creds.ProjectID, nil
}

func Contains[T comparable](s []T, e T) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func ContainsRegexWithIndex(s []string, e string) (bool, int, error) {
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

func RealTimeTotalStudyDurationOfSeat(seat repository.SeatDoc, now time.Time) (time.Duration, error) {
	var duration time.Duration
	switch seat.State {
	case repository.WorkState:
		duration = time.Duration(seat.CumulativeWorkSec)*time.Second + timeutil.NoNegativeDuration(now.Sub(seat.CurrentStateStartedAt))
	case repository.BreakState:
		duration = time.Duration(seat.CumulativeWorkSec) * time.Second
	default:
		return 0, errors.New("unknown seat.State: " + string(seat.State))
	}
	return duration, nil
}

func RealTimeDailyTotalStudyDurationOfSeat(seat repository.SeatDoc, now time.Time) (time.Duration, error) {
	var duration time.Duration
	// 今のstateになってから日付が変っている可能性
	if timeutil.DateEqualJST(seat.CurrentStateStartedAt, now) { // 日付変わってない
		switch seat.State {
		case repository.WorkState:
			duration = time.Duration(seat.DailyCumulativeWorkSec)*time.Second + timeutil.NoNegativeDuration(now.Sub(seat.CurrentStateStartedAt))
		case repository.BreakState:
			duration = time.Duration(seat.DailyCumulativeWorkSec) * time.Second
		default:
			return 0, errors.New("unknown seat.State: " + string(seat.State))
		}
	} else { // 日付変わってる
		switch seat.State {
		case repository.WorkState:
			duration = time.Duration(timeutil.SecondsOfDay(now)) * time.Second
		case repository.BreakState:
			duration = time.Duration(0)
		}
	}
	return duration, nil
}

func SortUserActivityByTakenAtAscending(docs []repository.UserActivityDoc) {
	sort.Slice(docs, func(i, j int) bool { return docs[i].TakenAt.Before(docs[j].TakenAt) })
}

// CheckEnterExitActivityOrder 入室と退室が交互に並んでいるか確認する。
func CheckEnterExitActivityOrder(activityDocs []repository.UserActivityDoc) bool {
	var lastActivityType repository.UserActivityType
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

func MatchEmojiCommand(text string, commandName string) bool {
	r, err := regexp.Compile(EmojiCommandPrefix + `[0-9]*` + commandName + `[0-9]*` + EmojiSide)
	if err != nil {
		slog.Error("failed to compile regex in MatchEmojiCommand", "error", err, "commandName", commandName)
		return false
	}
	return r.MatchString(text)
}

// MatchEmojiCommandString partial match.
func MatchEmojiCommandString(text string) bool {
	return emojiCommandRegex.MatchString(text)
}

func NameOf(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// TruncateStringUTF8 は文字列をmaxBytesバイト以内にトランケートする。
// UTF-8のマルチバイト文字の途中で切れないよう、文字境界を考慮する。
func TruncateStringUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	// maxBytesバイト以内に収まる最後の有効なルーン境界を探す
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
}

// GenerateSessionID generates a UUID v4 string with hyphens removed (32 chars).
func GenerateSessionID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
