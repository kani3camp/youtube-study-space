//go:build integration

package mypage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	"app.modules/core/serviceaccess"
	"app.modules/core/timeutil"
	"app.modules/internal/integrationtest"
)

func historyServiceFixture(t *testing.T, count int, ties bool) (*HistoryService, string, WebAccount, HistoryQuery) {
	t.Helper()
	integrationtest.ResetFirestore(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	uid := "UC" + digest(t.Name())[:22]
	now := time.Now().UTC().Truncate(time.Microsecond)
	today := StatisticsWindows(now).Today
	q := HistoryQuery{From: today.AddDate(0, 0, -1), To: today, PageSize: 2}
	_, err = client.Collection(repository.USERS).Doc(uid).Set(ctx, repository.UserDoc{WorkHistoryEpoch: repository.MonthlyWorkEpoch, WorkHistoryActivatedAt: q.From.Add(-time.Hour), WorkHistoryClosedThrough: now})
	require.NoError(t, err)
	key := q.From.Format("2006-01")
	month := repository.WorkMonthDoc{UserID: uid, Month: key, SchemaVersion: 1, BaselineComplete: true, WriterEpoch: repository.MonthlyWorkEpoch, Days: map[string]repository.WorkMonthDay{}}
	batch := client.Batch()
	for i := 0; i < count; i++ {
		end := q.From.Add(time.Hour + time.Duration(i+1)*time.Minute)
		if ties && i == 1 {
			end = q.From.Add(time.Hour + time.Minute)
		}
		segment := repository.WorkSegmentDoc{UserID: uid, SegmentType: repository.WorkState, WorkName: fmt.Sprintf("synthetic-%03d", i), StartedAt: end.Add(-time.Second), EndedAt: end, DurationSec: 1}
		batch.Create(client.Collection(repository.WorkSegments).Doc(fmt.Sprintf("entry-%03d", i)), segment)
		require.NoError(t, month.Append(segment))
	}
	batch.Set(client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, key)), month)
	_, err = batch.Commit(ctx)
	require.NoError(t, err)
	p := historyPolicy{Enabled: true, StandardDailyUnits: 1000, PremiumDailyUnits: 5000, MaxConcurrent: 2, MaxIndexEntries: 1000}
	_, err = client.Collection(historyPolicyCollection).Doc(historyPolicyID).Set(ctx, p)
	require.NoError(t, err)
	seedNormalMaintenance(t, client, "development", now)
	cursor, err := newHistoryCipher([]byte("synthetic-server-oauth-secret-only"), "development")
	require.NoError(t, err)
	s := &HistoryService{Client: client, Access: &serviceaccess.FirestoreStore{Client: client}, Maintenance: &MaintenanceGate{Client: client, Environment: "development"}, Environment: "development", Cipher: cursor, Now: time.Now}
	return s, uid, WebAccount{AccessCheckpoint: "absent", PrivacyPolicyVersion: "p1", TermsVersion: "t1"}, q
}

func TestHistoryBudgetDoesNotConsumeQuotaWhenMaintenanceIsActive(t *testing.T) {
	s, uid, _, _ := historyServiceFixture(t, 0, false)
	ctx := withMaintenanceRevision(context.Background(), 1)
	p, err := s.loadPolicy(ctx)
	require.NoError(t, err)
	_, err = maintenanceRef(s.Client).Set(ctx, MaintenanceControl{
		Environment: "development", State: MaintenanceActive, Revision: 2,
		UpdatedAt: time.Now().UTC(), MessageTemplate: MaintenanceMessageTemplate,
		ReasonCategory: "PLANNED_MAINTENANCE",
	})
	require.NoError(t, err)
	_, err = s.reserve(ctx, uid, "absent", strings.Repeat("m", 32), digest("maintenance"), p, 100, 1)
	require.Equal(t, "MAINTENANCE_ACTIVE", errorCode(err))
	_, err = s.Client.Collection(historyBudgets).Doc(digest("development\x00" + uid)).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestHistoryCursorPagesHaveStableTiesAndSnapshotDuringConcurrentAdds(t *testing.T) {
	s, uid, account, q := historyServiceFixture(t, 5, true)
	ctx := context.Background()
	first, err := s.Page(ctx, uid, account, q, strings.Repeat("a", 32))
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	require.NotNil(t, first.NextCursor)
	require.False(t, first.SourceConsistent)
	require.Equal(t, []string{"synthetic-000", "synthetic-001"}, []string{first.Items[0].WorkName, first.Items[1].WorkName})
	monthRef := s.Client.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, q.From.Format("2006-01")))
	doc, err := monthRef.Get(ctx)
	require.NoError(t, err)
	month, err := repository.DecodeWorkMonth(doc, uid, q.From.Format("2006-01"))
	require.NoError(t, err)
	batch := s.Client.Batch()
	for i, end := range []time.Time{first.Items[0].EndedAt, q.From.Add(time.Hour + 3*time.Minute + 30*time.Second)} {
		segment := repository.WorkSegmentDoc{UserID: uid, SegmentType: repository.WorkState, WorkName: "late concurrent", StartedAt: end.Add(-time.Second), EndedAt: end, DurationSec: 1}
		batch.Create(s.Client.Collection(repository.WorkSegments).Doc(fmt.Sprintf("entry-late-%d", i)), segment)
		require.NoError(t, month.Append(segment))
	}
	batch.Set(monthRef, month)
	_, err = batch.Commit(ctx)
	require.NoError(t, err)
	q.Cursor = *first.NextCursor
	second, err := s.Page(ctx, uid, account, q, strings.Repeat("b", 32))
	require.NoError(t, err)
	require.Equal(t, first.AsOf, second.AsOf)
	require.Equal(t, []string{"synthetic-002", "synthetic-003"}, []string{second.Items[0].WorkName, second.Items[1].WorkName})
	require.NotNil(t, second.NextCursor)
	q.Cursor = *second.NextCursor
	last, err := s.Page(ctx, uid, account, q, strings.Repeat("c", 32))
	require.NoError(t, err)
	require.Len(t, last.Items, 1)
	require.Equal(t, "synthetic-004", last.Items[0].WorkName)
	require.Nil(t, last.NextCursor)
	q.Cursor = *first.NextCursor
	_, err = s.Page(ctx, "UC"+strings.Repeat("x", 22), account, q, strings.Repeat("d", 32))
	require.Equal(t, "INVALID_CURSOR", errorCode(err))
}

func TestHistoryBudgetIsSharedAcrossInstancesAndReplaysDoNotRunAgain(t *testing.T) {
	s, uid, account, q := historyServiceFixture(t, 3, false)
	ctx := context.Background()
	_, err := s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Update(ctx, []firestore.Update{{Path: "standardDailyUnits", Value: int64(72)}})
	require.NoError(t, err)
	first, err := s.Page(ctx, uid, account, q, strings.Repeat("a", 32))
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	other := *s
	_, err = other.Page(ctx, uid, account, q, strings.Repeat("a", 32))
	require.Equal(t, "OPERATION_ALREADY_CONSUMED", errorCode(err))
	_, err = other.Page(ctx, uid, account, q, strings.Repeat("b", 32))
	require.NoError(t, err)
	_, err = s.Page(ctx, uid, account, q, strings.Repeat("c", 32))
	var wait *HistoryWaitError
	require.ErrorAs(t, err, &wait)
	require.True(t, wait.Daily)
	doc, err := s.Client.Collection(historyBudgets).Doc(digest("development\x00" + uid)).Get(ctx)
	require.NoError(t, err)
	state, err := decodeHistoryBudget(doc, uid)
	require.NoError(t, err)
	require.Equal(t, int64(72), state.Spent)
	require.Len(t, state.Operations, 2)
	require.Empty(t, state.Leases)
}

func TestHistoryBudgetConcurrentReservationNeverOverspendsAndSurvivesRestart(t *testing.T) {
	s, uid, _, _ := historyServiceFixture(t, 0, false)
	ctx := withMaintenanceRevision(context.Background(), 1)
	_, err := s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Update(ctx, []firestore.Update{{Path: "standardDailyUnits", Value: int64(1)}, {Path: "maxConcurrent", Value: int64(4)}})
	require.NoError(t, err)
	p, err := s.loadPolicy(ctx)
	require.NoError(t, err)
	var wg sync.WaitGroup
	success := make(chan historyClaim, 8)
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			copy := *s
			claim, err := copy.reserve(ctx, uid, "absent", fmt.Sprintf("reservation-key-%032d", i), digest(fmt.Sprint(i)), p, 1, 1)
			if err == nil {
				success <- claim
			}
		}(i)
	}
	wg.Wait()
	close(success)
	claims := []historyClaim{}
	for claim := range success {
		claims = append(claims, claim)
	}
	require.Len(t, claims, 1)
	doc, err := s.Client.Collection(historyBudgets).Doc(digest("development\x00" + uid)).Get(ctx)
	require.NoError(t, err)
	state, err := decodeHistoryBudget(doc, uid)
	require.NoError(t, err)
	require.Equal(t, int64(1), state.Spent)
	require.Len(t, state.Leases, 1)
	require.NoError(t, s.release(claims[0]))
}

func TestHistoryLeaseCrossesJSTDayWithoutResettingConcurrencyOrOldKey(t *testing.T) {
	s, uid, _, _ := historyServiceFixture(t, 0, false)
	ctx := withMaintenanceRevision(context.Background(), 1)
	p, err := s.loadPolicy(ctx)
	require.NoError(t, err)
	p.MaxConcurrent = 1
	_, err = s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Update(ctx, []firestore.Update{{Path: "maxConcurrent", Value: int64(1)}})
	require.NoError(t, err)
	p, err = s.loadPolicy(ctx)
	require.NoError(t, err)
	now := time.Now().In(timeutil.JapanLocation())
	oldDay := now.AddDate(0, 0, -1).Format("2006-01-02")
	key := strings.Repeat("a", 32)
	fp := digest("old-operation")
	lease := now.Add(15 * time.Second).UTC().Truncate(time.Microsecond)
	state := historyBudgetState{UID: uid, Day: oldDay, Spent: 1000, Operations: map[string]historyOperation{digest(key): {Fingerprint: fp, Day: oldDay}}, Leases: map[string]time.Time{digest(key): lease}}
	ref := s.Client.Collection(historyBudgets).Doc(digest("development\x00" + uid))
	_, err = ref.Set(ctx, state)
	require.NoError(t, err)
	_, err = s.reserve(ctx, uid, "absent", key, fp, p, 1000, 1)
	require.Equal(t, "OPERATION_ALREADY_CONSUMED", errorCode(err))
	_, err = s.reserve(ctx, uid, "absent", strings.Repeat("b", 32), digest("new-operation"), p, 1000, 1)
	var wait *HistoryWaitError
	require.ErrorAs(t, err, &wait)
	require.False(t, wait.Daily)
	require.NoError(t, s.release(historyClaim{UID: uid, Key: digest(key), Until: lease}))
	claim, err := s.reserve(ctx, uid, "absent", strings.Repeat("b", 32), digest("new-operation"), p, 1000, 1)
	require.NoError(t, err)
	require.NoError(t, s.release(claim))
	doc, err := ref.Get(ctx)
	require.NoError(t, err)
	next, err := decodeHistoryBudget(doc, uid)
	require.NoError(t, err)
	require.Equal(t, now.Format("2006-01-02"), next.Day)
	require.Equal(t, int64(1), next.Spent)
}

type cancelHistoryStream struct {
	grpc.ClientStream
	cancel context.CancelFunc
}

func (s *cancelHistoryStream) SendMsg(message any) error {
	if query, ok := message.(*firestorepb.RunQueryRequest); ok {
		for _, from := range query.GetStructuredQuery().GetFrom() {
			if from.GetCollectionId() == repository.WorkSegments {
				s.cancel()
				return context.Canceled
			}
		}
	}
	if err := s.ClientStream.SendMsg(message); err != nil {
		return fmt.Errorf("send synthetic history RPC: %w", err)
	}
	return nil
}

func TestHistoryFailedOrCancelledWorkKeepsConsumptionButReleasesSlot(t *testing.T) {
	for _, fault := range []string{"preflight", "query", "cancel", "payload"} {
		t.Run(fault, func(t *testing.T) {
			s, uid, account, q := historyServiceFixture(t, 1, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := s.Client
			switch fault {
			case "preflight":
				_, err := original.Collection(repository.WorkMonths).Doc(repository.WorkMonthID(uid, q.From.Format("2006-01"))).Delete(ctx)
				require.NoError(t, err)
			case "payload":
				_, err := original.Collection(repository.WorkSegments).Doc("entry-000").Update(ctx, []firestore.Update{{Path: "work-name", Value: strings.Repeat("x", 121*1024)}})
				require.NoError(t, err)
			case "query", "cancel":
				interceptor := func(ctx context.Context, desc *grpc.StreamDesc, conn *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
					stream, err := streamer(ctx, desc, conn, method, opts...)
					if err != nil {
						return nil, fmt.Errorf("create synthetic history RPC: %w", err)
					}
					if fault == "cancel" {
						return &cancelHistoryStream{ClientStream: stream, cancel: cancel}, nil
					}
					return &snapshotQueryFailureStream{ClientStream: stream, failCollections: map[string]bool{repository.WorkSegments: true}}, nil
				}
				conn, err := grpc.NewClient("passthrough:///"+os.Getenv("FIRESTORE_EMULATOR_HOST"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithPerRPCCredentials(snapshotEmulatorCredentials{}), grpc.WithStreamInterceptor(interceptor))
				require.NoError(t, err)
				client, err := firestore.NewClient(context.Background(), "demo-youtube-study-space-ci", option.WithGRPCConn(conn))
				require.NoError(t, err)
				s.Client = client
				t.Cleanup(func() { require.NoError(t, client.Close()) })
			}
			key := strings.Repeat("a", 32)
			_, err := s.Page(ctx, uid, account, q, key)
			require.Error(t, err)
			doc, err := original.Collection(historyBudgets).Doc(digest("development\x00" + uid)).Get(context.Background())
			require.NoError(t, err)
			state, err := decodeHistoryBudget(doc, uid)
			require.NoError(t, err)
			require.Equal(t, int64(36), state.Spent)
			require.Empty(t, state.Leases)
			require.Len(t, state.Operations, 1)
			s.Client = original
			_, err = s.Page(context.Background(), uid, account, q, key)
			require.Equal(t, "OPERATION_ALREADY_CONSUMED", errorCode(err))
		})
	}
}

func TestHistoryOldCursorRejectsPolicyTierAndSACChangesAndPublicationRechecks(t *testing.T) {
	for _, change := range []string{"policy", "tier", "access", "publication"} {
		t.Run(change, func(t *testing.T) {
			s, uid, account, q := historyServiceFixture(t, 3, false)
			ctx := context.Background()
			first, err := s.Page(ctx, uid, account, q, strings.Repeat("a", 32))
			require.NoError(t, err)
			require.NotNil(t, first.NextCursor)
			q.Cursor = *first.NextCursor
			switch change {
			case "policy":
				_, err := s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Update(ctx, []firestore.Update{{Path: "premiumDailyUnits", Value: int64(4000)}})
				require.NoError(t, err)
			case "tier":
				s.TierFor = func(context.Context, string, WebAccount) (HistoryTier, error) {
					return HistoryTier{Revision: "new-server-entitlement"}, nil
				}
			case "access":
				store := &serviceaccess.FirestoreStore{Client: s.Client}
				_, err := store.Change(ctx, uid, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: true, ReasonCode: "PRIVACY_DELETION", Reference: digest("synthetic-privacy")}, time.Now().UTC())
				require.NoError(t, err)
			case "publication":
				q.Cursor = ""
				calls := 0
				s.TierFor = func(context.Context, string, WebAccount) (HistoryTier, error) {
					calls++
					return HistoryTier{Revision: fmt.Sprintf("revision-%d", calls)}, nil
				}
			}
			result, err := s.Page(ctx, uid, account, q, strings.Repeat("b", 32))
			require.Error(t, err)
			require.Empty(t, result.Items)
			if change == "policy" || change == "tier" {
				require.Equal(t, "INVALID_CURSOR", errorCode(err))
			}
		})
	}
}

func TestHistoryIsOffWithoutPolicyAndInProductionAndRejectsLargeScan(t *testing.T) {
	s, uid, account, q := historyServiceFixture(t, 3, false)
	ctx := context.Background()
	s.Environment = "production"
	_, err := s.Page(ctx, uid, account, q, strings.Repeat("a", 32))
	require.Equal(t, "FEATURE_UNAVAILABLE", errorCode(err))
	s.Environment = "development"
	_, err = s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Update(ctx, []firestore.Update{{Path: "maxIndexEntries", Value: int64(2)}})
	require.NoError(t, err)
	_, err = s.Page(ctx, uid, account, q, strings.Repeat("b", 32))
	require.Equal(t, "HISTORY_LIMIT_EXCEEDED", errorCode(err))
	_, err = s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Delete(ctx)
	require.NoError(t, err)
	_, err = s.Page(ctx, uid, account, q, strings.Repeat("c", 32))
	require.Equal(t, "FEATURE_UNAVAILABLE", errorCode(err))
}

func TestHistoryHTTPPublishesBoundedPageAndDailyResetWithoutBlockingOverview(t *testing.T) {
	s, uid, account, q := historyServiceFixture(t, 3, false)
	h, store, verifier, _ := boundaryFixture()
	h.History = s
	verifier.identity.UID = uid
	store.account.AccessCheckpoint = account.AccessCheckpoint
	path := "/api/mypage/history?from=" + q.From.Format("2006-01-02") + "&to=" + q.To.Format("2006-01-02") + "&limit=2"
	request := func(key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Host = "mypage.example.test"
		r.Header.Set("Origin", h.PublicOrigin)
		r.Header.Set("X-Firebase-AppCheck", "synthetic")
		r.Header.Set("Authorization", "Bearer synthetic")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request(strings.Repeat("a", 32))
	require.Equal(t, 200, w.Code)
	require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
	require.NotContains(t, w.Body.String(), uid)
	_, err := s.Client.Collection(historyPolicyCollection).Doc(historyPolicyID).Update(context.Background(), []firestore.Update{{Path: "standardDailyUnits", Value: int64(1)}})
	require.NoError(t, err)
	w = request(strings.Repeat("b", 32))
	require.Equal(t, 429, w.Code)
	require.Equal(t, "ANALYSIS_BUDGET_EXHAUSTED", responseCode(t, w))
	require.NotEmpty(t, w.Header().Get("Retry-After"))
	require.NotEmpty(t, w.Header().Get("X-Quota-Reset"))
	w = boundaryRequest(h, "GET", "/api/mypage", "")
	require.Equal(t, 200, w.Code, "ordinary overview must not consume the analysis budget")
}
