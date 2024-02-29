package myfirestore

import (
	"time"
)

func (w *WorkHistoryDoc) WorkDurationOfDate(date time.Time) time.Duration {
	dateStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dateEnd := dateStart.AddDate(0, 0, 1)
	startedAt := w.StartedAt
	endedAt := w.EndedAt

	if startedAt.Before(dateStart) {
		startedAt = dateStart
	}
	if endedAt.After(dateEnd) {
		endedAt = dateEnd
	}
	if endedAt.Before(startedAt) {
		return 0
	}
	return endedAt.Sub(startedAt)
}

func (dw *DailyWorkHistoryDoc) WorkDuration() time.Duration {
	return time.Duration(dw.WorkSec) * time.Second
}

type DailyWorkSec struct {
	Date    string
	WorkSec int
}

func (w *WorkHistoryDoc) DivideToDailyWorkSecList(location *time.Location) []DailyWorkSec {
	var dailyWorkSecList []DailyWorkSec
	targetDay := w.StartedAt.In(location)
	endedAt := w.EndedAt.In(location)
	for targetDay.Before(endedAt) {
		nextDay := time.Date(targetDay.Year(), targetDay.Month(), targetDay.Day(), 0, 0, 0, 0, targetDay.Location()).AddDate(0, 0, 1)
		dailyWorkUntil := endedAt
		if nextDay.Before(endedAt) {
			dailyWorkUntil = nextDay
		}
		dailyWorkSecList = append(dailyWorkSecList, DailyWorkSec{
			Date:    targetDay.Format("2006-01-02"),
			WorkSec: int(dailyWorkUntil.Sub(targetDay).Seconds()),
		})
		targetDay = nextDay
	}
	return dailyWorkSecList
}

type DailyWorkSecList []DailyWorkSec

func SumEachDate(wl DailyWorkSecList) []DailyWorkSec {
	var dailyWorkHistoryList []DailyWorkSec
	var mapForSum = make(map[string]int)
	for _, dailyWorkSec := range wl {
		mapForSum[dailyWorkSec.Date] += dailyWorkSec.WorkSec
	}
	for date, workSec := range mapForSum {
		dailyWorkHistoryList = append(dailyWorkHistoryList, DailyWorkSec{
			Date:    date,
			WorkSec: workSec,
		})
	}
	return dailyWorkHistoryList
}

type WorkHistoryDocList []WorkHistoryDoc

func CreateDailyWorkSecList(wl WorkHistoryDocList, location *time.Location) []DailyWorkSec {
	var dailyWorkSecList []DailyWorkSec
	for _, wh := range wl {
		dailyWorkSecList = append(dailyWorkSecList, wh.DivideToDailyWorkSecList(location)...)
	}
	return SumEachDate(dailyWorkSecList)
}
