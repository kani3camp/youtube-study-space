package mypage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"app.modules/core/repository"
)

func recordedFixture() WorkSnapshot {
	s := workFixture()
	s.Coverage = nil
	s.Segments = []repository.WorkSegmentDoc{
		{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: s.AsOf.Add(-2 * time.Hour), EndedAt: s.AsOf.Add(-90 * time.Minute), DurationSec: 1800},
		{UserID: s.UID, SegmentType: repository.BreakState, StartedAt: s.AsOf.Add(-90 * time.Minute), EndedAt: s.AsOf.Add(-80 * time.Minute), DurationSec: 600},
		{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: s.AsOf.Add(-80 * time.Minute), EndedAt: s.AsOf.Add(-60 * time.Minute), DurationSec: 1200},
	}
	return s
}

func expectRecorded(t *testing.T, metric Metric, seconds int64) {
	t.Helper()
	if metric.Availability != Unavailable || metric.ReasonCode == nil || *metric.ReasonCode != HistoryIncomplete || metric.WorkSec != nil || metric.RecordedWorkSec == nil || *metric.RecordedWorkSec != seconds {
		t.Fatalf("recorded metric=%+v want %d without an exact total", metric, seconds)
	}
}

func TestRecordedHistoryPreservesUnknownCompletenessAndExcludesBreaks(t *testing.T) {
	s := recordedFixture()
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectRecorded(t, r.Summary.Data.Today, 3000)
	expectRecorded(t, r.Summary.Data.Week, 3000)
	expectRecorded(t, r.Recent7Days.Data[6].Metric, 3000)
	if !r.Partial || r.Recent7Days.Availability != Unavailable {
		t.Fatal("recorded evidence was promoted to a complete history")
	}
	for _, day := range r.Recent7Days.Data[:6] {
		if day.WorkSec != nil || day.RecordedWorkSec != nil {
			t.Fatal("unknown empty day was converted to zero")
		}
	}
	wire, err := json.Marshal(r)
	if err != nil || !strings.Contains(string(wire), `"workSec":null,"recordedWorkSec":3000`) {
		t.Fatal("additive recorded evidence missing from wire")
	}
	s.Coverage = workFixture().Coverage
	r, err = Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectSeconds(t, r.Summary.Data.Today, 3000)
	if r.Summary.Data.Today.RecordedWorkSec != nil {
		t.Fatal("exact metric also carried tentative evidence")
	}
}

func TestRecordedHistoryUsesJSTDayBoundariesAndLiveSegment(t *testing.T) {
	s := workFixture()
	s.Coverage = nil
	s.AsOf = instant("2026-10-05T00:00:00Z")
	s.Segments = []repository.WorkSegmentDoc{{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: instant("2026-10-04T14:59:30Z"), EndedAt: instant("2026-10-04T15:00:30Z"), DurationSec: 60}}
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectRecorded(t, r.Recent7Days.Data[5].Metric, 30)
	expectRecorded(t, r.Recent7Days.Data[6].Metric, 30)
	expectRecorded(t, r.Summary.Data.Week, 30)
	s = workFixture()
	s.Coverage = nil
	s.Seat = &repository.SeatDoc{UserID: s.UID, SeatID: 1, State: repository.WorkState, EnteredAt: s.AsOf.Add(-time.Hour), CurrentStateStartedAt: s.AsOf.Add(-time.Hour), CurrentSegmentStartedAt: s.AsOf.Add(-30 * time.Minute)}
	s.Segments = []repository.WorkSegmentDoc{{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: s.AsOf.Add(-time.Hour), EndedAt: s.AsOf.Add(-30 * time.Minute), DurationSec: 1800}}
	r, err = Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectRecorded(t, r.Summary.Data.Today, 3600)
}

func TestRecordedHistoryNeverLeaksEvidenceOnInvalidOrUnreadHistory(t *testing.T) {
	for _, failure := range []string{"read", "seat", "limit", "duration", "overlap", "future", "empty", "break-only"} {
		t.Run(failure, func(t *testing.T) {
			s := recordedFixture()
			switch failure {
			case "read":
				s.HistoryUnavailable = true
			case "seat":
				s.SeatUnavailable = true
			case "limit":
				s.Segments = make([]repository.WorkSegmentDoc, HistoryReadLimit)
			case "duration":
				s.Segments[0].DurationSec++
			case "overlap":
				s.Segments = append(s.Segments, s.Segments[0])
			case "future":
				s.Segments[0].EndedAt = s.AsOf.Add(time.Hour)
			case "empty":
				s.Segments = nil
			case "break-only":
				s.Segments = s.Segments[1:2]
			}
			r, err := Aggregate(s, fixtureAccount())
			if err != nil {
				t.Fatal(err)
			}
			if r.Summary.Data != nil && r.Summary.Data.Today.RecordedWorkSec != nil {
				t.Fatal("unsafe summary evidence")
			}
			for _, day := range r.Recent7Days.Data {
				if day.RecordedWorkSec != nil {
					t.Fatal("unsafe day evidence")
				}
			}
		})
	}
}
