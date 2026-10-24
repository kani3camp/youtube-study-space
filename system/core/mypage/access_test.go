package mypage

import (
	"context"
	"errors"
	"strings"
	"sync"
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
}

func (r *mutableAccess) Read(context.Context, string) (serviceaccess.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.value, r.err
}

func (r *mutableAccess) set(value serviceaccess.Snapshot, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.value, r.err = value, err
}

func TestAccessHTTPDeniesBeforeAccountOrWarmCache(t *testing.T) {
	for _, deletion := range []bool{false, true} {
		h, store, _, reads := boundaryFixture()
		access := &mutableAccess{}
		h.Auth.Access, h.BFF.Access = access, access
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
	b := BFF{Access: h.Auth.Access, Now: nowForAccessTest, Environment: "demo"}
	_, err := b.Get(context.Background(), "synthetic", WebAccount{})
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(err))
	require.True(t, metadataGateError(err))
	require.False(t, metadataGateError(apiError("TEMPORARY_UNAVAILABLE")))
}

func TestLegacyAccountBlockIsCompatibilityOnly(t *testing.T) {
	h, store, _, _ := boundaryFixture()
	store.account.AccessBlocked = true
	require.Equal(t, 200, boundaryRequest(h, "GET", "/api/mypage", "").Code)
}

func TestAccessConcurrentDetachedFlightAndAnotherInstance(t *testing.T) {
	now := workFixture().AsOf
	access := &mutableAccess{}
	started, release := make(chan struct{}), make(chan struct{})
	b := &BFF{Access: access, Environment: "demo", Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) {
		close(started)
		<-release
		return workFixture(), nil
	})}
	other := &BFF{Access: access, Environment: "demo", Now: func() time.Time { return now }, Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { return workFixture(), nil })}
	account := healthyAccount(now)
	require.NotNil(t, other)
	_, err := other.Get(context.Background(), "synthetic", account)
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() { _, err := b.Get(context.Background(), "synthetic", account); result <- err }()
	<-started
	blocked := controlFixture(nowForAccessTest(), false)
	access.set(blocked, nil)
	_, err = other.Get(context.Background(), "synthetic", account)
	require.Equal(t, "SERVICE_ACCESS_RESTRICTED", errorCode(err))
	// A block/unblock cycle still invalidates the old flight's captured decision.
	blocked.Control.Moderation = serviceaccess.ReasonState{}
	blocked.Control.Generation++
	blocked.Revision = blocked.Revision.Add(time.Second)
	access.set(blocked, nil)
	close(release)
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(<-result))
	b.mu.Lock()
	require.Empty(t, b.cache)
	b.mu.Unlock()
	_, err = other.Get(context.Background(), "synthetic", account)
	require.NoError(t, err)
}
