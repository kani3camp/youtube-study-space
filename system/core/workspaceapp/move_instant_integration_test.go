//go:build integration

package workspaceapp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/utils"
	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
)

func TestSeatMoveSharesOneInstantWithAdvancingClock(t *testing.T) {
	for _, state := range []repository.SeatState{repository.WorkState, repository.BreakState} {
		t.Run(string(state), func(t *testing.T) {
			integrationtest.RequireFirestoreEmulator(t)
			integrationtest.ResetFirestore(t)
			repo := integrationtest.NewFirestoreController(t)
			ctx := context.Background()
			boundary := time.Date(2026, 10, 5, 15, 0, 0, 0, time.UTC)
			start := boundary.Add(-10 * time.Minute)
			transition := boundary.Add(-time.Second)
			uid := "synthetic-moving-user"
			user := repository.UserDoc{TotalStudySec: 100, IsContinuousActive: true, CurrentActivityStateStarted: start}
			if err := repo.CreateUser(ctx, nil, uid, user); err != nil {
				t.Fatal(err)
			}
			seat := repository.SeatDoc{UserID: uid, SeatID: 1, SessionID: "synthetic-moving-session", WorkName: "reading", State: state, EnteredAt: start, CurrentStateStartedAt: start, CurrentSegmentStartedAt: start, Until: boundary.Add(time.Hour), CurrentStateUntil: boundary.Add(time.Hour)}
			var previous []repository.WorkSegmentDoc
			if state == repository.BreakState {
				seat.CurrentStateStartedAt = start.Add(5 * time.Minute)
				seat.CurrentSegmentStartedAt = seat.CurrentStateStartedAt
				seat.CumulativeWorkSec = 300
				seat.DailyCumulativeWorkSec = 300
				previous = []repository.WorkSegmentDoc{{UserID: uid, SessionID: seat.SessionID, SegmentType: repository.WorkState, StartedAt: start, EndedAt: seat.CurrentStateStartedAt, DurationSec: 300}}
			}
			if err := repo.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error { return repo.CreateSeat(tx, seat, false) }); err != nil {
				t.Fatal(err)
			}
			clock := transition
			app := &WorkspaceApp{Repository: repo, alertOwnerBot: moderatorbot.DummyMessageBot{}, nowFunc: func() time.Time { now := clock; clock = clock.Add(2 * time.Second); return now }}
			var closedSec int
			if err := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
				if _, err := repo.ReadUser(ctx, tx, uid); err != nil {
					return fmt.Errorf("emulator fixture transaction: %w", err)
				}
				if _, err := repo.ReadSeat(ctx, tx, 1, false); err != nil {
					return fmt.Errorf("emulator fixture transaction: %w", err)
				}
				var err error
				closedSec, _, _, err = app.moveSeat(ctx, tx, 2, "", false, false, utils.MinWorkOrderOption{}, seat, &user, previous)
				if err != nil {
					return fmt.Errorf("move fixture seat: %w", err)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			moved, err := repo.ReadSeat(ctx, nil, 2, false)
			if err != nil {
				t.Fatal(err)
			}
			docs, err := repo.FirestoreClient().Collection("work-segments").Where("user-id", "==", uid).Documents(ctx).GetAll()
			if err != nil {
				t.Fatal(err)
			}
			if len(docs) != 1 {
				t.Fatal("move did not close exactly one current interval")
			}
			var closed repository.WorkSegmentDoc
			if err := docs[0].DataTo(&closed); err != nil {
				t.Fatal(err)
			}
			if !closed.EndedAt.Equal(moved.CurrentSegmentStartedAt) || !moved.EnteredAt.Equal(transition) {
				t.Fatal("advancing clock created overlap or a gap during move")
			}
			if state == repository.BreakState {
				if !moved.CurrentStateStartedAt.Equal(seat.CurrentStateStartedAt) || closedSec != 300 {
					t.Fatal("moving break changed original start or added break work")
				}
			} else if closedSec != int(transition.Sub(start)/time.Second) {
				t.Fatal("lifetime used a second transition instant")
			}
		})
	}
}
