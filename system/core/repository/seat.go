package repository

import (
	"time"
)

func (s *SeatDoc) RealtimeEntryDurationMin(now time.Time) time.Duration {
	return now.Sub(s.EnteredAt)
}

func (s *SeatDoc) RemainingWorkDuration(now time.Time) time.Duration {
	return s.Until.Sub(now)
}
