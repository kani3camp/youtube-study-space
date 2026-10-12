//go:build integration

package workspaceapp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"app.modules/core/mypage"
	"app.modules/core/repository"
	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
)

func TestWriterIdentityQueriesParticipateInTransaction(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	integrationtest.ResetFirestore(t)
	repo := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	uid := "synthetic-writer-user"
	client, ok := repo.FirestoreClient().(*firestore.Client)
	if !ok {
		t.Fatal("emulator repository client type")
	}
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	initial := repository.SeatDoc{UserID: uid, SeatID: 1, SessionID: "synthetic-session", State: repository.WorkState, WorkName: "reading", EnteredAt: start, CurrentStateStartedAt: start, CurrentSegmentStartedAt: start}
	if err := repo.CreateUser(ctx, nil, uid, repository.UserDoc{TotalStudySec: 100}); err != nil {
		t.Fatal(err)
	}
	if err := repo.FirestoreClient().RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error { return repo.CreateSeat(tx, initial, false) }); err != nil {
		t.Fatal(err)
	}
	app := &WorkspaceApp{Repository: repo}
	var wg sync.WaitGroup
	errors := make(chan error, 4)
	for range 4 {
		wg.Go(func() {
			for range 3 {
				err := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
					seat, err := app.CurrentSeat(ctx, uid, false)
					if err != nil {
						return fmt.Errorf("emulator fixture transaction: %w", err)
					}
					seat.CumulativeWorkSec++
					return repo.UpdateSeat(ctx, tx, seat, false)
				})
				if err != nil {
					errors <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	seat, err := repo.ReadSeatWithUserID(ctx, uid, false)
	if err != nil {
		t.Fatal(err)
	}
	if seat.CumulativeWorkSec != 12 {
		t.Fatalf("lost concurrent writer changes: cumulative=%d", seat.CumulativeWorkSec)
	}
	// Use the existing methods used by change/break/out and verify each snapshot.
	for _, operation := range []string{"rename", "break", "resume", "exit"} {
		at := start.Add(30 * time.Minute)
		err := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			seat, err := app.CurrentSeat(ctx, uid, false)
			if err != nil {
				return fmt.Errorf("emulator fixture transaction: %w", err)
			}
			user, err := repo.ReadUser(ctx, tx, uid)
			if err != nil {
				return fmt.Errorf("emulator fixture transaction: %w", err)
			}
			if _, err := repo.ReadWorkStateSegmentsBySessionID(ctx, seat.SessionID); err != nil {
				return fmt.Errorf("emulator fixture transaction: %w", err)
			}
			// Shift each boundary by a minute so no zero-length fixture is needed.
			switch operation {
			case "break":
				at = at.Add(time.Minute)
			case "resume":
				at = at.Add(2 * time.Minute)
			case "exit":
				at = at.Add(3 * time.Minute)
			}
			segment, err := seat.GenerateWorkSegment(at, false)
			if err != nil {
				return fmt.Errorf("emulator fixture transaction: %w", err)
			}
			if err := repo.CreateWorkSegmentDoc(ctx, tx, segment); err != nil {
				return fmt.Errorf("emulator fixture transaction: %w", err)
			}
			switch operation {
			case "rename":
				seat.SetCurrentSegmentStartedAt(at)
				seat.SetWorkName("new reading")
			case "break":
				if err := seat.StartBreak(at, 1); err != nil {
					return fmt.Errorf("emulator fixture transaction: %w", err)
				}
			case "resume":
				if err := seat.ResumeWork(at, seat.WorkName); err != nil {
					return fmt.Errorf("emulator fixture transaction: %w", err)
				}
			case "exit":
				if err := app.UpdateTotalWorkTime(tx, uid, &user, seat.CumulativeWorkSec+int(at.Sub(seat.CurrentStateStartedAt)/time.Second), 0); err != nil {
					return fmt.Errorf("emulator fixture transaction: %w", err)
				}
				return repo.DeleteSeat(ctx, tx, 1, false)
			}
			return repo.UpdateSeat(ctx, tx, seat, false)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	user, err := repo.ReadUser(ctx, nil, uid)
	if err != nil {
		t.Fatal(err)
	}
	segments, err := repo.ReadWorkStateSegmentsBySessionID(ctx, "synthetic-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(segments) != 3 {
		t.Fatal("rename/break/resume/exit did not close three work intervals")
	}
	total := 0
	for _, segment := range segments {
		total += segment.DurationSec
	}
	if user.TotalStudySec != 100+12+total {
		t.Fatal("writer lifetime transfer lost or duplicated session work")
	}
	reader := mypage.FirestoreSnapshotReader{Client: client, Coverage: []mypage.Interval{{Start: start.AddDate(0, 0, -30), End: time.Now().UTC().Add(time.Hour)}}}
	snapshot, err := reader.Read(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	response, err := mypage.Aggregate(snapshot, mypage.Section[mypage.Account]{Availability: mypage.Available, Data: &mypage.Account{DisplayName: "Sample"}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Partial || response.Current.Data.State != "not_seated" || *response.Summary.Data.Lifetime.WorkSec != int64(user.TotalStudySec) {
		t.Fatal("writer exit was not reflected as one valid snapshot")
	}
}
