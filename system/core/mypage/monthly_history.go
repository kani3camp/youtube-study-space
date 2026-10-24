package mypage

import (
	"time"

	"app.modules/core/repository"
)

func monthlyMetric(s WorkSnapshot, start, end time.Time) Metric {
	if s.User.WorkHistoryEpoch != "" && !repository.ValidWorkHistoryWatermark(*s.User, s.AsOf) {
		return missingMetric(DataInconsistent)
	}
	if !validSeat(s) {
		return missingMetric(DataInconsistent)
	}
	complete := covered(s.Coverage, start, end)
	var total int64
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		month, exists := s.Months[day.Format("2006-01")]
		// This is an explicit transactional source watermark, not inference
		// from month creation time. No source interval can exist after the
		// closed-through bound verified at cutover and advanced by writers.
		verifiedEmpty := !exists && s.User.WorkHistoryEpoch == repository.MonthlyWorkEpoch &&
			!s.User.WorkHistoryActivatedAt.IsZero() && !day.Before(s.User.WorkHistoryActivatedAt) &&
			s.User.WorkHistoryClosedThrough.Before(day)
		if !verifiedEmpty && (!exists || !month.BaselineComplete || month.WriterEpoch != s.MonthlyEpoch) {
			complete = false
		}
		value, exists := month.Days[day.Format("2006-01-02")]
		if exists {
			if value.Invalid || value.Last.After(s.AsOf) || value.SourceEnd.After(s.AsOf) {
				return missingMetric(DataInconsistent)
			}
			if s.Seat != nil && value.Last.After(s.Seat.CurrentSegmentStartedAt) {
				return missingMetric(DataInconsistent)
			}
			total += value.WorkSec
			if start.Equal(StatisticsWindows(s.AsOf).Week) && end.Equal(s.AsOf) && end.Sub(start) >= 24*time.Hour {
				total += value.CrossDayCarrySec
			}
		}
	}
	if s.Seat != nil && s.Seat.State == repository.WorkState {
		total += secondsWithin(Interval{s.Seat.CurrentSegmentStartedAt, s.AsOf}, start, end)
	}
	if !complete {
		metric := missingMetric(HistoryIncomplete)
		if total > 0 {
			metric.RecordedWorkSec = ptr(total)
		}
		return metric
	}
	return valueMetric(total)
}
