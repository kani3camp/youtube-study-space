package repository

import (
	"time"
)

func (s *SeatDoc) RealtimeEntryDurationMin(now time.Time) time.Duration {
	return now.Sub(s.StartTime)
}

func (s *SeatDoc) RemainingWorkDuration(now time.Time) time.Duration {
	return s.EndTime.Sub(now)
}
