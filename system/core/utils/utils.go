package utils

import (
	"fmt"
	"github.com/joho/godotenv"
	"image/color"
	"log"
	"time"
)

func JapanLocation() *time.Location {
	return time.FixedZone("Asia/Tokyo", 9*60*60)
}

// JstNow 日本時間におけるtime.Now()を返す。
func JstNow() time.Time {
	return time.Now().UTC().In(JapanLocation())
}

func InSeconds(t time.Time) int {
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
