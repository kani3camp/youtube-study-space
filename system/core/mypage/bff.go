package mypage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type (
	cacheKey        struct{ environment, uid string }
	cachedAggregate struct {
		response       Response
		accountVersion string
	}
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

func (b *BFF) cacheHit(key cacheKey, now time.Time, account WebAccount) (Response, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	value, ok := b.cache[key]
	if !ok {
		return Response{}, false
	}
	asOf := value.response.GeneratedAt
	if value.accountVersion != metadataAccountVersion(account) || (accountSection(account, now).Data == nil && value.response.Account.Data != nil) || now.Before(asOf) || now.Sub(asOf) >= 30*time.Second || !StatisticsWindows(now).Today.Equal(StatisticsWindows(asOf).Today) {
		delete(b.cache, key)
		return Response{}, false
	}
	return value.response, true
}

func accountSection(account WebAccount, now time.Time) Section[Account] {
	if account.DisplayName == "" {
		// Empty metadata is terminal even after another process cleared fetchedAt.
		return missingSection[Account](MetadataTooOld)
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
	if value, ok := b.cacheHit(key, now, account); ok {
		return value, nil
	}
	// The HTTP budget starts before authorization and the browser also has a
	// deadline. Reserve time for both flight delivery and response serialization;
	// detaching cancellation must not reset an already partly spent budget.
	workDeadline := time.Now().Add(8 * time.Second)
	waitCtx := ctx
	if outerDeadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(outerDeadline)
		if remaining <= 0 {
			return Response{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		reserve := min(time.Second, remaining/4)
		waitDeadline := outerDeadline.Add(-reserve)
		workDeadline = minDeadline(workDeadline, waitDeadline.Add(-reserve))
		var stop context.CancelFunc
		waitCtx, stop = context.WithDeadline(ctx, waitDeadline)
		defer stop()
	}
	// The first caller disconnecting must not cancel a request shared with a
	// second caller; the shared operation still has its own bounded budget.
	result := b.group.DoChan(b.Environment+"\x00"+uid+"\x00"+metadataAccountVersion(account), func() (any, error) {
		// Refresh only the flight's copy; a caller's early fallback must not race
		// with metadata updates in the detached operation.
		account := account
		if value, ok := b.cacheHit(key, b.Now().UTC(), account); ok {
			return value, nil
		}
		workCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), workDeadline)
		defer cancel()
		if b.Metadata != nil && (account.MetadataFetchedAt.IsZero() || b.Now().Sub(account.MetadataFetchedAt) >= 24*time.Hour) {
			metadataCtx, stop := context.WithTimeout(workCtx, time.Second)
			if refreshed, err := b.Metadata.Refresh(metadataCtx, uid, account); err == nil {
				account = refreshed
			} else if metadataGateError(err) {
				stop()
				return Response{}, fmt.Errorf("metadata access gate: %w", err)
			} else if errors.Is(err, ErrPublicChannelMissing) {
				// A failed durable clear must still stop displaying proven absent data.
				account.DisplayName = ""
				account.Handle, account.AvatarURL = nil, nil
				account.MetadataFetchedAt = time.Time{}
			}
			stop()
		}
		snapshot, err := b.Reader.Read(workCtx, uid)
		if err != nil {
			return b.failedSnapshot(account)
		}
		response, err := Aggregate(snapshot, accountSection(account, b.Now().UTC()))
		if err != nil {
			return b.failedSnapshot(account)
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
		b.cache[key] = cachedAggregate{response: response, accountVersion: metadataAccountVersion(account)}
		b.mu.Unlock()
		return response, nil
	})
	select {
	case <-waitCtx.Done():
		// A later caller can have less budget than the shared flight's first
		// caller. Deliver its already known terminal account before its deadline,
		// while the bounded shared operation continues for other callers.
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return b.failedSnapshot(account)
		}
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

func minDeadline(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// Preserve a known terminal metadata state even when work cannot be read. No
// work snapshot is fabricated: every work field is unavailable, never zero or
// unregistered, and this failure response is not cached. The existing wire
// contract lets browsers drop the account while retaining prior work as stale.
func (b *BFF) failedSnapshot(account WebAccount) (Response, error) {
	now := b.Now().UTC()
	section := accountSection(account, now)
	if section.ReasonCode == nil || *section.ReasonCode != MetadataTooOld {
		return Response{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	windows := StatisticsWindows(now)
	days := make([]Day, 0, 7)
	for i := range 7 {
		days = append(days, Day{Date: windows.Recent.AddDate(0, 0, i).Format("2006-01-02"), Metric: missingMetric(SourceUnavailable)})
	}
	return Response{
		GeneratedAt: now, Timezone: "Asia/Tokyo", Partial: true,
		Current: missingSection[Current](SourceUnavailable), Summary: missingSection[Summary](SourceUnavailable),
		Recent7Days: RecentSection{Availability: Unavailable, ReasonCode: ptr(SourceUnavailable), Data: days}, Account: section,
	}, nil
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

func metadataAccountVersion(account WebAccount) string {
	present := "empty"
	if account.DisplayName != "" && !account.MetadataFetchedAt.IsZero() {
		present = "present"
	}
	return account.Revision.UTC().Format(time.RFC3339Nano) + ":" + present
}
