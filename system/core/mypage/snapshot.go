package mypage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app.modules/core/repository"

	"cloud.google.com/go/firestore"
)

// SnapshotReadObservation contains only bounded counters; never store channel
// identities, document paths, or data. Returned documents are NOT the same as
// billable reads: missing-query minimums, index entries and retries require
// Query Explain / Cloud Billing for a complete cost measurement.
type SnapshotReadObservation struct {
	UserLookups         int
	SeatQueries         int
	SeatsReturned       int
	SegmentQueries      int
	SegmentsReturned    int
	ElapsedMilliseconds int64
}

type SnapshotReader interface {
	Read(context.Context, string) (WorkSnapshot, error)
}

type FirestoreSnapshotReader struct {
	Client *firestore.Client
	// Only verified history coverage may be configured. No default epoch.
	Coverage []Interval
	// Optional and called only once after a successful snapshot read.
	Observe func(SnapshotReadObservation)
}

func (r *FirestoreSnapshotReader) Read(ctx context.Context, uid string) (WorkSnapshot, error) {
	if uid == "" || strings.Contains(uid, "/") {
		return WorkSnapshot{}, apiError("AUTH_REQUIRED")
	}
	var result WorkSnapshot
	var observation SnapshotReadObservation
	started := time.Now()
	err := r.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Each transaction attempt gets fresh counters. Report once after success;
		// retries may generate extra billed reads, not included in these counts.
		attempt := SnapshotReadObservation{UserLookups: 1}
		defer func() { observation = attempt }()
		s := WorkSnapshot{UID: uid, Coverage: r.Coverage}
		// GetAll exposes readTime even for a missing user. Use it to choose the
		// JST window after the snapshot exists, avoiding request-time boundaries.
		docs, err := tx.GetAll([]*firestore.DocumentRef{r.Client.Collection("users").Doc(uid)})
		if err != nil {
			return fmt.Errorf("read snapshot user: %w", err)
		}
		if len(docs) != 1 || docs[0].ReadTime.IsZero() {
			return fmt.Errorf("snapshot user read time missing")
		}
		s.AsOf = docs[0].ReadTime.UTC()
		if !docs[0].Exists() {
			result = s
			return nil
		}
		var user repository.UserDoc
		if err := docs[0].DataTo(&user); err != nil {
			return fmt.Errorf("decode snapshot user: %w", err)
		}
		s.User = &user
		for _, name := range []string{"seats", "member-seats"} {
			attempt.SeatQueries++
			seats, err := tx.Documents(r.Client.Collection(name).Where("user-id", "==", uid).Limit(2)).GetAll()
			if err != nil {
				s.SeatUnavailable = true
				continue
			}
			attempt.SeatsReturned += len(seats)
			if len(seats) > 1 || (s.Seat != nil && len(seats) > 0) {
				s.SeatInconsistent = true
			}
			if len(seats) > 0 {
				var seat repository.SeatDoc
				if err := seats[0].DataTo(&seat); err != nil {
					s.SeatInconsistent = true
					continue
				}
				s.Seat = &seat
				s.MemberSeat = name == "member-seats"
			}
		}
		window := StatisticsWindows(s.AsOf)
		query := r.Client.Collection("work-segments").Where("user-id", "==", uid).
			Where("ended-at", ">", window.QueryStart()).Where("started-at", "<", s.AsOf).
			OrderBy("ended-at", firestore.Asc).OrderBy("started-at", firestore.Asc).Limit(HistoryReadLimit)
		attempt.SegmentQueries++
		segments, err := tx.Documents(query).GetAll()
		if err != nil {
			s.HistoryUnavailable = true
		} else {
			attempt.SegmentsReturned = len(segments)
			for _, doc := range segments {
				var segment repository.WorkSegmentDoc
				if err := doc.DataTo(&segment); err != nil {
					// An unreadable interval may affect any day in the queried
					// window; do not discard it and report an apparently exact sum.
					s.HistoryUnavailable = true
					break
				}
				s.Segments = append(s.Segments, segment)
			}
		}
		result = s
		return nil
	}, firestore.ReadOnly)
	if err != nil {
		return WorkSnapshot{}, fmt.Errorf("read work snapshot: %w", err)
	}
	if r.Observe != nil {
		observation.ElapsedMilliseconds = time.Since(started).Milliseconds()
		r.Observe(observation)
	}
	return result, nil
}
