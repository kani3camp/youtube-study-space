package mypage

import (
	"testing"
	"time"

	"app.modules/core/repository"
)

func TestBreakSeatMovePreservesOriginalStartAndNewSegmentBoundary(t *testing.T) {
	s := workFixture()
	breakStart := s.AsOf.Add(-time.Hour)
	moveTime := s.AsOf.Add(-30 * time.Minute)
	s.Seat = &repository.SeatDoc{
		UserID: s.UID, SeatID: 2, State: repository.BreakState,
		WorkName: "reading", EnteredAt: moveTime,
		CurrentStateStartedAt: breakStart, CurrentSegmentStartedAt: moveTime,
	}
	s.Segments = []repository.WorkSegmentDoc{
		{UserID: s.UID, SegmentType: repository.WorkState, StartedAt: breakStart.Add(-time.Hour), EndedAt: breakStart, DurationSec: 3600},
		{UserID: s.UID, SegmentType: repository.BreakState, StartedAt: breakStart, EndedAt: moveTime, DurationSec: 1800},
	}
	r, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	if r.Partial || r.Current.Data == nil || r.Current.Data.State != "break" || !r.Current.Data.StateStartedAt.Equal(breakStart) {
		t.Fatal("legitimate moved break was invalidated or its original start changed")
	}
	expectSeconds(t, r.Summary.Data.Today, 3600)
	expectSeconds(t, r.Summary.Data.Lifetime, int64(s.User.TotalStudySec))
	// A moved seat cannot claim an ongoing segment belonging to the old seat.
	s.Seat.CurrentSegmentStartedAt = breakStart
	bad, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	if bad.Current.Availability != Unavailable || bad.Current.ReasonCode == nil || *bad.Current.ReasonCode != DataInconsistent {
		t.Fatal("segment before the new seat's entry was accepted")
	}
}
