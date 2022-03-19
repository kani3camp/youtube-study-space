package utils

import (
	"github.com/joho/godotenv"
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
	return t.Second() + int(time.Minute.Seconds()) * t.Minute() + int(time.Hour.Seconds()) * t.Hour()
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
