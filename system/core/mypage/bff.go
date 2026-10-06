package mypage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type (
	cacheKey          struct{ environment, uid string }
	cachedAggregate   struct{ response Response }
	MetadataRefresher interface {
		Refresh(context.Context, string, WebAccount) (WebAccount, error)
	}
)

// BFF is process-memory only. Authorization and the current WebAccount gate are
// checked by the HTTP boundary on every request before consulting this cache.
type BFF struct {
	Reader      SnapshotReader
	Environment string
	Now         func() time.Time
	Metadata    MetadataRefresher
	mu          sync.Mutex
	cache       map[cacheKey]cachedAggregate
	group       singleflight.Group
}

func (b *BFF) cacheHit(key cacheKey, now time.Time) (Response, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	value, ok := b.cache[key]
	if !ok {
		return Response{}, false
	}
	asOf := value.response.GeneratedAt
	if now.Before(asOf) || now.Sub(asOf) >= 30*time.Second || !StatisticsWindows(now).Today.Equal(StatisticsWindows(asOf).Today) {
		delete(b.cache, key)
		return Response{}, false
	}
	return value.response, true
}

func accountSection(account WebAccount, now time.Time) Section[Account] {
	if account.DisplayName == "" {
		if !account.MetadataFetchedAt.IsZero() && now.Sub(account.MetadataFetchedAt) >= 30*24*time.Hour {
			return missingSection[Account](MetadataTooOld)
		}
		return missingSection[Account](SourceUnavailable)
	}
	age := now.Sub(account.MetadataFetchedAt)
	if account.MetadataFetchedAt.IsZero() || age >= 30*24*time.Hour || age < 0 {
		return missingSection[Account](MetadataTooOld)
	}
	data := Account{DisplayName: account.DisplayName, Handle: account.Handle, AvatarURL: account.AvatarURL}
	if age >= 24*time.Hour {
		return Section[Account]{Availability: Partial, ReasonCode: ptr(MetadataRefreshFailed), Data: &data}
	}
	return availableSection(data)
}

func (b *BFF) Get(ctx context.Context, uid string, account WebAccount) (Response, error) {
	key := cacheKey{b.Environment, uid}
	now := b.Now().UTC()
	if value, ok := b.cacheHit(key, now); ok {
		return value, nil
	}
	// The first caller disconnecting must not cancel a request shared with a
	// second caller; the shared operation still has its own bounded budget.
	result := b.group.DoChan(b.Environment+"\x00"+uid, func() (any, error) {
		if value, ok := b.cacheHit(key, b.Now().UTC()); ok {
			return value, nil
		}
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if b.Metadata != nil && (account.MetadataFetchedAt.IsZero() || b.Now().Sub(account.MetadataFetchedAt) >= 24*time.Hour) {
			metadataCtx, stop := context.WithTimeout(workCtx, time.Second)
			if refreshed, err := b.Metadata.Refresh(metadataCtx, uid, account); err == nil {
				account = refreshed
			} else if metadataGateError(err) {
				stop()
				return Response{}, fmt.Errorf("metadata access gate: %w", err)
			}
			stop()
		}
		snapshot, err := b.Reader.Read(workCtx, uid)
		if err != nil {
			return Response{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		response, err := Aggregate(snapshot, accountSection(account, b.Now().UTC()))
		if err != nil {
			return Response{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		b.mu.Lock()
		if b.cache == nil {
			b.cache = make(map[cacheKey]cachedAggregate)
		}
		// Cap attacker-controlled uid cardinality; evict one value at capacity.
		if len(b.cache) >= 1024 {
			for victim := range b.cache {
				delete(b.cache, victim)
				break
			}
		}
		b.cache[key] = cachedAggregate{response: response}
		b.mu.Unlock()
		return response, nil
	})
	select {
	case <-ctx.Done():
		return Response{}, apiError("TEMPORARY_UNAVAILABLE")
	case value := <-result:
		if value.Err != nil {
			if metadataGateError(value.Err) {
				return Response{}, value.Err
			}
			return Response{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		response, ok := value.Val.(Response)
		if !ok {
			return Response{}, apiError("INTERNAL_ERROR")
		}
		return response, nil
	}
}

// Invalidate is available to the server-side account deletion/consent workflow;
// it never substitutes for checking those gates on each request.
func (b *BFF) Invalidate(uid string) {
	b.mu.Lock()
	delete(b.cache, cacheKey{b.Environment, uid})
	b.mu.Unlock()
}

func metadataGateError(err error) bool {
	switch errorCode(err) {
	case "AUTH_REQUIRED", "WEB_ACCOUNT_REQUIRED", "PRIVACY_RECONSENT_REQUIRED":
		return true
	}
	return false
}
