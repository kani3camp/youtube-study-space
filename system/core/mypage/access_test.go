package mypage

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/serviceaccess"
)

type accessReaderFunc func(context.Context, string) (serviceaccess.Snapshot, error)

func (f accessReaderFunc) Read(ctx context.Context, uid string) (serviceaccess.Snapshot, error) {
	return f(ctx, uid)
}

func allowedAccess() serviceaccess.Reader {
	return accessReaderFunc(func(context.Context, string) (serviceaccess.Snapshot, error) { return serviceaccess.Snapshot{}, nil })
}

func nowForAccessTest() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC) }

func controlFixture(now time.Time, deletion bool) serviceaccess.Snapshot {
	reason, code := serviceaccess.Moderation, "MODERATION"
	if deletion {
		reason, code = serviceaccess.PrivacyDeletion, "PRIVACY_DELETION"
	}
	control, _, err := serviceaccess.Transition(serviceaccess.Snapshot{}, serviceaccess.Change{Reason: reason, Active: true, ReasonCode: code, Reference: strings.Repeat("a", 64)}, now)
	if err != nil {
		panic("invalid synthetic fixture")
	}
	return serviceaccess.Snapshot{Control: control, Exists: true, Revision: now}
}

func restrictedAccess(now time.Time, deletion bool) serviceaccess.Reader {
	return accessReaderFunc(func(context.Context, string) (serviceaccess.Snapshot, error) {
		return controlFixture(now, deletion), nil
	})
}

type mutableAccess struct {
	mu    sync.Mutex
	value serviceaccess.Snapshot
	err   error
	uids  []string
}

func (r *mutableAccess) Read(_ context.Context, uid string) (serviceaccess.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uids = append(r.uids, uid)
	return r.value, r.err
}

func (r *mutableAccess) set(value serviceaccess.Snapshot, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.value, r.err = value, err
}

func (r *mutableAccess) readUIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.uids...)
}

func TestAccessHTTPReadsOncePerRequestForMissHitAndCurrent(t *testing.T) {
	h, store, verifier, reads := boundaryFixture()
	access := &mutableAccess{}
	h.Auth.Access = access
	for _, path := range []string{"/api/mypage", "/api/mypage", "/api/mypage?view=current", "/api/mypage?view=current"} {
		require.Equal(t, 200, boundaryRequest(h, "GET", path, "").Code)
	}
	verifier.identity.UID = "another-user"
	require.Equal(t, 200, boundaryRequest(h, "GET", "/api/mypage", "").Code)
	require.Equal(t, []string{"synthetic", "synthetic", "synthetic", "synthetic", "another-user"}, access.readUIDs())
	require.Equal(t, 5, store.reads)
	require.Equal(t, 3, *reads, "both views and each uid need their own snapshot cache")
}

func TestAccessHTTPDeniesBeforeAccountOrWarmCache(t *testing.T) {
	for _, deletion := range []bool{false, true} {
		h, store, _, reads := boundaryFixture()
		access := &mutableAccess{}
		h.Auth.Access = access
		require.Equal(t, 200, boundaryRequest(h, "GET", "/api/mypage", "").Code)
		access.set(controlFixture(nowForAccessTest(), deletion), nil)
		before := store.reads
		w := boundaryRequest(h, "GET", "/api/mypage", "")
		require.Equal(t, 403, w.Code)
		want := "SERVICE_ACCESS_RESTRICTED"
		if deletion {
			want = "DATA_DELETION_IN_PROGRESS"
		}
		require.Equal(t, want, responseCode(t, w))
		require.Equal(t, before, store.reads)
		require.Equal(t, 1, *reads)
		require.Equal(t, []string{"synthetic", "synthetic"}, access.readUIDs())
		require.NotContains(t, w.Body.String(), "MODERATION")
		require.NotContains(t, w.Body.String(), strings.Repeat("a", 64))
	}
}

func TestAccessReadFailureCannotBecomePartial200(t *testing.T) {
	h, store, _, reads := boundaryFixture()
	h.Auth.Access = accessReaderFunc(func(context.Context, string) (serviceaccess.Snapshot, error) {
		return serviceaccess.Snapshot{}, errors.New("synthetic private SDK error")
	})
	w := boundaryRequest(h, "GET", "/api/mypage", "")
	require.Equal(t, 503, w.Code)
	require.Equal(t, "TEMPORARY_UNAVAILABLE", responseCode(t, w))
	require.Zero(t, store.reads)
	require.Zero(t, *reads)
	require.NotContains(t, w.Body.String(), "SDK")
	_, err := readAccess(context.Background(), h.Auth.Access, "synthetic", "")
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(err))
	require.True(t, metadataGateError(err))
	require.False(t, metadataGateError(apiError("TEMPORARY_UNAVAILABLE")))
}

func TestLegacyAccountBlockIsCompatibilityOnly(t *testing.T) {
	h, store, _, _ := boundaryFixture()
	store.account.AccessBlocked = true
	require.Equal(t, 200, boundaryRequest(h, "GET", "/api/mypage", "").Code)
}

type accountStoreHook struct {
	AuthStore
	afterRead func()
}

func (s accountStoreHook) ReadAccount(ctx context.Context, uid string) (WebAccount, error) {
	account, err := s.AuthStore.ReadAccount(ctx, uid)
	s.afterRead()
	return account, err
}

func TestAccessRestrictionImmediatelyAfterHTTPGate(t *testing.T) {
	h, store, _, reads := boundaryFixture()
	access := &mutableAccess{}
	h.Auth.Access = access
	h.Auth.Store = accountStoreHook{AuthStore: store, afterRead: func() {
		access.set(controlFixture(nowForAccessTest(), true), nil)
	}}

	inFlight := boundaryRequest(h, "GET", "/api/mypage", "")
	require.Equal(t, 200, inFlight.Code)
	rejected := boundaryRequest(h, "GET", "/api/mypage", "")
	require.Equal(t, 403, rejected.Code)
	require.Equal(t, "DATA_DELETION_IN_PROGRESS", responseCode(t, rejected))
	require.Equal(t, 1, store.reads)
	require.Equal(t, 1, *reads)
	require.Equal(t, []string{"synthetic", "synthetic"}, access.readUIDs())
}

func TestAccessRestrictionDuringSharedSnapshot(t *testing.T) {
	h, store, _, _ := boundaryFixture()
	access := &mutableAccess{}
	h.Auth.Access = access
	started, release := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	defer unblock.Do(func() { close(release) })
	var snapshots atomic.Int32
	h.BFF.Reader = readerFunc(func(ctx context.Context, _ string) (WorkSnapshot, error) {
		if snapshots.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return workFixture(), nil
		case <-ctx.Done():
			return WorkSnapshot{}, ctx.Err()
		}
	})

	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- boundaryRequest(h, "GET", "/api/mypage", "") }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start its snapshot")
	}

	// 先行リクエストが集計中でも、合流するリクエストは個別に判定される。
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	follower := httptest.NewRecorder()
	h.ServeHTTP(follower, newBoundaryRequest(h, "GET", "/api/mypage", "").WithContext(ctx))
	require.Equal(t, 503, follower.Code, "the follower expires while the shared snapshot is still running")
	require.EqualValues(t, 1, snapshots.Load())
	require.Equal(t, 2, store.reads)
	require.Equal(t, []string{"synthetic", "synthetic"}, access.readUIDs())

	access.set(controlFixture(nowForAccessTest(), false), nil)
	rejected := boundaryRequest(h, "GET", "/api/mypage", "")
	require.Equal(t, 403, rejected.Code)
	require.Equal(t, "SERVICE_ACCESS_RESTRICTED", responseCode(t, rejected))
	unblock.Do(func() { close(release) })
	select {
	case inFlight := <-result:
		require.Equal(t, 200, inFlight.Code)
	case <-time.After(time.Second):
		t.Fatal("authorized request did not complete")
	}

	// 制限前に開始した処理がキャッシュを作成しても、後続リクエストは拒否される。
	rejected = boundaryRequest(h, "GET", "/api/mypage", "")
	require.Equal(t, 403, rejected.Code)
	require.Equal(t, []string{"synthetic", "synthetic", "synthetic", "synthetic"}, access.readUIDs())
	require.Equal(t, 2, store.reads)
	require.EqualValues(t, 1, snapshots.Load())
}

func TestAccessGenerationChangeDoesNotReuseWarmCache(t *testing.T) {
	h, _, _, reads := boundaryFixture()
	access := &mutableAccess{}
	h.Auth.Access = access
	require.Equal(t, 200, boundaryRequest(h, "GET", "/api/mypage", "").Code)

	// 制限開始時にアクセスがなければ、キャッシュを無効化する403も発生しない。
	blocked := controlFixture(nowForAccessTest(), false)
	unblocked, _, err := serviceaccess.Transition(blocked, serviceaccess.Change{Reason: serviceaccess.Moderation}, blocked.Revision.Add(time.Second))
	require.NoError(t, err)
	access.set(serviceaccess.Snapshot{Control: unblocked, Exists: true, Revision: unblocked.UpdatedAt}, nil)
	require.Equal(t, 200, boundaryRequest(h, "GET", "/api/mypage", "").Code)
	require.Equal(t, 2, *reads, "new access generation must fetch a fresh snapshot")
	require.Equal(t, []string{"synthetic", "synthetic"}, access.readUIDs())
}
