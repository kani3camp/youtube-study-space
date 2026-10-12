//go:build integration

package mypage

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"app.modules/core/repository"

	"cloud.google.com/go/firestore"
)

func snapshotFixture(t *testing.T) (*FirestoreSnapshotReader, string) {
	t.Helper()
	_, store, _, _, now := authTestService(t)
	uid, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	reader := &FirestoreSnapshotReader{Client: store.Client, Coverage: []Interval{{now.AddDate(0, 0, -30), now.Add(time.Hour)}}}
	return reader, uid
}

func TestSnapshotMissingUserSkipsCorruptSeatAndHistory(t *testing.T) {
	r, uid := snapshotFixture(t)
	if _, err := r.Client.Collection("seats").Doc(uid).Set(context.Background(), map[string]any{"user-id": uid, "seat-id": "invalid"}); err != nil {
		t.Fatal(err)
	}
	s, err := r.Read(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if s.AsOf.IsZero() || s.User != nil || s.Seat != nil || s.SeatInconsistent || len(s.Segments) != 0 {
		t.Fatal("unregistered lookup scanned other collections or lost readTime")
	}
}

func TestSnapshotQueriesOnlyIdentityAndIntersectingWindow(t *testing.T) {
	r, uid := snapshotFixture(t)
	capturedAt := time.Now().UTC()
	if _, err := r.Client.Collection("users").Doc(uid).Set(context.Background(), repository.UserDoc{TotalStudySec: 600}); err != nil {
		t.Fatal(err)
	}
	segments := snapshotQuerySegments(uid, capturedAt)
	for i, segment := range segments {
		if _, err := r.Client.Collection("work-segments").Doc(uid+strconv.Itoa(i)).Set(context.Background(), segment); err != nil {
			t.Fatal(err)
		}
	}
	s, err := r.Read(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if s.HistoryUnavailable || len(s.Segments) != 1 || s.Segments[0].UserID != uid {
		t.Fatal("snapshot query did not enforce user and bounded overlap window")
	}
	response, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	// A snapshot taken near JST midnight may correctly count this minute as
	// yesterday. The query contract preserves its total across recent days;
	// fixed-clock cases cover the exact day allocation separately.
	expectRecentSeconds(t, response.Recent7Days, 60)
	expectSeconds(t, response.Summary.Data.Lifetime, 600)
	if !response.GeneratedAt.Equal(s.AsOf) {
		t.Fatal("snapshot timestamp changed")
	}
}

func TestSnapshotDetectsMultipleSeatsAndHistoryOverflow(t *testing.T) {
	r, uid := snapshotFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Hour)
	if _, err := r.Client.Collection("users").Doc(uid).Set(ctx, repository.UserDoc{TotalStudySec: 600}); err != nil {
		t.Fatal(err)
	}
	seat := repository.SeatDoc{UserID: uid, SeatID: 1, State: repository.WorkState, EnteredAt: now, CurrentStateStartedAt: now, CurrentSegmentStartedAt: now}
	for _, collection := range []string{"seats", "member-seats"} {
		if _, err := r.Client.Collection(collection).Doc(uid).Set(ctx, seat); err != nil {
			t.Fatal(err)
		}
	}
	s, err := r.Read(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !s.SeatInconsistent {
		t.Fatal("multiple room memberships accepted")
	}
	for _, collection := range []string{"seats", "member-seats"} {
		if _, err := r.Client.Collection(collection).Doc(uid).Delete(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for base := 0; base < HistoryReadLimit+4; base += 400 {
		err := r.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			for i := base; i < base+400 && i < HistoryReadLimit+4; i++ {
				ref := r.Client.Collection("work-segments").Doc(uid + strconv.Itoa(i))
				if err := tx.Set(ref, repository.WorkSegmentDoc{UserID: uid, StartedAt: now, EndedAt: now.Add(time.Second), DurationSec: 1, SegmentType: repository.WorkState}); err != nil {
					return fmt.Errorf("seed bounded history: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	s, err = r.Read(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Segments) != HistoryReadLimit {
		t.Fatalf("read %d segments; must stop at %d", len(s.Segments), HistoryReadLimit)
	}
	response, err := Aggregate(s, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	if *response.Summary.Data.Today.ReasonCode != HistoryLimitExceeded {
		t.Fatal("1000 overflow not detected")
	}
	expectSeconds(t, response.Summary.Data.Lifetime, 600)
}

func TestSnapshotKeepsUserSeatAndHistoryAtomicAtExit(t *testing.T) {
	r, uid := snapshotFixture(t)
	ctx := context.Background()
	start := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second)
	userRef := r.Client.Collection("users").Doc(uid)
	seatRef := r.Client.Collection("seats").Doc(uid)
	if _, err := userRef.Set(ctx, repository.UserDoc{TotalStudySec: 100}); err != nil {
		t.Fatal(err)
	}
	if _, err := seatRef.Set(ctx, repository.SeatDoc{UserID: uid, SeatID: 1, State: repository.WorkState, EnteredAt: start, CurrentStateStartedAt: start, CurrentSegmentStartedAt: start}); err != nil {
		t.Fatal(err)
	}
	before, err := r.Read(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	closedAt := time.Now().UTC().Truncate(time.Second)
	err = r.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if _, err := tx.Get(userRef); err != nil {
			return fmt.Errorf("emulator fixture transaction: %w", err)
		}
		if _, err := tx.Get(seatRef); err != nil {
			return fmt.Errorf("emulator fixture transaction: %w", err)
		}
		if err := tx.Set(userRef, repository.UserDoc{TotalStudySec: 100 + int(closedAt.Sub(start)/time.Second)}); err != nil {
			return fmt.Errorf("emulator fixture transaction: %w", err)
		}
		if err := tx.Delete(seatRef); err != nil {
			return fmt.Errorf("emulator fixture transaction: %w", err)
		}
		return tx.Create(r.Client.Collection("work-segments").Doc(uid), repository.WorkSegmentDoc{UserID: uid, SegmentType: repository.WorkState, StartedAt: start, EndedAt: closedAt, DurationSec: int(closedAt.Sub(start) / time.Second)})
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := r.Read(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if before.Seat == nil || len(before.Segments) != 0 || before.User.TotalStudySec != 100 {
		t.Fatal("pre-exit snapshot included closed data")
	}
	if after.Seat != nil || len(after.Segments) != 1 {
		t.Fatal("post-exit snapshot mixed old seat with closed history")
	}
	response, err := Aggregate(after, fixtureAccount())
	if err != nil {
		t.Fatal(err)
	}
	expectSeconds(t, response.Summary.Data.Lifetime, int64(after.User.TotalStudySec))
}
