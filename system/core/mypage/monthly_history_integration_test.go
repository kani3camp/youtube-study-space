//go:build integration

package mypage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
)

func TestMonthlySnapshotReadsAtMostTwoMonthsAndNeverRawHistory(t *testing.T) {
	for _, count := range []int{0, 10, 100, 500, 1000, 1001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			r, uid := snapshotFixture(t)
			r.MonthlyEpoch = "synthetic-v1"
			ctx := context.Background()
			now := time.Now().UTC().Truncate(time.Second)
			r.Coverage = []Interval{{now.AddDate(0, 0, -10), now.Add(time.Hour)}}
			_, err := r.Client.Collection("users").Doc(uid).Set(ctx, repository.UserDoc{TotalStudySec: 6000})
			require.NoError(t, err)
			docs := []*firestore.DocumentRef{r.Client.Collection("users").Doc(uid)}
			months := map[string]repository.WorkMonthDoc{}
			w := StatisticsWindows(now)
			for day := w.QueryStart(); !day.After(now); day = day.AddDate(0, 0, 1) {
				key := day.Format("2006-01")
				months[key] = repository.WorkMonthDoc{UserID: uid, Month: key, SchemaVersion: 1, BaselineComplete: true, WriterEpoch: r.MonthlyEpoch, Days: map[string]repository.WorkMonthDay{}}
			}
			for base := 0; base < count; base += 400 {
				batch := r.Client.Batch()
				for i := base; i < min(base+400, count); i++ {
					start := now.Add(-2 * time.Hour).Add(time.Duration(i) * time.Second)
					s := repository.WorkSegmentDoc{UserID: uid, SegmentType: repository.WorkState, StartedAt: start, EndedAt: start.Add(time.Second), DurationSec: 1}
					ref := r.Client.Collection(repository.WorkSegments).NewDoc()
					docs = append(docs, ref)
					batch.Create(ref, s)
					key := start.In(timeutil.JapanLocation()).Format("2006-01")
					m := months[key]
					require.NoError(t, m.Append(s))
					months[key] = m
				}
				_, err := batch.Commit(ctx)
				require.NoError(t, err)
			}
			for key, m := range months {
				ref := r.Client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, key))
				docs = append(docs, ref)
				_, err := ref.Set(ctx, m)
				require.NoError(t, err)
			}
			t.Cleanup(func() {
				for base := 0; base < len(docs); base += 400 {
					batch := r.Client.Batch()
					for _, ref := range docs[base:min(base+400, len(docs))] {
						batch.Delete(ref)
					}
					_, err := batch.Commit(context.Background())
					require.NoError(t, err)
				}
			})
			var stats SnapshotReadObservation
			r.Observe = func(s SnapshotReadObservation) { stats = s }
			snapshot, err := r.Read(ctx, uid)
			require.NoError(t, err)
			require.Zero(t, stats.SegmentQueries)
			require.Zero(t, stats.SegmentsReturned)
			require.Zero(t, stats.MonthReadErrors)
			require.GreaterOrEqual(t, stats.MonthLookups, 1)
			require.LessOrEqual(t, stats.MonthLookups, 2)
			require.Empty(t, snapshot.Segments)
			response, err := Aggregate(snapshot, fixtureAccount())
			require.NoError(t, err)
			var total int64
			for _, day := range response.Recent7Days.Data {
				require.NotNil(t, day.WorkSec)
				total += *day.WorkSec
			}
			require.Equal(t, int64(count), total)
			require.Equal(t, snapshot.AsOf, response.GeneratedAt)
			// Missing month must not silently run the legacy 1001-doc query.
			for key := range months {
				_, err := r.Client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, key)).Delete(ctx)
				require.NoError(t, err)
			}
			snapshot, err = r.Read(ctx, uid)
			require.NoError(t, err)
			require.Zero(t, stats.SegmentQueries)
			response, err = Aggregate(snapshot, fixtureAccount())
			require.NoError(t, err)
			require.True(t, response.Partial)
		})
	}
}

func TestMonthlySnapshotNeverDecodesMissingNullOrWrongTypedValuesAsZero(t *testing.T) {
	r, uid := snapshotFixture(t)
	r.MonthlyEpoch = "synthetic-v1"
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	r.Coverage = []Interval{{Start: now.AddDate(0, 0, -10), End: now.Add(time.Hour)}}
	_, err := r.Client.Collection(repository.USERS).Doc(uid).Set(ctx, repository.UserDoc{})
	require.NoError(t, err)
	segment := repository.WorkSegmentDoc{UserID: uid, SegmentType: repository.WorkState, StartedAt: now.Add(-time.Minute), EndedAt: now.Add(-30 * time.Second), DurationSec: 30}
	key := segment.StartedAt.In(timeutil.JapanLocation()).Format("2006-01")
	date := segment.StartedAt.In(timeutil.JapanLocation()).Format("2006-01-02")
	month := repository.WorkMonthDoc{UserID: uid, Month: key, SchemaVersion: 1, WriterEpoch: r.MonthlyEpoch, BaselineComplete: true, Days: map[string]repository.WorkMonthDay{}}
	require.NoError(t, month.Append(segment))
	ref := r.Client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, key))
	t.Cleanup(func() {
		_, err := ref.Delete(context.Background())
		require.NoError(t, err)
		_, err = r.Client.Collection(repository.USERS).Doc(uid).Delete(context.Background())
		require.NoError(t, err)
	})
	for _, field := range []string{"work-sec", "cross-day-carry-sec", "completed-work-segments", "source-ended-at", "first", "last", "invalid", "baseline-complete", "writer-epoch", "schema-version"} {
		for _, fault := range []string{"missing", "null", "wrong-type"} {
			t.Run(field+"/"+fault, func(t *testing.T) {
				_, err := ref.Set(ctx, month)
				require.NoError(t, err)
				doc, err := ref.Get(ctx)
				require.NoError(t, err)
				data := doc.Data()
				target := data
				if field != "baseline-complete" && field != "writer-epoch" && field != "schema-version" {
					target = data["days"].(map[string]any)[date].(map[string]any)
				}
				switch fault {
				case "missing":
					delete(target, field)
				case "null":
					target[field] = nil
				case "wrong-type":
					target[field] = []any{"invalid scalar"}
				}
				_, err = ref.Set(ctx, data)
				require.NoError(t, err)
				snapshot, err := r.Read(ctx, uid)
				require.NoError(t, err)
				require.True(t, snapshot.HistoryInconsistent)
				response, err := Aggregate(snapshot, fixtureAccount())
				require.NoError(t, err)
				require.True(t, response.Partial)
				require.Nil(t, response.Summary.Data.Today.WorkSec)
				require.Equal(t, DataInconsistent, *response.Summary.Data.Today.ReasonCode)
			})
		}
	}
}

func TestMonthlyBrokenReaderMarkerCannotSilentlyFallBackToRaw(t *testing.T) {
	r, uid := snapshotFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Hour)
	for _, fault := range []string{"missing-epoch", "null-epoch", "empty-epoch", "zero-watermark", "regressing-watermark"} {
		t.Run(fault, func(t *testing.T) {
			data := map[string]any{"total-study-sec": int64(0), "work-history-epoch": repository.MonthlyWorkEpoch, "work-history-activated-at": now, "work-history-closed-through": now}
			switch fault {
			case "missing-epoch":
				delete(data, "work-history-epoch")
			case "null-epoch":
				data["work-history-epoch"] = nil
			case "empty-epoch":
				data["work-history-epoch"] = ""
			case "zero-watermark":
				data["work-history-closed-through"] = time.Time{}
			case "regressing-watermark":
				data["work-history-closed-through"] = now.Add(-time.Second)
			}
			_, err := r.Client.Collection(repository.USERS).Doc(uid).Set(ctx, data)
			require.NoError(t, err)
			var stats SnapshotReadObservation
			r.Observe = func(s SnapshotReadObservation) { stats = s }
			snapshot, err := r.Read(ctx, uid)
			require.NoError(t, err)
			require.True(t, snapshot.HistoryInconsistent)
			require.Zero(t, stats.SegmentQueries)
			response, err := Aggregate(snapshot, fixtureAccount())
			require.NoError(t, err)
			require.True(t, response.Partial)
			require.Nil(t, response.Summary.Data.Today.WorkSec)
		})
	}
	t.Cleanup(func() {
		_, err := r.Client.Collection(repository.USERS).Doc(uid).Delete(context.Background())
		require.NoError(t, err)
	})
}

func TestMonthlyUnknownLifetimeSourceIsNotReportedAsNormalZero(t *testing.T) {
	r, uid := snapshotFixture(t)
	ctx := context.Background()
	for _, missing := range []bool{false, true} {
		data := map[string]any{"total-study-sec": nil}
		if missing {
			delete(data, "total-study-sec")
		}
		_, err := r.Client.Collection(repository.USERS).Doc(uid).Set(ctx, data)
		require.NoError(t, err)
		snapshot, err := r.Read(ctx, uid)
		require.NoError(t, err)
		require.True(t, snapshot.UserTotalInconsistent)
		response, err := Aggregate(snapshot, fixtureAccount())
		require.NoError(t, err)
		require.NotNil(t, response.Summary.Data)
		require.Nil(t, response.Summary.Data.Lifetime.WorkSec)
		require.Equal(t, DataInconsistent, *response.Summary.Data.Lifetime.ReasonCode)
	}
	t.Cleanup(func() {
		_, err := r.Client.Collection(repository.USERS).Doc(uid).Delete(context.Background())
		require.NoError(t, err)
	})
}
