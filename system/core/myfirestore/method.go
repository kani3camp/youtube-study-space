package myfirestore

import "time"

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
