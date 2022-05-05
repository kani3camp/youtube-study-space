package utils

import (
	"fmt"
	"github.com/joho/godotenv"
	"image/color"
	"log"
	"strconv"
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

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println(err)
		err = godotenv.Load("../.env")
		if err != nil {
			log.Println(err.Error())
			log.Fatal("Error loading .env file")
		}
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

// DateEqual from https://stackoverflow.com/questions/21053427/check-if-two-time-objects-are-on-the-same-date-in-go
func DateEqual(date1, date2 time.Time) bool {
	y1, m1, d1 := date1.Date()
	y2, m2, d2 := date2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func DurationToString(duration time.Duration) string {
	if duration < time.Hour {
		return strconv.Itoa(int(duration.Minutes())) + "分"
	} else {
		return strconv.Itoa(int(duration.Hours())) + "時間" + strconv.Itoa(int(duration.Minutes())%60) + "分"
	}
}
