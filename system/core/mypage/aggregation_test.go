package mypage

import (
	"testing"
	"time"

	"app.modules/core/repository"
)

func instant(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}

func workFixture() WorkSnapshot {
	asOf := instant("2026-10-06T03:00:00Z")
	return WorkSnapshot{AsOf: asOf, UID: "synthetic", User: &repository.UserDoc{TotalStudySec: 6000}}
}

func fixtureAccount() Section[Account] { return availableSection(Account{DisplayName: "Sample"}) }

func expectSeconds(t *testing.T, m Metric, want int64) {
	t.Helper()
	if m.Availability != Available || m.WorkSec == nil || *m.WorkSec != want {
		t.Fatalf("metric=%+v want seconds=%d", m, want)
	}
}

func TestAggregationUsesSeparateSegmentAndLifetimeClocks(t *testing.T) {
	s := workFixture()
	s.Seat = &repository.SeatDoc{UserID: s.UID, SeatID: 1, State: repository.WorkState, WorkName: "new name", EnteredAt: s.AsOf.Add(-3 * time.Hour), CurrentStateStartedAt: s.AsOf.Add(-time.Hour), CurrentSegmentStartedAt: s.AsOf.Add(-30 * time.Minute), CumulativeWorkSec: 3600}
	s.Segments = []repository.WorkSegmentDoc{{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: s.AsOf.Add(-time.Hour), EndedAt: s.AsOf.Add(-30 * time.Minute), DurationSec: 1800}}
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectSeconds(t, r.Summary.Data.Today, 3600)
	expectSeconds(t, r.Summary.Data.Lifetime, 13200)
	if r.Partial || *r.Current.Data.WorkName != "new name" {
		t.Fatal("valid work snapshot became partial")
	}
}

func TestAggregationBreakKeepsClosedSessionWorkInLifetime(t *testing.T) {
	s := workFixture()
	s.Seat = &repository.SeatDoc{UserID: s.UID, SeatID: 1, State: repository.BreakState, WorkName: "reading", EnteredAt: s.AsOf.Add(-2 * time.Hour), CurrentStateStartedAt: s.AsOf.Add(-time.Hour), CurrentSegmentStartedAt: s.AsOf.Add(-time.Hour), CumulativeWorkSec: 3600}
	s.Segments = []repository.WorkSegmentDoc{{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: s.AsOf.Add(-2 * time.Hour), EndedAt: s.AsOf.Add(-time.Hour), DurationSec: 3600}}
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectSeconds(t, r.Summary.Data.Today, 3600)
	expectSeconds(t, r.Summary.Data.Lifetime, 9600)
	if r.Current.Data.State != "break" || *r.Current.Data.WorkName != "reading" {
		t.Fatal("break lost ordinary work name")
	}
}

func TestAggregationMidnightAndMondayWindows(t *testing.T) {
	s := workFixture()
	s.AsOf = instant("2026-10-05T00:00:00Z")
	start := instant("2026-10-04T14:59:29Z")
	end := instant("2026-10-04T15:00:31Z")
	s.Segments = []repository.WorkSegmentDoc{{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: start, EndedAt: end, DurationSec: 62}}
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectSeconds(t, r.Summary.Data.Today, 31)
	expectSeconds(t, r.Summary.Data.Week, 31)
	expectSeconds(t, r.Recent7Days.Data[5].Metric, 31)
	expectSeconds(t, r.Recent7Days.Data[6].Metric, 31)
	if r.Recent7Days.Data[0].Date != "2026-09-29" || r.Recent7Days.Data[6].Date != "2026-10-05" {
		t.Fatal("recent window is not seven JST calendar days")
	}
	w := StatisticsWindows(instant("2026-10-04T14:59:59Z"))
	if w.Week.Format("2006-01-02") != "2026-09-28" {
		t.Fatal("week started before JST Monday")
	}
	if !r.GeneratedAt.Equal(s.AsOf) {
		t.Fatal("generatedAt changed from snapshot")
	}
}

func TestAggregationReadFailureAndInvalidSeatAreNotZero(t *testing.T) {
	for _, kind := range []string{"history-failure", "seat-failure", "duplicate-seat"} {
		t.Run(kind, func(t *testing.T) {
			s := workFixture()
			switch kind {
			case "history-failure":
				s.HistoryUnavailable = true
			case "seat-failure":
				s.SeatUnavailable = true
			case "duplicate-seat":
				s.SeatInconsistent = true
			}
			r, err := Aggregate(s, fixtureAccount())
			if err != nil {
				t.Fatal(err)
			}
			if !r.Partial {
				t.Fatal("missing source marked complete")
			}
			if r.Summary.Data != nil && r.Summary.Data.Today.WorkSec != nil {
				t.Fatal("missing history fabricated zero")
			}
			if r.Recent7Days.Data[6].WorkSec != nil {
				t.Fatal("unknown day fabricated zero")
			}
			if kind == "seat-failure" || kind == "duplicate-seat" {
				if r.Current.Availability != Unavailable || r.Current.Data != nil {
					t.Fatal("failed seat lookup fabricated not_seated")
				}
			}
		})
	}
}

func TestAggregationHistoryLimitPreservesIndependentSections(t *testing.T) {
	s := workFixture()
	s.Segments = make([]repository.WorkSegmentDoc, HistoryReadLimit)
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary.Availability != Partial || *r.Summary.Data.Today.ReasonCode != HistoryLimitExceeded {
		t.Fatal("limit not detected")
	}
	expectSeconds(t, r.Summary.Data.Lifetime, 6000)
	if r.Current.Data.State != "not_seated" || r.Account.Availability != Available {
		t.Fatal("independent sections discarded")
	}
}

func TestAggregationOverlapAndFutureOnlyInvalidateAffectedHistory(t *testing.T) {
	for _, future := range []bool{false, true} {
		s := workFixture()
		start := s.AsOf.Add(-time.Hour)
		end := s.AsOf
		if future {
			end = s.AsOf.Add(time.Hour)
		}
		segment := repository.WorkSegmentDoc{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: start, EndedAt: end, DurationSec: int(end.Sub(start) / time.Second)}
		s.Segments = []repository.WorkSegmentDoc{segment}
		if !future {
			s.Segments = append(s.Segments, segment)
		}
		r, err := Aggregate(s, fixtureAccount())
		if err != nil {
			t.Fatal(err)
		}
		if r.Summary.Data.Today.Availability != Unavailable || *r.Summary.Data.Today.ReasonCode != DataInconsistent {
			t.Fatal("invalid interval counted")
		}
		expectSeconds(t, r.Summary.Data.Lifetime, 6000)
		expectSeconds(t, r.Recent7Days.Data[0].Metric, 0)
	}
}

func TestAggregationEmptyHistoryAndBreakOnlyGiveNormalZero(t *testing.T) {
	for _, tc := range []struct {
		name     string
		segments []repository.WorkSegmentDoc
	}{
		{name: "empty"},
		{name: "break-only", segments: []repository.WorkSegmentDoc{{
			UserID: "synthetic", SegmentType: repository.BreakState,
			StartedAt: instant("2026-10-06T00:00:00Z"), EndedAt: instant("2026-10-06T01:00:00Z"), DurationSec: 3600,
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := workFixture()
			s.Segments = tc.segments
			r, err := Aggregate(s, fixtureAccount())
			if err != nil {
				t.Fatal(err)
			}
			if r.Partial {
				t.Fatal("normal zero became partial")
			}
			expectSeconds(t, r.Summary.Data.Today, 0)
			expectSeconds(t, r.Summary.Data.Week, 0)
			expectRecentSeconds(t, r.Recent7Days, 0)
		})
	}
}

func TestAggregationNegativeDurationIsNotZero(t *testing.T) {
	s := workFixture()
	s.Segments = []repository.WorkSegmentDoc{{
		UserID: s.UID, SegmentType: repository.WorkState,
		StartedAt: s.AsOf.Add(-time.Hour), EndedAt: s.AsOf, DurationSec: -3600,
	}}
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	want := DataInconsistent
	if metric := r.Summary.Data.Today; metric.Availability != Unavailable || metric.WorkSec != nil || metric.ReasonCode == nil || *metric.ReasonCode != want {
		t.Fatalf("negative duration was not rejected: %+v", metric)
	}
	expectSeconds(t, r.Summary.Data.Lifetime, 6000)
	expectSeconds(t, r.Recent7Days.Data[0].Metric, 0)
}

func TestAggregationUnregisteredHasNormalZeroAndNoSeat(t *testing.T) {
	s := workFixture()
	s.User = nil
	s.SeatUnavailable = true
	s.HistoryUnavailable = true
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	if r.Partial || r.Current.Data.State != "unregistered" || r.Current.Data.WorkName != nil {
		t.Fatal("unregistered is not a normal empty state")
	}
	expectSeconds(t, r.Summary.Data.Lifetime, 0)
	expectSeconds(t, r.Summary.Data.Today, 0)
	if _, err := Aggregate(WorkSnapshot{}, fixtureAccount()); err == nil {
		t.Fatal("missing snapshot time accepted")
	}
}

func TestAggregationCountsOngoingWorkWithoutClosedSegments(t *testing.T) {
	s := workFixture()
	s.Seat = &repository.SeatDoc{UserID: s.UID, SeatID: 1, State: repository.WorkState, EnteredAt: s.AsOf.Add(-time.Hour), CurrentStateStartedAt: s.AsOf.Add(-time.Hour), CurrentSegmentStartedAt: s.AsOf.Add(-30 * time.Minute)}
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectSeconds(t, r.Summary.Data.Today, 1800)
	expectSeconds(t, r.Summary.Data.Week, 1800)
	expectRecentSeconds(t, r.Recent7Days, 1800)
}
