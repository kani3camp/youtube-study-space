package utils

import "time"

func JapanLocation() *time.Location {
	return time.FixedZone("Asia/Tokyo", 9*60*60)
}

// JstNow 日本時間におけるtime.Now()を返す。
func JstNow() time.Time {
	return time.Now().UTC().In(JapanLocation())
}

func InSeconds(t time.Time) int {
	return t.Second() + int(time.Minute.Seconds()) * t.Minute() + int(time.Hour.Seconds()) * t.Hour()
}
