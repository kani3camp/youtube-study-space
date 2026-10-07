package mypage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"app.modules/core/supportdelete"

	"cloud.google.com/go/firestore"

	"github.com/stretchr/testify/require"
)

const runtimeChannel = "UCsynthetic0000000000001"

type runtimeEffectsFunc func(context.Context, supportdelete.Operation) (supportdelete.Evidence, error)

func (f runtimeEffectsFunc) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	return f(ctx, op)
}

func runtimeExecution(now time.Time) supportdelete.Execution {
	return supportdelete.Execution{
		Selector: supportdelete.Selector{Target: supportdelete.Target{Environment: "development", ProjectID: "demo-mypage", ChannelID: runtimeChannel}, RequestRef: strings.Repeat("a", 64), ExecutionRef: strings.Repeat("b", 64), ProofRef: strings.Repeat("c", 64)},
		Manifest: supportdelete.Manifest{Ref: strings.Repeat("d", 64), Mode: "mock"}, OwnerRef: strings.Repeat("e", 64), Revision: 1, Generation: 3, GuardSince: now, Cutoff: now, AcceptedAt: now.Add(-time.Hour), DeleteBy: now.Add(-time.Hour).Add(7 * 24 * time.Hour), UpdatedAt: now, EvidenceDigest: strings.Repeat("f", 64),
	}
}

func runtimeOperation(e supportdelete.Execution, action string) supportdelete.Operation {
	for cursor, step := range supportdelete.Steps() {
		if step.Action == action && step.Scope == "runtime-my-page" {
			e.Cursor, e.Revision = cursor, max(e.Revision, int64(cursor+1))
			return supportdelete.NewOperation(e, e.UpdatedAt)
		}
	}
	panic("missing runtime step")
}

func runtimeEvidence(op supportdelete.Operation, now time.Time) supportdelete.Evidence {
	e := op.Execution
	return supportdelete.Evidence{Selector: e.Selector, ManifestRef: e.Manifest.Ref, OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope, Mode: e.Manifest.Mode, TraceRef: strings.Repeat("1", 64), Generation: e.Generation, Cutoff: e.Cutoff, ObservedAt: now, Known: true, Complete: true, AllInstances: true, OldQueueRejected: true}
}

func runtimeAdapter(r *RuntimeRegistry, now time.Time) *RuntimeDrainEffects {
	return &RuntimeDrainEffects{Registry: r, Clock: func() time.Time { return now }, FleetGuard: runtimeEffectsFunc(func(_ context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
		return runtimeEvidence(op, now), nil
	})}
}

func runtimeApply(t *testing.T, a *RuntimeDrainEffects, e supportdelete.Execution, action string) {
	t.Helper()
	op := runtimeOperation(e, action)
	v, err := a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.NoError(t, v.Validate(op, a.Clock()))
}

func awaitRuntimeIdle(t *testing.T, r *RuntimeRegistry) {
	t.Helper()
	require.Eventually(t, func() bool { o := r.Observe(runtimeChannel); return o.Active == 0 }, time.Second, time.Millisecond)
}

type runtimeAuthStore struct {
	AuthStore
	tx       OAuthTransaction
	verified atomic.Int32
	failed   atomic.Int32
	consumed atomic.Int32
}

func (s *runtimeAuthStore) ReadRuntimeTransaction(context.Context, string) (OAuthTransaction, error) {
	return s.tx, nil
}

func (*runtimeAuthStore) Claim(context.Context, string, string, time.Time) error { return nil }

func (s *runtimeAuthStore) Fail(context.Context, string) error { s.failed.Add(1); return nil }

func (s *runtimeAuthStore) Verify(context.Context, string, Channel, string, time.Time) error {
	s.verified.Add(1)
	return nil
}

func (s *runtimeAuthStore) ReadVerified(context.Context, string, time.Time) (OAuthTransaction, error) {
	return s.tx, nil
}

func (s *runtimeAuthStore) Consume(context.Context, string, string, string, Policy, time.Time) (Channel, error) {
	s.consumed.Add(1)
	return s.tx.Channel, nil
}

type runtimeOAuthFunc func(context.Context, string) ([]Channel, error)

func (runtimeOAuthFunc) AuthorizationURL(string) string { return "https://example.invalid" }

func (f runtimeOAuthFunc) Resolve(ctx context.Context, code string) ([]Channel, error) {
	return f(ctx, code)
}

type runtimeMintFunc func(context.Context, string) (string, error)

func (f runtimeMintFunc) Mint(ctx context.Context, uid string) (string, error) { return f(ctx, uid) }

func runtimeAuth(r *RuntimeRegistry, now time.Time, store *runtimeAuthStore) *AuthService {
	return &AuthService{Runtime: r, Access: allowedAccess(), Store: store, Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now }}
}

func TestRuntimeActualUnresolvedCallbackBlocksEveryTargetUntilChannelBinding(t *testing.T) {
	for _, resolveTarget := range []bool{true, false} {
		t.Run(map[bool]string{true: "paused target", false: "unrelated target"}[resolveTarget], func(t *testing.T) {
			now := workFixture().AsOf
			r := NewRuntimeRegistry("development", "demo-mypage")
			store := &runtimeAuthStore{tx: OAuthTransaction{CreatedAt: now}}
			s := runtimeAuth(r, now, store)
			entered, release := make(chan struct{}), make(chan struct{})
			s.Provider = runtimeOAuthFunc(func(context.Context, string) ([]Channel, error) {
				close(entered)
				<-release
				channel := runtimeChannel
				if !resolveTarget {
					channel = "UCsynthetic0000000000002"
				}
				return []Channel{{ID: channel, DisplayName: "Synthetic"}}, nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- s.Callback(ctx, "synthetic", "synthetic", "synthetic", false) }()
			<-entered
			defer cancel()
			if resolveTarget {
				cancel()
			}
			require.EqualValues(t, 1, r.Observe(runtimeChannel).Unattributed)
			require.EqualValues(t, 1, r.Observe("UCsynthetic0000000000002").Unattributed)
			a, e := runtimeAdapter(r, now), runtimeExecution(now)
			runtimeApply(t, a, e, "pause")
			_, err := a.Apply(context.Background(), runtimeOperation(e, "drain"))
			require.ErrorIs(t, err, supportdelete.ErrEvidence)
			close(release)
			if resolveTarget {
				require.Error(t, <-done, "canceled callback must not publish verification")
				require.Zero(t, store.verified.Load())
			} else {
				require.NoError(t, <-done, "binding an unrelated channel preserves its verification")
				require.EqualValues(t, 1, store.verified.Load())
			}
			runtimeApply(t, a, e, "drain")
			runtimeApply(t, a, e, "resume")
		})
	}
}

func TestRuntimeCallbackDefinitiveFailureRecordsFailAndUnknownFailureStaysPending(t *testing.T) {
	for _, providerErr := range []error{apiError("OAUTH_FAILED"), ErrRuntimeOutcomeUnknown} {
		now := workFixture().AsOf
		r := NewRuntimeRegistry("development", "demo-mypage")
		store := &runtimeAuthStore{tx: OAuthTransaction{CreatedAt: now}}
		s := runtimeAuth(r, now, store)
		s.Provider = runtimeOAuthFunc(func(context.Context, string) ([]Channel, error) { return nil, providerErr })
		err := s.Callback(context.Background(), "synthetic", "synthetic", "synthetic", false)
		require.Error(t, err)
		require.EqualValues(t, 1, store.failed.Load(), "provider verdict must retain failure bookkeeping")
		require.Zero(t, r.Observe(runtimeChannel).Active)
		if errors.Is(providerErr, ErrRuntimeOutcomeUnknown) {
			require.EqualValues(t, 1, r.Observe(runtimeChannel).Pending)
		} else {
			require.Zero(t, r.Observe(runtimeChannel).Pending)
			require.Equal(t, "OAUTH_FAILED", errorCode(err))
		}
	}
}

func TestRuntimeMintOutlivesCallerCancellationAndRejectsItsLateToken(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	store := &runtimeAuthStore{tx: OAuthTransaction{CreatedAt: now, Channel: Channel{ID: runtimeChannel}}}
	s := runtimeAuth(r, now, store)
	entered, release := make(chan struct{}), make(chan struct{})
	s.Minter = runtimeMintFunc(func(context.Context, string) (string, error) {
		close(entered)
		<-release
		return "synthetic-old-token", nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan ConfirmResponse, 1)
	go func() {
		response, err := s.ConfirmResult(ctx, "synthetic", "synthetic")
		require.Error(t, err)
		done <- response
	}()
	<-entered
	cancel()
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	runtimeApply(t, a, e, "pause")
	_, err := a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
	require.Positive(t, r.Observe(runtimeChannel).Active)
	close(release)
	require.Empty(t, (<-done).CustomToken)
	runtimeApply(t, a, e, "drain")
	runtimeApply(t, a, e, "resume")
	store.tx.CreatedAt = now.Add(time.Second)
	s.Now = func() time.Time { return now.Add(time.Second) }
	s.Minter = runtimeMintFunc(func(context.Context, string) (string, error) { return "synthetic-fresh-token", nil })
	token, err := s.Confirm(context.Background(), "synthetic-fresh", "synthetic")
	require.NoError(t, err)
	require.Equal(t, "synthetic-fresh-token", token)
}

func TestRuntimeUnknownMintReturnCannotProveDrainEvenAfterCallerEnds(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	store := &runtimeAuthStore{tx: OAuthTransaction{CreatedAt: now, Channel: Channel{ID: runtimeChannel}}}
	s := runtimeAuth(r, now, store)
	s.Minter = runtimeMintFunc(func(context.Context, string) (string, error) { return "", context.DeadlineExceeded })
	_, err := s.Confirm(context.Background(), "synthetic", "synthetic")
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(err))
	require.Equal(t, RuntimeObservation{Pending: 1}, r.Observe(runtimeChannel))
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	runtimeApply(t, a, e, "pause")
	for _, action := range []string{"drain", "resume"} {
		_, err = a.Apply(context.Background(), runtimeOperation(e, action))
		require.ErrorIs(t, err, supportdelete.ErrEvidence)
	}
}

func TestRuntimeDetachedBFFCannotPublishOldCacheAndFreshWorkSurvivesResume(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	b := &BFF{Runtime: r, Access: allowedAccess(), Environment: "development", Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		snapshot := workFixture()
		snapshot.UID = runtimeChannel
		return snapshot, nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := b.Get(ctx, runtimeChannel, healthyAccount(now)); done <- err }()
	<-entered
	cancel()
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(<-done))
	require.EqualValues(t, 1, r.Observe(runtimeChannel).Active)
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	runtimeApply(t, a, e, "pause")
	_, err := a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
	close(release)
	awaitRuntimeIdle(t, r)
	runtimeApply(t, a, e, "drain")
	runtimeApply(t, a, e, "resume")
	for range 2 {
		_, err := b.Get(context.Background(), runtimeChannel, healthyAccount(now))
		require.NoError(t, err)
	}
	require.EqualValues(t, 2, calls.Load(), "only the fresh result may populate the resumed cache")
}

func TestRuntimeOldCacheAndCompletedResultReplayAreRejectedAfterResume(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	ctx, oldResult, err := beginRuntime(context.Background(), r, runtimeChannel)
	require.NoError(t, err)
	oldResult.done()
	var calls atomic.Int32
	b := &BFF{Runtime: r, Access: allowedAccess(), Environment: "development", Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { calls.Add(1); return workFixture(), nil })}
	_, err = b.Get(context.Background(), runtimeChannel, healthyAccount(now))
	require.NoError(t, err)
	awaitRuntimeIdle(t, r)
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	for _, action := range []string{"pause", "drain", "resume"} {
		runtimeApply(t, a, e, action)
	}
	w := httptest.NewRecorder()
	writeRuntimeResponse(ctx, w, "synthetic", func() error {
		return encodeRuntimeJSON(w, 200, ConfirmResponse{Purpose: "login", CustomToken: "synthetic-old-token"})
	})
	require.Equal(t, 503, w.Code)
	require.NotContains(t, w.Body.String(), "synthetic-old-token")
	_, err = b.Get(context.Background(), runtimeChannel, healthyAccount(now))
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load(), "old cached aggregate cannot survive a pause/resume cycle")
}

type runtimeBlockingWriter struct {
	*httptest.ResponseRecorder
	entered, release chan struct{}
}

func (w *runtimeBlockingWriter) Write(p []byte) (int, error) {
	close(w.entered)
	<-w.release
	n, err := w.ResponseRecorder.Write(p)
	if err != nil {
		return n, fmt.Errorf("write synthetic response: %w", err)
	}
	return n, nil
}

func TestRuntimeActualHTTPTokenDeliveryRemainsTrackedUntilWriterFinishes(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	store := &runtimeAuthStore{tx: OAuthTransaction{CreatedAt: now, Channel: Channel{ID: runtimeChannel}}}
	s := runtimeAuth(r, now, store)
	s.Minter = runtimeMintFunc(func(context.Context, string) (string, error) { return "synthetic-custom-token", nil })
	h := &HTTPHandler{Runtime: r, Auth: s, Verifier: &boundaryVerifier{}, PublicOrigin: "https://mypage.example.test"}
	request := httptest.NewRequest(http.MethodPost, "https://mypage.example.test/api/auth/youtube/confirm", strings.NewReader(`{"confirmationRef":"synthetic"}`))
	request.Header.Set("Origin", h.PublicOrigin)
	request.Header.Set("X-Firebase-AppCheck", "synthetic")
	request.Header.Set("Content-Type", "application/json")
	w := &runtimeBlockingWriter{ResponseRecorder: httptest.NewRecorder(), entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() { defer close(done); h.ServeHTTP(w, request) }()
	<-w.entered
	require.EqualValues(t, 1, r.Observe(runtimeChannel).Active, "library return cannot finish HTTP delivery")
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Apply(ctx, runtimeOperation(e, "pause"))
	require.ErrorIs(t, err, supportdelete.ErrUnavailable, "pause cannot acknowledge while an admitted write is blocked")
	close(w.release)
	<-done
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "synthetic-custom-token")
	for _, action := range []string{"pause", "drain", "resume"} {
		runtimeApply(t, a, e, action)
	}
}

func TestRuntimeActualMetadataRefreshRejectsLateProviderWrite(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	entered, release := make(chan struct{}), make(chan struct{})
	var writes atomic.Int32
	refresh := &AccountMetadataRefresh{Runtime: r, Access: allowedAccess(), Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now }, Provider: metadataReaderFunc(func(context.Context, string) (Channel, error) {
		close(entered)
		<-release
		return Channel{ID: runtimeChannel, DisplayName: "Synthetic"}, nil
	}), Store: metadataWriterFunc(func(context.Context, string, WebAccount, Channel, Policy, time.Time) (WebAccount, error) {
		writes.Add(1)
		return healthyAccount(now), nil
	})}
	account := healthyAccount(now)
	account.Revision = now
	done := make(chan error, 1)
	go func() { _, err := refresh.Refresh(context.Background(), runtimeChannel, account); done <- err }()
	<-entered
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	runtimeApply(t, a, e, "pause")
	close(release)
	require.ErrorIs(t, <-done, ErrRuntimeFenced)
	require.Zero(t, writes.Load())
	runtimeApply(t, a, e, "drain")
	runtimeApply(t, a, e, "resume")
}

func TestRuntimeCleanupDetachedCancellationObservationBlocksDrain(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	entered, release := make(chan struct{}), make(chan struct{})
	job := &MetadataCleanupJob{Runtime: r, Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 1, Store: cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
		return MetadataCleanupPage{}, nil
	}}, Recorder: cleanupRecorderFunc(func(ctx context.Context, _ MetadataCleanupObservation) error {
		require.NoError(t, ctx.Err())
		close(entered)
		<-release
		return nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := job.Run(ctx); done <- err }()
	<-entered
	cancel()
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	runtimeApply(t, a, e, "pause")
	_, err := a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
	require.EqualValues(t, 1, r.Observe(runtimeChannel).Unattributed)
	close(release)
	require.NoError(t, <-done)
	runtimeApply(t, a, e, "drain")
}

func TestRuntimeLocalCountsNeverSubstituteForIndependentFullFleetEvidence(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	e := runtimeExecution(now)
	a := &RuntimeDrainEffects{Registry: r, Clock: func() time.Time { return now }}
	_, err := a.Apply(context.Background(), runtimeOperation(e, "pause"))
	require.ErrorIs(t, err, supportdelete.ErrUnavailable)
	require.False(t, r.Observe(runtimeChannel).Paused)
	for _, change := range []func(*supportdelete.Evidence){func(v *supportdelete.Evidence) { v.AllInstances = false }, func(v *supportdelete.Evidence) { v.ManifestRef = strings.Repeat("2", 64) }, func(v *supportdelete.Evidence) { v.Selector.ProofRef = strings.Repeat("3", 64) }, func(v *supportdelete.Evidence) { v.Generation++ }, func(v *supportdelete.Evidence) { v.Cutoff = v.Cutoff.Add(time.Second) }, func(v *supportdelete.Evidence) { v.OperationID = strings.Repeat("4", 64) }} {
		a.FleetGuard = runtimeEffectsFunc(func(_ context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
			v := runtimeEvidence(op, now)
			change(&v)
			return v, nil
		})
		_, err := a.Apply(context.Background(), runtimeOperation(e, "pause"))
		require.ErrorIs(t, err, supportdelete.ErrEvidence)
		require.False(t, r.Observe(runtimeChannel).Paused)
	}
}

func TestRuntimeMultipleMockWorkersAndSecondDeletionRejectOldCaseReplay(t *testing.T) {
	now := workFixture().AsOf
	workers := []*RuntimeRegistry{NewRuntimeRegistry("development", "demo-mypage"), NewRuntimeRegistry("development", "demo-mypage")}
	e := runtimeExecution(now)
	_, work, err := beginRuntime(context.Background(), workers[1], runtimeChannel)
	require.NoError(t, err)
	for _, worker := range workers {
		runtimeApply(t, runtimeAdapter(worker, now), e, "pause")
	}
	guard := runtimeEffectsFunc(func(_ context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
		for _, worker := range workers {
			o := worker.Observe(runtimeChannel)
			if o.Active != 0 || o.Pending != 0 || !o.Paused {
				return supportdelete.Evidence{}, supportdelete.ErrEvidence
			}
		}
		return runtimeEvidence(op, now), nil
	})
	a := runtimeAdapter(workers[0], now)
	a.FleetGuard = guard
	_, err = a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence, "an idle worker cannot speak for its busy peer")
	work.done()
	runtimeApply(t, a, e, "drain")
	for _, worker := range workers {
		runtimeApply(t, runtimeAdapter(worker, now), e, "resume")
	}
	next := e
	next.Selector.RequestRef = strings.Repeat("2", 64)
	next.Selector.ExecutionRef = strings.Repeat("3", 64)
	next.Selector.ProofRef = strings.Repeat("4", 64)
	next.Generation++
	next.GuardSince = now.Add(time.Hour)
	next.Cutoff = next.GuardSince
	next.UpdatedAt = next.Cutoff
	for _, worker := range workers {
		adapter := runtimeAdapter(worker, next.UpdatedAt)
		runtimeApply(t, adapter, next, "pause")
		for _, action := range []string{"pause", "drain", "resume"} {
			_, err := adapter.Apply(context.Background(), runtimeOperation(e, action))
			require.ErrorIs(t, err, supportdelete.ErrConflict)
		}
		runtimeApply(t, adapter, next, "drain")
		runtimeApply(t, adapter, next, "resume")
		_, fresh, err := beginRuntime(context.Background(), worker, runtimeChannel)
		require.NoError(t, err)
		runtimeApply(t, adapter, next, "resume")
		require.NoError(t, fresh.check(), "stable resume retry cannot fence legitimate fresh work")
		fresh.done()
	}
}

func TestRuntimeTrustedOwnerRecoveryPreservesCutoffAndPendingWork(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	_, work, err := beginRuntime(context.Background(), r, runtimeChannel)
	require.NoError(t, err)
	runtimeApply(t, a, e, "pause")
	cut := runtimeVersion(work)
	recovered := e
	recovered.OwnerRef, recovered.Revision = strings.Repeat("5", 64), 4
	// Higher revision/owner input alone cannot authorize a takeover.
	a.FleetGuard = runtimeEffectsFunc(func(context.Context, supportdelete.Operation) (supportdelete.Evidence, error) {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	})
	_, err = a.Apply(context.Background(), runtimeOperation(recovered, "pause"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
	require.Equal(t, e.OwnerRef, r.fences[runtimeChannel].execution.OwnerRef)
	a = runtimeAdapter(r, now)
	runtimeApply(t, a, recovered, "pause")
	require.Equal(t, cut, runtimeVersion(work), "recovery must retain the old result barrier")
	_, err = a.Apply(context.Background(), runtimeOperation(recovered, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence, "recovery cannot drop an active worker")
	work.done()
	runtimeApply(t, a, recovered, "drain")
	_, err = a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	runtimeApply(t, a, recovered, "resume")
	require.ErrorIs(t, work.check(), ErrRuntimeFenced)
	_, fresh, err := beginRuntime(context.Background(), r, runtimeChannel)
	require.NoError(t, err)
	require.NoError(t, fresh.check())
	fresh.done()
}

func TestRuntimeResumeReplayAdvancesOwnerRevisionAndGenerationHighWater(t *testing.T) {
	now := workFixture().AsOf
	r := NewRuntimeRegistry("development", "demo-mypage")
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	for _, action := range []string{"pause", "drain", "resume"} {
		runtimeApply(t, a, e, action)
	}
	newer := e
	newer.Generation, newer.Revision, newer.OwnerRef = e.Generation+1, 40, strings.Repeat("6", 64)
	runtimeApply(t, a, newer, "resume")
	_, err := a.Apply(context.Background(), runtimeOperation(e, "resume"))
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	require.Equal(t, newer.OwnerRef, r.fences[runtimeChannel].execution.OwnerRef)
	require.Equal(t, newer.Generation, r.fences[runtimeChannel].execution.Generation)
	_, fresh, err := beginRuntime(context.Background(), r, runtimeChannel)
	require.NoError(t, err)
	require.NoError(t, fresh.check())
	fresh.done()
}

type runtimeFailingFlushWriter struct{ *httptest.ResponseRecorder }

func (*runtimeFailingFlushWriter) FlushError() error { return ErrRuntimeOutcomeUnknown }

func TestRuntimeHTTPFlushFailureRetainsUnknownDeliveryBeyondHandlerReturn(t *testing.T) {
	r := NewRuntimeRegistry("development", "demo-mypage")
	ctx, work, err := beginRuntime(context.Background(), r, runtimeChannel)
	require.NoError(t, err)
	w := &runtimeFailingFlushWriter{httptest.NewRecorder()}
	writeRuntimeResponse(ctx, w, "synthetic", func() error {
		return encodeRuntimeJSON(w, 200, ConfirmResponse{Purpose: "login", CustomToken: "synthetic"})
	})
	work.done()
	require.Equal(t, RuntimeObservation{Pending: 1}, r.Observe(runtimeChannel))
	now := workFixture().AsOf
	a, e := runtimeAdapter(r, now), runtimeExecution(now)
	runtimeApply(t, a, e, "pause")
	_, err = a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
}

func TestRuntimeMissingOrFutureCallbackProvenanceFailsBeforeProvider(t *testing.T) {
	now := workFixture().AsOf
	for _, created := range []time.Time{{}, now.Add(time.Second)} {
		r := NewRuntimeRegistry("development", "demo-mypage")
		store := &runtimeAuthStore{tx: OAuthTransaction{CreatedAt: created}}
		s := runtimeAuth(r, now, store)
		var calls atomic.Int32
		s.Provider = runtimeOAuthFunc(func(context.Context, string) ([]Channel, error) {
			calls.Add(1)
			return []Channel{{ID: runtimeChannel, DisplayName: "Synthetic"}}, nil
		})
		require.ErrorIs(t, s.Callback(context.Background(), "synthetic", "synthetic", "synthetic", false), ErrRuntimeFenced)
		require.Zero(t, calls.Load())
		require.Zero(t, store.verified.Load())
	}
}

func TestRuntimeServerInjectsOneRegistryIntoEveryConstructedPathWithoutIO(t *testing.T) {
	config := ServerConfig{Environment: "development", ProjectID: "demo-mypage", ProjectNumber: "123456789", WebAppID: "1:123456789:web:synthetic", PublicOrigin: "https://example.invalid", Policy: Policy{Privacy: "synthetic-p", Terms: "synthetic-t"}}
	r := NewRuntimeRegistry(config.Environment, config.ProjectID)
	deps := ServerDependencies{Runtime: r, Firestore: &firestore.Client{}, Firebase: &fakeFirebaseClient{}, OAuth: serverTestOAuth{}, PublicMetadata: metadataReaderFunc(func(context.Context, string) (Channel, error) { panic("constructor must not invoke provider") })}
	h, err := NewMyPageServer(config, deps)
	require.NoError(t, err)
	require.Same(t, r, h.Runtime)
	require.Same(t, r, h.Auth.Runtime)
	require.Same(t, r, h.BFF.Runtime)
	refresh, ok := h.BFF.Metadata.(*AccountMetadataRefresh)
	require.True(t, ok)
	require.Same(t, r, refresh.Runtime)
	deps.Runtime = NewRuntimeRegistry(config.Environment, "demo-other")
	_, err = NewMyPageServer(config, deps)
	require.Error(t, err)
}
