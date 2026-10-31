package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/timeutil"
)

func TestWorkMonthSplitsHalfOpenIntervalsAndPreservesSecondRounding(t *testing.T) {
	boundary := time.Date(2026, 11, 1, 0, 0, 0, 0, timeutil.JapanLocation())
	s := WorkSegmentDoc{UserID: "synthetic", SegmentType: WorkState, StartedAt: boundary.Add(-30 * time.Second), EndedAt: boundary.Add(30 * time.Second), DurationSec: 60}
	months, err := WorkSegmentMonths(s)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-10", "2026-11"}, months)
	for _, key := range months {
		m := WorkMonthDoc{UserID: s.UserID, Month: key, SchemaVersion: 1}
		require.NoError(t, m.Append(s))
		require.NoError(t, m.Validate(s.UserID, key))
		require.Len(t, m.Days, 1)
		for _, day := range m.Days {
			require.Equal(t, int64(30), day.WorkSec)
			require.False(t, day.Invalid)
		}
	}
	s.EndedAt = boundary
	s.DurationSec = 30
	months, err = WorkSegmentMonths(s)
	require.NoError(t, err)
	require.Equal(t, []string{"2026-10"}, months)
}

func TestWorkMonthCompletionCountUsesEndDayWithoutAddingMidnightWork(t *testing.T) {
	boundary := time.Date(2026, 11, 1, 0, 0, 0, 0, timeutil.JapanLocation())
	for _, state := range []SeatState{WorkState, BreakState} {
		s := WorkSegmentDoc{UserID: "synthetic", SegmentType: state, StartedAt: boundary.Add(-time.Minute), EndedAt: boundary, DurationSec: 60}
		before := WorkMonthDoc{UserID: s.UserID, Month: "2026-10", SchemaVersion: 1}
		after := WorkMonthDoc{UserID: s.UserID, Month: "2026-11", SchemaVersion: 1}
		require.NoError(t, before.Append(s))
		require.NoError(t, after.Append(s))
		require.NoError(t, after.Validate(s.UserID, after.Month))
		require.Zero(t, before.Days["2026-10-31"].CompletedWorkSegments)
		require.Zero(t, after.Days["2026-11-01"].WorkSec)
		if state == WorkState {
			require.Equal(t, int64(1), after.Days["2026-11-01"].CompletedWorkSegments)
		} else {
			require.Zero(t, after.Days["2026-11-01"].CompletedWorkSegments)
		}
		require.True(t, after.Days["2026-11-01"].SourceEnd.Equal(boundary))
	}
}

func TestWorkMonthNeverTreatsDelayedOverlappingOrMalformedIntervalsAsExact(t *testing.T) {
	base := time.Date(2026, 10, 24, 8, 0, 0, 0, timeutil.JapanLocation())
	for _, fault := range []string{"overlap", "late", "bad-duration", "zero", "bad-type"} {
		t.Run(fault, func(t *testing.T) {
			m := WorkMonthDoc{UserID: "synthetic", Month: "2026-10", SchemaVersion: 1}
			s := WorkSegmentDoc{UserID: m.UserID, SegmentType: WorkState, StartedAt: base, EndedAt: base.Add(time.Hour), DurationSec: 3600}
			require.NoError(t, m.Append(s))
			s.StartedAt = s.EndedAt
			s.EndedAt = s.StartedAt.Add(time.Hour)
			switch fault {
			case "overlap":
				s.StartedAt = s.StartedAt.Add(-time.Minute)
				s.DurationSec = 3660
			case "late":
				s.StartedAt = base.Add(-2 * time.Hour)
				s.EndedAt = base.Add(-time.Hour)
			case "bad-duration":
				s.DurationSec = -1
			case "zero":
				s.EndedAt = s.StartedAt
				s.DurationSec = 0
			case "bad-type":
				s.SegmentType = "invalid"
			}
			require.NoError(t, m.Append(s))
			require.True(t, m.Days["2026-10-24"].Invalid)
		})
	}
}
