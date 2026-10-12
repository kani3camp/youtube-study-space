package mypage

import (
	"fmt"
	"math"
	"slices"
	"time"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
)

const HistoryReadLimit = 1001

type Interval struct{ Start, End time.Time }

// WorkSnapshot is populated only from one consistent Firestore snapshot. An
// empty Coverage deliberately means unknown coverage, never seven fake zeros.
type WorkSnapshot struct {
	AsOf               time.Time
	UID                string
	User               *repository.UserDoc
	Seat               *repository.SeatDoc
	MemberSeat         bool
	Segments           []repository.WorkSegmentDoc
	Coverage           []Interval
	SeatUnavailable    bool
	SeatInconsistent   bool
	HistoryUnavailable bool
}

type Windows struct{ Today, Week, Recent, End time.Time }

func StatisticsWindows(asOf time.Time) Windows {
	jst := asOf.In(timeutil.JapanLocation())
	today := time.Date(jst.Year(), jst.Month(), jst.Day(), 0, 0, 0, 0, jst.Location())
	daysSinceMonday := (int(today.Weekday()) + 6) % 7
	return Windows{Today: today, Week: today.AddDate(0, 0, -daysSinceMonday), Recent: today.AddDate(0, 0, -6), End: asOf}
}

func (w Windows) QueryStart() time.Time {
	if w.Week.Before(w.Recent) {
		return w.Week
	}
	return w.Recent
}

func secondsWithin(v Interval, start, end time.Time) int64 {
	if v.Start.Before(start) {
		v.Start = start
	}
	if v.End.After(end) {
		v.End = end
	}
	if !v.End.After(v.Start) {
		return 0
	}
	return int64(v.End.Sub(v.Start) / time.Second)
}

func intersects(a, b Interval) bool { return a.Start.Before(b.End) && b.Start.Before(a.End) }

func covered(coverage []Interval, start, end time.Time) bool {
	if !end.After(start) {
		return true
	}
	parts := slices.Clone(coverage)
	slices.SortFunc(parts, func(a, b Interval) int { return a.Start.Compare(b.Start) })
	cursor := start
	for _, part := range parts {
		if part.Start.After(cursor) {
			return false
		}
		if part.End.After(cursor) {
			cursor = part.End
		}
		if !cursor.Before(end) {
			return true
		}
	}
	return false
}

func validSeat(s WorkSnapshot) bool {
	if s.SeatInconsistent {
		return false
	}
	seat := s.Seat
	if seat == nil {
		return true
	}
	return !s.SeatInconsistent && seat.UserID == s.UID && seat.SeatID > 0 && seat.CumulativeWorkSec >= 0 &&
		(seat.State == repository.WorkState || seat.State == repository.BreakState) && !seat.EnteredAt.IsZero() &&
		!seat.CurrentStateStartedAt.IsZero() &&
		(seat.State == repository.BreakState || !seat.CurrentStateStartedAt.Before(seat.EnteredAt)) &&
		!seat.CurrentSegmentStartedAt.Before(seat.EnteredAt) && !seat.CurrentSegmentStartedAt.Before(seat.CurrentStateStartedAt) &&
		!seat.CurrentSegmentStartedAt.After(s.AsOf) && !seat.CurrentStateStartedAt.After(s.AsOf)
}

func currentSection(s WorkSnapshot) Section[Current] {
	if s.User == nil {
		return availableSection(Current{State: "unregistered"})
	}
	if s.SeatUnavailable {
		return missingSection[Current](SourceUnavailable)
	}
	if !validSeat(s) {
		return missingSection[Current](DataInconsistent)
	}
	if s.Seat == nil {
		return availableSection(Current{State: "not_seated"})
	}
	seat := s.Seat
	room := "standard"
	if s.MemberSeat {
		room = "member"
	}
	current := Current{State: string(seat.State), WorkName: ptr(seat.WorkName), RoomType: &room, SeatNumber: ptr(seat.SeatID), StateStartedAt: ptr(seat.CurrentStateStartedAt.UTC())}
	if !seat.CurrentStateUntil.IsZero() {
		current.ExpectedEndAt = ptr(seat.CurrentStateUntil.UTC())
	}
	return availableSection(current)
}

func lifetimeMetric(s WorkSnapshot) Metric {
	if s.User == nil {
		return valueMetric(0)
	}
	if s.SeatUnavailable {
		return missingMetric(SourceUnavailable)
	}
	if s.User.TotalStudySec < 0 || !validSeat(s) {
		return missingMetric(DataInconsistent)
	}
	total := int64(s.User.TotalStudySec)
	if s.Seat != nil {
		// Existing writer transfers the whole seat session to users at exit only.
		// Work-name edits reset CurrentSegmentStartedAt, not this lifetime clock.
		delta := int64(s.Seat.CumulativeWorkSec)
		if s.Seat.State == repository.WorkState {
			ongoing := secondsWithin(Interval{s.Seat.CurrentStateStartedAt, s.AsOf}, s.Seat.CurrentStateStartedAt, s.AsOf)
			if delta > math.MaxInt64-ongoing {
				return missingMetric(DataInconsistent)
			}
			delta += ongoing
		}
		if total > math.MaxInt64-delta {
			return missingMetric(DataInconsistent)
		}
		total += delta
	}
	return valueMetric(total)
}

type historyModel struct {
	work   []Interval
	bad    []Interval
	allBad bool
	reason string
}

func history(s WorkSnapshot) historyModel {
	m := historyModel{}
	if s.User == nil {
		return m
	}
	if s.SeatUnavailable || s.HistoryUnavailable {
		m.reason = SourceUnavailable
		return m
	}
	if len(s.Segments) >= HistoryReadLimit {
		m.reason = HistoryLimitExceeded
		return m
	}
	if !validSeat(s) {
		m.reason = DataInconsistent
		return m
	}
	all := make([]Interval, 0, len(s.Segments)+1)
	for _, segment := range s.Segments {
		span := Interval{segment.StartedAt, segment.EndedAt}
		invalid := segment.UserID != s.UID || segment.StartedAt.IsZero() || !segment.EndedAt.After(segment.StartedAt) || segment.EndedAt.After(s.AsOf) || segment.DurationSec < 0 || int64(segment.DurationSec) != int64(segment.EndedAt.Sub(segment.StartedAt)/time.Second) || (segment.SegmentType != repository.WorkState && segment.SegmentType != repository.BreakState)
		if invalid {
			if span.Start.IsZero() || span.End.IsZero() {
				m.allBad = true
			} else {
				if span.End.Before(span.Start) {
					span.Start, span.End = span.End, span.Start
				}
				if span.Start.Equal(span.End) {
					span.End = span.End.Add(time.Nanosecond)
				}
				m.bad = append(m.bad, span)
			}
			continue
		}
		all = append(all, span)
		if segment.SegmentType == repository.WorkState {
			m.work = append(m.work, span)
		}
	}
	if s.Seat != nil {
		span := Interval{s.Seat.CurrentSegmentStartedAt, s.AsOf}
		all = append(all, span)
		if s.Seat.State == repository.WorkState {
			m.work = append(m.work, span)
		}
	}
	slices.SortFunc(all, func(a, b Interval) int { return a.Start.Compare(b.Start) })
	if len(all) > 0 {
		previous := all[0]
		for _, span := range all[1:] {
			if span.Start.Before(previous.End) {
				m.bad = append(m.bad, Interval{span.Start, previous.End})
			}
			if span.End.After(previous.End) {
				previous = span
			}
		}
	}
	return m
}

func (m historyModel) metric(s WorkSnapshot, start, end time.Time) Metric {
	if s.User == nil {
		return valueMetric(0)
	}
	if m.reason != "" {
		return missingMetric(m.reason)
	}
	if m.allBad {
		return missingMetric(DataInconsistent)
	}
	// Faults later in today's calendar day are not silently treated as zero.
	faultEnd := end
	if end.Equal(s.AsOf) {
		faultEnd = StatisticsWindows(s.AsOf).Today.AddDate(0, 0, 1)
	}
	for _, bad := range m.bad {
		if intersects(bad, Interval{start, faultEnd}) {
			return missingMetric(DataInconsistent)
		}
	}
	if !covered(s.Coverage, start, end) {
		return missingMetric(HistoryIncomplete)
	}
	var total int64
	for _, span := range m.work {
		seconds := secondsWithin(span, start, end)
		if total > math.MaxInt64-seconds {
			return missingMetric(DataInconsistent)
		}
		total += seconds
	}
	return valueMetric(total)
}

func metricsSection(metrics []Metric) (Availability, *string) {
	available := 0
	var reason *string
	for _, m := range metrics {
		if m.Availability == Available {
			available++
		} else if reason == nil {
			reason = m.ReasonCode
		}
	}
	if available == len(metrics) {
		return Available, nil
	}
	if available == 0 {
		return Unavailable, reason
	}
	return Partial, reason
}

func Aggregate(s WorkSnapshot, account Section[Account]) (Response, error) {
	if s.AsOf.IsZero() {
		return Response{}, fmt.Errorf("snapshot read time is required")
	}
	w := StatisticsWindows(s.AsOf)
	model := history(s)
	summary := Summary{Today: model.metric(s, w.Today, s.AsOf), Week: model.metric(s, w.Week, s.AsOf), Lifetime: lifetimeMetric(s)}
	sa, sr := metricsSection([]Metric{summary.Today, summary.Week, summary.Lifetime})
	summarySection := Section[Summary]{Availability: sa, ReasonCode: sr}
	if sa != Unavailable {
		summarySection.Data = &summary
	}
	days := make([]Day, 0, 7)
	metrics := make([]Metric, 0, 7)
	for i := range 7 {
		start := w.Recent.AddDate(0, 0, i)
		end := start.AddDate(0, 0, 1)
		if end.After(s.AsOf) {
			end = s.AsOf
		}
		m := model.metric(s, start, end)
		days = append(days, Day{Date: start.Format("2006-01-02"), Metric: m})
		metrics = append(metrics, m)
	}
	ra, rr := metricsSection(metrics)
	current := currentSection(s)
	return Response{GeneratedAt: s.AsOf.UTC(), Timezone: "Asia/Tokyo", Partial: current.Availability != Available || sa != Available || ra != Available || account.Availability != Available, Current: current, Summary: summarySection, Recent7Days: RecentSection{Availability: ra, ReasonCode: rr, Data: days}, Account: account}, nil
}
