package mypage

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
)

func monthlyFixture(t *testing.T, s WorkSnapshot) WorkSnapshot {
	t.Helper()
	s.MonthlyEpoch = "synthetic-cutover-v1"
	s.Months = map[string]repository.WorkMonthDoc{}
	w := StatisticsWindows(s.AsOf)
	for day := w.QueryStart(); day.Before(s.AsOf); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01")
		s.Months[key] = repository.WorkMonthDoc{UserID: s.UID, Month: key, SchemaVersion: 1, BaselineComplete: true, WriterEpoch: s.MonthlyEpoch, Days: map[string]repository.WorkMonthDay{}}
	}
	for _, segment := range s.Segments {
		months, err := repository.WorkSegmentMonths(segment)
		require.NoError(t, err)
		for _, key := range months {
			m := s.Months[key]
			if m.Month == "" {
				continue
			}
			require.NoError(t, m.Append(segment))
			s.Months[key] = m
		}
	}
	s.Segments = nil
	return s
}

func TestMonthlyOverviewDoesNotDependOnRawHistoryCount(t *testing.T) {
	for _, count := range []int{0, 10, 100, 500, 1000, 1001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := workFixture()
			s.AsOf = time.Date(2026, 11, 1, 12, 0, 0, 0, timeutil.JapanLocation())
			s.Coverage = []Interval{{s.AsOf.AddDate(0, 0, -10), s.AsOf}}
			for i := 0; i < count; i++ {
				start := s.AsOf.Add(-time.Hour).Add(time.Duration(i) * time.Second)
				s.Segments = append(s.Segments, repository.WorkSegmentDoc{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: start, EndedAt: start.Add(time.Second), DurationSec: 1})
			}
			rollup := monthlyFixture(t, s)
			require.Len(t, rollup.Months, 2)
			response, err := Aggregate(rollup, fixtureAccount())
			require.NoError(t, err)
			expectSeconds(t, response.Summary.Data.Today, int64(count))
			expectSeconds(t, response.Summary.Data.Lifetime, 6000)
			if count < HistoryReadLimit {
				legacy, err := Aggregate(s, fixtureAccount())
				require.NoError(t, err)
				require.Equal(t, legacy, response)
			}
		})
	}
}

func TestMonthlyPreservesUnknownEpochCoverageAndInvalidState(t *testing.T) {
	for _, fault := range []string{"missing-month", "wrong-epoch", "unverified-baseline", "unknown-coverage", "invalid-day", "future-day", "seat-failure"} {
		t.Run(fault, func(t *testing.T) {
			s := monthlyFixture(t, workFixture())
			key := s.AsOf.In(timeutil.JapanLocation()).Format("2006-01")
			m := s.Months[key]
			switch fault {
			case "missing-month":
				delete(s.Months, key)
			case "wrong-epoch":
				m.WriterEpoch = "old"
				s.Months[key] = m
			case "unverified-baseline":
				m.BaselineComplete = false
				s.Months[key] = m
			case "unknown-coverage":
				s.Coverage = nil
			case "invalid-day":
				m.Days[s.AsOf.In(timeutil.JapanLocation()).Format("2006-01-02")] = repository.WorkMonthDay{Invalid: true}
				s.Months[key] = m
			case "future-day":
				m.Days[s.AsOf.In(timeutil.JapanLocation()).Format("2006-01-02")] = repository.WorkMonthDay{Last: s.AsOf.Add(time.Second)}
				s.Months[key] = m
			case "seat-failure":
				s.SeatUnavailable = true
			}
			response, err := Aggregate(s, fixtureAccount())
			require.NoError(t, err)
			require.True(t, response.Partial)
			if response.Summary.Data != nil {
				require.Equal(t, Unavailable, response.Summary.Data.Today.Availability)
				require.Nil(t, response.Summary.Data.Today.WorkSec)
			}
		})
	}
}

func TestMonthlyMatchesRawAtJSTMonthBoundaryWithOngoingWorkAndBreak(t *testing.T) {
	for _, state := range []repository.SeatState{repository.WorkState, repository.BreakState} {
		t.Run(string(state), func(t *testing.T) {
			s := workFixture()
			s.AsOf = time.Date(2026, 11, 1, 0, 15, 0, 0, timeutil.JapanLocation())
			s.Coverage = []Interval{{s.AsOf.AddDate(0, 0, -10), s.AsOf}}
			start := s.AsOf.Add(-30 * time.Minute)
			boundary := s.AsOf.Add(-5 * time.Minute)
			s.Segments = []repository.WorkSegmentDoc{{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: start, EndedAt: boundary, DurationSec: 1500}}
			s.Seat = &repository.SeatDoc{UserID: s.UID, SeatID: 1, State: state, EnteredAt: start, CurrentStateStartedAt: boundary, CurrentSegmentStartedAt: boundary, CumulativeWorkSec: 1500}
			legacy, err := Aggregate(s, fixtureAccount())
			require.NoError(t, err)
			monthly, err := Aggregate(monthlyFixture(t, s), fixtureAccount())
			require.NoError(t, err)
			require.Equal(t, legacy, monthly)
		})
	}
}

func TestMonthlyPreservesRawWeeklySecondFloorAcrossFractionalBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, start, end, asOf string }{
		{"same-week", "2026-10-26T23:59:59.5+09:00", "2026-10-27T00:00:00.5+09:00", "2026-10-27T01:00:00+09:00"},
		{"month", "2026-10-31T23:59:59.5+09:00", "2026-11-01T00:00:00.5+09:00", "2026-11-01T01:00:00+09:00"},
		{"week", "2026-11-01T23:59:59.5+09:00", "2026-11-02T00:00:00.5+09:00", "2026-11-02T01:00:00+09:00"},
		{"many-days", "2026-10-26T23:59:59.5+09:00", "2026-10-29T00:00:00.5+09:00", "2026-10-29T01:00:00+09:00"},
		{"different-fractions", "2026-10-26T23:59:59.8+09:00", "2026-10-27T00:00:00.2+09:00", "2026-10-27T01:00:00+09:00"},
		{"exact-end", "2026-10-26T23:59:59.5+09:00", "2026-10-27T00:00:00+09:00", "2026-10-27T01:00:00+09:00"},
		{"future-end-across-month", "2026-10-31T23:59:59.5+09:00", "2026-11-01T12:00:00.5+09:00", "2026-11-01T12:00:00+09:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := workFixture()
			s.AsOf = instant(tc.asOf)
			s.Coverage = []Interval{{s.AsOf.AddDate(0, 0, -10), s.AsOf}}
			start, end := instant(tc.start), instant(tc.end)
			s.Segments = []repository.WorkSegmentDoc{{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: start, EndedAt: end, DurationSec: int(end.Sub(start) / time.Second)}}
			legacy, err := Aggregate(s, fixtureAccount())
			require.NoError(t, err)
			monthly, err := Aggregate(monthlyFixture(t, s), fixtureAccount())
			require.NoError(t, err)
			require.Equal(t, legacy, monthly)
		})
	}
}

func TestMonthlyVerifiedEmptyWatermarkDoesNotHideMissingRecordedMonth(t *testing.T) {
	s := workFixture()
	s.AsOf = instant("2026-11-02T03:00:00Z")
	s.Coverage = []Interval{{s.AsOf.AddDate(0, 0, -10), s.AsOf}}
	s.MonthlyEpoch = repository.MonthlyWorkEpoch
	s.User.WorkHistoryEpoch = repository.MonthlyWorkEpoch
	s.User.WorkHistoryActivatedAt = instant("2026-10-20T03:00:00Z")
	s.User.WorkHistoryClosedThrough = s.User.WorkHistoryActivatedAt
	response, err := Aggregate(s, fixtureAccount())
	require.NoError(t, err)
	require.False(t, response.Partial)
	expectSeconds(t, response.Summary.Data.Today, 0)
	s.User.WorkHistoryClosedThrough = instant("2026-11-02T02:00:00Z")
	response, err = Aggregate(s, fixtureAccount())
	require.NoError(t, err)
	require.True(t, response.Partial)
	require.Nil(t, response.Summary.Data.Today.WorkSec)
}

func TestMonthlyInvalidWatermarkNeverTurnsMissingMonthsIntoNormalZero(t *testing.T) {
	for _, watermark := range []time.Time{{}, instant("2026-10-19T03:00:00Z")} {
		s := workFixture()
		s.AsOf = instant("2026-11-02T03:00:00Z")
		s.Coverage = []Interval{{s.AsOf.AddDate(0, 0, -10), s.AsOf}}
		s.MonthlyEpoch = repository.MonthlyWorkEpoch
		s.User.WorkHistoryEpoch = repository.MonthlyWorkEpoch
		s.User.WorkHistoryActivatedAt = instant("2026-10-20T03:00:00Z")
		s.User.WorkHistoryClosedThrough = watermark
		r, err := Aggregate(s, fixtureAccount())
		require.NoError(t, err)
		require.True(t, r.Partial)
		require.Nil(t, r.Summary.Data.Today.WorkSec)
		require.Equal(t, DataInconsistent, *r.Summary.Data.Today.ReasonCode)
	}
}
