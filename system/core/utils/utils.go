package utils

import "time"

// JstNow 日本時間におけるtime.Now()を返す。
//
func JstNow() time.Time {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	return time.Now().UTC().In(jst)
}
