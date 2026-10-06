package mypage

import (
	"context"
	"fmt"
	"strings"

	"app.modules/core/repository"

	"cloud.google.com/go/firestore"
)

type SnapshotReader interface {
	Read(context.Context, string) (WorkSnapshot, error)
}

type FirestoreSnapshotReader struct {
	Client *firestore.Client
	// Only verified history coverage may be configured. No default epoch.
	Coverage []Interval
}

func (r *FirestoreSnapshotReader) Read(ctx context.Context, uid string) (WorkSnapshot, error) {
	if uid == "" || strings.Contains(uid, "/") {
		return WorkSnapshot{}, apiError("AUTH_REQUIRED")
	}
	var result WorkSnapshot
	err := r.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
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
			seats, err := tx.Documents(r.Client.Collection(name).Where("user-id", "==", uid).Limit(2)).GetAll()
			if err != nil {
				s.SeatUnavailable = true
				continue
			}
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
		segments, err := tx.Documents(query).GetAll()
		if err != nil {
			s.HistoryUnavailable = true
		} else {
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
	return result, nil
}
