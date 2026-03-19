package timeutil

import (
	"strconv"
	"time"
)

// JapanLocation returns the Japan time zone (JST/UTC+9).
func JapanLocation() *time.Location {
	return time.FixedZone("Asia/Tokyo", 9*60*60)
}

// JstNow returns the current time in Japan Standard Time.
func JstNow() time.Time {
	return time.Now().UTC().In(JapanLocation())
}

// SecondsOfDay returns the number of seconds elapsed since midnight (00:00:00)
// for the given time.
func SecondsOfDay(t time.Time) int {
	return t.Second() + int(time.Minute.Seconds())*t.Minute() + int(time.Hour.Seconds())*t.Hour()
}

// SecondsToHours converts seconds to hours (truncated).
func SecondsToHours(seconds int) int {
	duration := time.Duration(seconds) * time.Second
	return int(duration.Hours())
}

// DateEqualJST checks if two times are on the same date in JST.
// From https://stackoverflow.com/questions/21053427/check-if-two-time-objects-are-on-the-same-date-in-go
func DateEqualJST(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.In(JapanLocation()).Date()
	y2, m2, d2 := date2.In(JapanLocation()).Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// DurationToString converts a duration to a Japanese string representation.
// TODO: support other languages using i18n
func DurationToString(duration time.Duration) string {
	if duration < time.Hour {
		return strconv.Itoa(int(duration.Minutes())) + "分"
	} else {
		return strconv.Itoa(int(duration.Hours())) + "時間" + strconv.Itoa(int(duration.Minutes())%60) + "分"
	}
}

// NoNegativeDuration returns 0 if the duration is negative, otherwise returns the duration as-is.
func NoNegativeDuration(duration time.Duration) time.Duration {
	if duration < 0 {
		return time.Duration(0)
	}
	return duration
}
