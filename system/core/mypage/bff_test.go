package mypage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type readerFunc func(context.Context, string) (WorkSnapshot, error)

func (f readerFunc) Read(ctx context.Context, uid string) (WorkSnapshot, error) { return f(ctx, uid) }

func healthyAccount(now time.Time) WebAccount {
	return WebAccount{PrivacyPolicyVersion: "p1", TermsVersion: "t1", DisplayName: "Sample", MetadataFetchedAt: now}
}

func TestBFFCacheExpiryBoundaryAndFailure(t *testing.T) {
	now := instant("2026-10-05T14:59:50Z")
	calls := 0
	fail := false
	b := BFF{Environment: "demo", Now: func() time.Time { return now }, Reader: readerFunc(func(_ context.Context, uid string) (WorkSnapshot, error) {
		calls++
		if fail {
			return WorkSnapshot{}, errors.New("private dependency detail")
		}
		s := workFixture()
		s.AsOf = now
		s.UID = uid
		return s, nil
	})}
	get := func(uid string) Response {
		t.Helper()
		r, err := b.Get(context.Background(), uid, healthyAccount(now))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	first := get("sample-a")
	now = now.Add(9 * time.Second)
	if r := get("sample-a"); calls != 1 || !r.GeneratedAt.Equal(first.GeneratedAt) {
		t.Fatal("cache changed snapshot time")
	}
	now = now.Add(time.Second)
	get("sample-a")
	if calls != 2 {
		t.Fatal("JST day boundary reused yesterday")
	}
	get("sample-b")
	if calls != 3 {
		t.Fatal("cache crossed identity")
	}
	now = now.Add(30 * time.Second)
	fail = true
	for range 2 {
		if _, err := b.Get(context.Background(), "sample-a", healthyAccount(now)); errorCode(err) != "TEMPORARY_UNAVAILABLE" {
			t.Fatalf("error=%v", err)
		}
	}
	if calls != 5 {
		t.Fatal("failed dependency was cached")
	}
	fail = false
	get("sample-a")
	b.Invalidate("sample-a")
	get("sample-a")
	if calls != 7 {
		t.Fatal("invalidation did not evict")
	}
}

func TestBFFSingleflightCallerCancellation(t *testing.T) {
	now := workFixture().AsOf
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	b := BFF{Environment: "demo", Now: func() time.Time { return now }, Reader: readerFunc(func(ctx context.Context, _ string) (WorkSnapshot, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return workFixture(), nil
		case <-ctx.Done():
			return WorkSnapshot{}, ctx.Err()
		}
	})}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := b.Get(ctx, "sample", healthyAccount(now)); first <- err }()
	<-entered
	cancel()
	if errorCode(<-first) != "TEMPORARY_UNAVAILABLE" {
		t.Fatal("disconnected caller did not return")
	}
	second := make(chan error, 1)
	go func() { _, err := b.Get(context.Background(), "sample", healthyAccount(now)); second <- err }()
	close(release)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("shared snapshot was cancelled or repeated")
	}
}

func TestAccountMetadataAge(t *testing.T) {
	now := workFixture().AsOf
	for _, tc := range []struct {
		age    time.Duration
		want   Availability
		reason string
	}{
		{23 * time.Hour, Available, ""}, {24 * time.Hour, Partial, MetadataRefreshFailed}, {30 * 24 * time.Hour, Unavailable, MetadataTooOld}, {-time.Second, Unavailable, MetadataTooOld},
	} {
		a := healthyAccount(now)
		a.MetadataFetchedAt = now.Add(-tc.age)
		s := accountSection(a, now)
		if s.Availability != tc.want || (tc.reason != "" && (s.ReasonCode == nil || *s.ReasonCode != tc.reason)) {
			t.Fatalf("age=%s section=%+v", tc.age, s)
		}
	}
}
