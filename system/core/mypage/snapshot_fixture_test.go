package mypage

import (
	"testing"
	"time"

	"app.modules/core/repository"
)

// The same fixture is used by the Emulator query and fixed-clock boundary tests.
func snapshotQuerySegments(uid string, capturedAt time.Time) []repository.WorkSegmentDoc {
	end := capturedAt.Add(-time.Minute)
	return []repository.WorkSegmentDoc{
		{UserID: uid, StartedAt: end.Add(-time.Minute), EndedAt: end, DurationSec: 60, SegmentType: repository.WorkState},
		{UserID: "other-synthetic-user", StartedAt: end.Add(-time.Minute), EndedAt: end, DurationSec: 60, SegmentType: repository.WorkState},
		{UserID: uid, StartedAt: end.AddDate(0, 0, -30), EndedAt: end.AddDate(0, 0, -29), DurationSec: 86400, SegmentType: repository.WorkState},
	}
}

func expectRecentSeconds(t *testing.T, recent RecentSection, want int64) {
	t.Helper()
	if recent.Availability != Available || len(recent.Data) != 7 {
		t.Fatal("snapshot fixture did not produce seven available days")
	}
	var total int64
	for _, day := range recent.Data {
		if day.Availability != Available || day.WorkSec == nil {
			t.Fatalf("recent day %s is unavailable", day.Date)
		}
		total += *day.WorkSec
	}
	if total != want {
		t.Fatalf("recent total seconds=%d want=%d", total, want)
	}
}

func TestSnapshotQueryFixtureAccountsAcrossJSTMidnight(t *testing.T) {
	for _, tc := range []struct {
		name, capturedAt, asOf string
		today, yesterday       int64
	}{
		{"before-midnight", "2026-10-06T14:59:59Z", "2026-10-06T14:59:59Z", 60, 0},
		{"at-midnight", "2026-10-06T15:00:00Z", "2026-10-06T15:00:00Z", 0, 60},
		{"ci-failure-time", "2026-10-06T15:00:05Z", "2026-10-06T15:00:05Z", 0, 60},
		{"minute-straddles-midnight", "2026-10-06T15:01:30Z", "2026-10-06T15:01:30Z", 30, 30},
		{"after-midnight", "2026-10-06T15:02:00Z", "2026-10-06T15:02:00Z", 60, 0},
		{"read-crosses-midnight", "2026-10-06T14:59:59Z", "2026-10-06T15:00:05Z", 0, 60},
		{"monday-midnight", "2026-10-04T15:00:05Z", "2026-10-04T15:00:05Z", 0, 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := workFixture()
			s.AsOf = instant(tc.asOf)
			s.User.TotalStudySec = 600
			s.Coverage = []Interval{{s.AsOf.AddDate(0, 0, -30), s.AsOf}}
			// This is the matching, in-window segment returned by the query.
			s.Segments = snapshotQuerySegments(s.UID, instant(tc.capturedAt))[:1]
			r, err := Aggregate(s, fixtureAccount())
			if err != nil {
				t.Fatal(err)
			}
			expectSeconds(t, r.Summary.Data.Today, tc.today)
			expectSeconds(t, r.Recent7Days.Data[5].Metric, tc.yesterday)
			expectSeconds(t, r.Recent7Days.Data[6].Metric, tc.today)
			expectRecentSeconds(t, r.Recent7Days, 60)
			expectSeconds(t, r.Summary.Data.Lifetime, 600)
			if !r.GeneratedAt.Equal(s.AsOf) {
				t.Fatal("snapshot timestamp changed")
			}
		})
	}
}
