//go:build integration

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"

	"app.modules/core/mypage"
	"app.modules/core/repository"
	"app.modules/core/serviceaccess"
	"app.modules/core/timeutil"
	"app.modules/internal/integrationtest"
)

func historyFixture(t *testing.T) (*repository.FirestoreControllerImplements, *firestore.Client, repository.WorkSegmentDoc) {
	t.Helper()
	integrationtest.ResetFirestore(t)
	repo := newTestRepository(t)
	client := repo.FirestoreClient().(*firestore.Client)
	start := time.Date(2026, 10, 24, 8, 0, 0, 0, timeutil.JapanLocation())
	s := repository.WorkSegmentDoc{UserID: "UCsynthetic0000000000001", SessionID: "synthetic-session", SeatID: 1, SegmentType: repository.WorkState, StartedAt: start, EndedAt: start.Add(time.Hour), DurationSec: 3600}
	_, err := client.Collection(repository.USERS).Doc(s.UserID).Set(context.Background(), repository.UserDoc{})
	require.NoError(t, err)
	_, err = client.Collection(repository.SEATS).Doc("1").Set(context.Background(), repository.SeatDoc{UserID: s.UserID, SessionID: s.SessionID, SeatID: 1, State: s.SegmentType, EnteredAt: start, CurrentStateStartedAt: start, CurrentSegmentStartedAt: start})
	require.NoError(t, err)
	return repo, client, s
}

func closeHistory(repo *repository.FirestoreControllerImplements, s repository.WorkSegmentDoc) error {
	ctx := repository.WithWorkHistoryAdmission(context.Background())
	return repo.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		prepared, err := repository.PrepareWorkSegmentWrites(ctx, tx, repo.FirestoreClient(), s)
		if err != nil {
			return err
		}
		return repo.CreateWorkSegmentDoc(prepared, tx, s)
	})
}

func readMonth(t *testing.T, client *firestore.Client, s repository.WorkSegmentDoc) repository.WorkMonthDoc {
	t.Helper()
	key := s.StartedAt.In(timeutil.JapanLocation()).Format("2006-01")
	doc, err := client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(s.UserID, key)).Get(context.Background())
	require.NoError(t, err)
	var value repository.WorkMonthDoc
	require.NoError(t, doc.DataTo(&value))
	return value
}

func TestHistorySourceAndMonthCommitOrRollbackTogether(t *testing.T) {
	repo, client, s := historyFixture(t)
	sentinel := errors.New("synthetic rollback")
	for _, rollback := range []bool{true, false} {
		ctx := repository.WithWorkHistoryAdmission(context.Background())
		err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
			prepared, err := repository.PrepareWorkSegmentWrites(ctx, tx, client, s)
			if err != nil {
				return err
			}
			if err := repo.CreateWorkSegmentDoc(prepared, tx, s); err != nil {
				return err
			}
			if rollback {
				return sentinel
			}
			return nil
		})
		if rollback {
			require.ErrorIs(t, err, sentinel)
		} else {
			require.NoError(t, err)
		}
		sources, err := client.Collection(repository.WorkSegments).Documents(context.Background()).GetAll()
		require.NoError(t, err)
		months, err := client.Collection(repository.WorkMonths).Documents(context.Background()).GetAll()
		require.NoError(t, err)
		if rollback {
			require.Empty(t, sources)
			require.Empty(t, months)
		} else {
			require.Len(t, sources, 1)
			require.Len(t, months, 1)
			m := readMonth(t, client, s)
			require.Equal(t, int64(3600), m.Days["2026-10-24"].WorkSec)
			require.False(t, m.BaselineComplete)
		}
	}
}

func TestHistoryReplayAndConcurrentTransactionsDoNotDoubleAdd(t *testing.T) {
	repo, client, s := historyFixture(t)
	var wg sync.WaitGroup
	errorsOut := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); errorsOut <- closeHistory(repo, s) }()
	}
	wg.Wait()
	close(errorsOut)
	for err := range errorsOut {
		require.NoError(t, err)
	}
	m := readMonth(t, client, s)
	require.Equal(t, int64(3600), m.Days["2026-10-24"].WorkSec)
	require.Equal(t, int64(1), m.Days["2026-10-24"].CompletedWorkSegments)
	sources, err := client.Collection(repository.WorkSegments).Documents(context.Background()).GetAll()
	require.NoError(t, err)
	require.Len(t, sources, 1)
	s.WorkName = "different payload for the same immutable event"
	require.ErrorIs(t, closeHistory(repo, s), repository.ErrWorkHistoryConflict)
	m = readMonth(t, client, s)
	require.Equal(t, int64(3600), m.Days["2026-10-24"].WorkSec)
}

func TestHistoryRequiresThisTransactionPreparationAndRejectsGenericDeletion(t *testing.T) {
	repo, client, s := historyFixture(t)
	err := client.RunTransaction(context.Background(), func(ctx context.Context, tx *firestore.Transaction) error {
		return repo.CreateWorkSegmentDoc(ctx, tx, s)
	})
	require.ErrorIs(t, err, repository.ErrWorkHistoryConflict)
	require.ErrorIs(t, repo.CreateWorkSegmentDoc(context.Background(), nil, s), repository.ErrWorkHistoryConflict)
	require.NoError(t, closeHistory(repo, s))
	rows, err := client.Collection(repository.WorkSegments).Documents(context.Background()).GetAll()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.ErrorIs(t, repo.DeleteDocRef(context.Background(), nil, rows[0].Ref), repository.ErrWorkHistoryConflict)
	_, err = rows[0].Ref.Get(context.Background())
	require.NoError(t, err)
}

func TestMonthMigrationIsBoundedDryRunVersionedAndRepairsLateOrdering(t *testing.T) {
	repo, client, s := historyFixture(t)
	late := s
	late.StartedAt = s.StartedAt.Add(-time.Hour)
	late.EndedAt = s.StartedAt
	require.NoError(t, closeHistory(repo, s))
	_, err := client.Collection(repository.SEATS).Doc("1").Update(context.Background(), []firestore.Update{{Path: "current-segment-started-at", Value: late.StartedAt}})
	require.NoError(t, err)
	require.NoError(t, closeHistory(repo, late))
	require.True(t, readMonth(t, client, s).Days["2026-10-24"].Invalid)
	input := repository.WorkMonthBackfill{UID: s.UserID, Month: "2026-10", WriterEpoch: "synthetic-v1", ControlCheckpoint: "absent", MaxDocuments: 1}
	report, err := repo.RebuildWorkMonth(context.Background(), input)
	require.ErrorIs(t, err, repository.ErrWorkHistoryConflict)
	require.Equal(t, 2, report.Documents)
	require.False(t, report.Applied)
	input.MaxDocuments = 10
	report, err = repo.RebuildWorkMonth(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(7200), report.RebuiltSec)
	require.False(t, report.Applied)
	require.True(t, readMonth(t, client, s).Days["2026-10-24"].Invalid, "dry run must not publish or repair")
	input.Apply = true
	input.ExpectedVersion = report.SourceVersion
	report, err = repo.RebuildWorkMonth(context.Background(), input)
	require.NoError(t, err)
	require.True(t, report.Applied)
	m := readMonth(t, client, s)
	require.False(t, m.Days["2026-10-24"].Invalid)
	require.True(t, m.BaselineComplete)
	require.Equal(t, int64(7200), m.Days["2026-10-24"].WorkSec)
	_, err = repo.RebuildWorkMonth(context.Background(), input)
	require.ErrorIs(t, err, repository.ErrWorkHistoryConflict, "old dry run version cannot overwrite a changed month")
	input.Apply = false
	input.ExpectedVersion = ""
	report, err = repo.RebuildWorkMonth(context.Background(), input)
	require.NoError(t, err)
	require.False(t, report.Changed)
}

func TestHistoryPrivacyFenceBlocksActiveAndOldAdmissionAfterClear(t *testing.T) {
	repo, client, s := historyFixture(t)
	ctx := repository.WithWorkHistoryAdmission(context.Background())
	// Capture the original absent checkpoint in an admitted operation.
	err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		_, err := repository.PrepareWorkSegmentWrites(ctx, tx, client, s)
		return err
	})
	require.NoError(t, err)
	control := &serviceaccess.FirestoreStore{Client: client}
	now := time.Now().UTC().Truncate(time.Microsecond)
	ref := fmt.Sprintf("%064d", 1)
	_, err = control.Change(context.Background(), s.UserID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "MODERATION", Reference: ref}, now)
	require.NoError(t, err)
	_, err = control.Change(context.Background(), s.UserID, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: true, ReasonCode: "PRIVACY_DELETION", Reference: ref}, now.Add(time.Second))
	require.NoError(t, err)
	require.ErrorIs(t, closeHistory(repo, s), serviceaccess.ErrDeletion)
	_, err = client.Collection(repository.SEATS).Doc("1").Delete(context.Background())
	require.NoError(t, err)
	_, err = client.Collection(repository.USERS).Doc(s.UserID).Delete(context.Background())
	require.NoError(t, err)
	_, err = control.Change(context.Background(), s.UserID, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: false}, now.Add(2*time.Second))
	require.NoError(t, err)
	err = client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		_, err := repository.PrepareWorkSegmentWrites(ctx, tx, client, s)
		return err
	})
	require.ErrorIs(t, err, repository.ErrWorkHistoryConflict)
	require.ErrorIs(t, repo.CreateWorkSegmentDoc(context.Background(), nil, s), repository.ErrWorkHistoryConflict)
	require.ErrorIs(t, closeHistory(repo, s), repository.ErrWorkHistoryConflict, "fresh invocation of an old source cannot revive deleted User/Seat")
	_, err = repo.RebuildWorkMonth(context.Background(), repository.WorkMonthBackfill{UID: s.UserID, Month: "2026-10", WriterEpoch: "synthetic", ControlCheckpoint: "absent", MaxDocuments: 10})
	require.ErrorIs(t, err, repository.ErrWorkHistoryConflict)
	rows, err := client.Collection(repository.WorkMonths).Documents(context.Background()).GetAll()
	require.NoError(t, err)
	require.Empty(t, rows)
	state, err := control.Read(context.Background(), s.UserID)
	require.NoError(t, err)
	require.True(t, state.Control.Moderation.Active)
}

func TestHistoryReaderCutoverWatermarkMissingMonthAndRollback(t *testing.T) {
	repo, client, s := historyFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 7; i++ {
		key := now.AddDate(0, 0, -i).In(timeutil.JapanLocation()).Format("2006-01")
		input := repository.WorkMonthBackfill{UID: s.UserID, Month: key, WriterEpoch: repository.MonthlyWorkEpoch, ControlCheckpoint: "absent", MaxDocuments: 10}
		dry, err := repo.RebuildWorkMonth(ctx, input)
		require.NoError(t, err)
		input.Apply = true
		input.ExpectedVersion = dry.SourceVersion
		_, err = repo.RebuildWorkMonth(ctx, input)
		require.NoError(t, err)
	}
	userRef := client.Collection(repository.USERS).Doc(s.UserID)
	user, err := userRef.Get(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.SetWorkMonthReader(ctx, s.UserID, "absent", user.UpdateTime, true, false))
	unchanged, err := userRef.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, user.UpdateTime, unchanged.UpdateTime)
	require.NoError(t, repo.SetWorkMonthReader(ctx, s.UserID, "absent", user.UpdateTime, true, true))
	reader := &mypage.FirestoreSnapshotReader{Client: client, Coverage: []mypage.Interval{{Start: now.AddDate(0, 0, -10), End: now.Add(time.Hour)}}}
	var stats mypage.SnapshotReadObservation
	reader.Observe = func(s mypage.SnapshotReadObservation) { stats = s }
	snapshot, err := reader.Read(ctx, s.UserID)
	require.NoError(t, err)
	require.Zero(t, stats.SegmentQueries)
	require.Equal(t, repository.MonthlyWorkEpoch, snapshot.MonthlyEpoch)
	// Close the actual source Seat after cutover and remove it in the same tx.
	s.StartedAt = now.Add(-10 * time.Minute)
	s.EndedAt = time.Now().UTC().Truncate(time.Microsecond)
	s.DurationSec = int(s.EndedAt.Sub(s.StartedAt) / time.Second)
	_, err = client.Collection(repository.SEATS).Doc("1").Update(ctx, []firestore.Update{{Path: "current-segment-started-at", Value: s.StartedAt}})
	require.NoError(t, err)
	err = client.RunTransaction(repository.WithWorkHistoryAdmission(ctx), func(ctx context.Context, tx *firestore.Transaction) error {
		prepared, err := repository.PrepareWorkSegmentWrites(ctx, tx, client, s)
		if err != nil {
			return err
		}
		if err := repo.CreateWorkSegmentDoc(prepared, tx, s); err != nil {
			return err
		}
		return tx.Delete(client.Collection(repository.SEATS).Doc("1"))
	})
	require.NoError(t, err)
	user, err = userRef.Get(ctx)
	require.NoError(t, err)
	var stored repository.UserDoc
	require.NoError(t, user.DataTo(&stored))
	require.True(t, stored.WorkHistoryClosedThrough.Equal(s.EndedAt))
	snapshot, err = reader.Read(ctx, s.UserID)
	require.NoError(t, err)
	require.Zero(t, stats.SegmentQueries)
	response, err := mypage.Aggregate(snapshot, mypage.Section[mypage.Account]{Availability: mypage.Available, Data: &mypage.Account{DisplayName: "synthetic"}})
	require.NoError(t, err)
	require.Equal(t, int64(timeutil.OverlapSecondsInJSTDay(s.StartedAt, s.EndedAt, snapshot.AsOf)), *response.Summary.Data.Today.WorkSec)
	month := now.In(timeutil.JapanLocation()).Format("2006-01")
	_, err = client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(s.UserID, month)).Delete(ctx)
	require.NoError(t, err)
	snapshot, err = reader.Read(ctx, s.UserID)
	require.NoError(t, err)
	require.Zero(t, stats.SegmentQueries)
	response, err = mypage.Aggregate(snapshot, mypage.Section[mypage.Account]{Availability: mypage.Available, Data: &mypage.Account{DisplayName: "synthetic"}})
	require.NoError(t, err)
	require.Nil(t, response.Summary.Data.Today.WorkSec)
	require.ErrorIs(t, repo.SetWorkMonthReader(ctx, s.UserID, "wrong-checkpoint", user.UpdateTime, false, true), repository.ErrWorkHistoryConflict)
	require.NoError(t, repo.SetWorkMonthReader(ctx, s.UserID, "absent", user.UpdateTime, false, true))
	snapshot, err = reader.Read(ctx, s.UserID)
	require.NoError(t, err)
	require.Equal(t, 1, stats.SegmentQueries)
	require.Empty(t, snapshot.MonthlyEpoch)
}
