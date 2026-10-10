package mypage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"app.modules/core/serviceaccess"
)

type (
	metadataFlight struct {
		done     chan struct{}
		response Response
		err      error
		terminal bool
	}
	cacheKey        struct{ environment, uid string }
	cachedAggregate struct {
		response       Response
		accountVersion string
	}
	MetadataRefresher interface {
		Refresh(context.Context, string, WebAccount) (WebAccount, error)
	}
)

// BFF is process-memory only. Fresh access reads fence every cache lookup,
// detached flight, publication and response; inactive results are never cached.
type BFF struct {
	Access      serviceaccess.Reader
	Reader      SnapshotReader
	Environment string
	Now         func() time.Time
	Metadata    MetadataRefresher
	mu          sync.Mutex
	cache       map[cacheKey]cachedAggregate
	flights     map[string]*metadataFlight
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

func (b *BFF) Get(ctx context.Context, uid string, account WebAccount) (result Response, resultErr error) {
	checkpoint, err := readAccess(ctx, b.Access, uid, account.AccessCheckpoint)
	if err != nil {
		b.Invalidate(uid)
		return Response{}, err
	}
	account.AccessCheckpoint = checkpoint
	defer func() {
		if resultErr == nil {
			if _, err := readAccess(ctx, b.Access, uid, checkpoint); err != nil {
				b.Invalidate(uid)
				result, resultErr = Response{}, err
			}
		}
	}()
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
	flightKey := b.Environment + "\x00" + uid + "\x00" + metadataAccountVersion(account)
	flight, leader := b.acquireFlight(flightKey)
	if flight == nil {
		return b.failedSnapshot(account)
	}
	if leader {
		go func() {
			response := Response{}
			flightErr := apiError("TEMPORARY_UNAVAILABLE")
			defer func() {
				b.mu.Lock()
				flight.response, flight.err = response, flightErr
				delete(b.flights, flightKey)
				close(flight.done)
				b.mu.Unlock()
			}()
			response, flightErr = func() (result Response, resultErr error) {
				// Refresh only the flight's copy; a caller's early fallback must not race
				// with metadata updates in the detached operation.
				account := account
				workCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), workDeadline)
				defer cancel()
				if _, err := readAccess(workCtx, b.Access, uid, checkpoint); err != nil {
					return Response{}, err
				}
				defer func() {
					if resultErr == nil {
						if _, err := readAccess(workCtx, b.Access, uid, checkpoint); err != nil {
							result, resultErr = Response{}, err
						}
					}
				}()
				if value, ok := b.cacheHit(key, b.Now().UTC(), account); ok {
					return value, nil
				}
				if b.Metadata != nil && (account.MetadataFetchedAt.IsZero() || b.Now().Sub(account.MetadataFetchedAt) >= 24*time.Hour) {
					metadataCtx, stop := context.WithTimeout(workCtx, time.Second)
					if refreshed, err := b.Metadata.Refresh(metadataCtx, uid, account); err == nil {
						account = refreshed
					} else if metadataGateError(err) {
						stop()
						gateErr := fmt.Errorf("metadata access gate: %w", err)
						b.mu.Lock()
						flight.err = gateErr
						b.mu.Unlock()
						return Response{}, gateErr
					} else if errors.Is(err, ErrPublicChannelMissing) {
						// A failed durable clear must still stop displaying proven absent data.
						account.DisplayName = ""
						account.Handle, account.AvatarURL = nil, nil
						account.MetadataFetchedAt = time.Time{}
					}
					stop()
				}
				section := accountSection(account, b.Now().UTC())
				b.mu.Lock()
				flight.terminal = section.ReasonCode != nil && *section.ReasonCode == MetadataTooOld
				b.mu.Unlock()

				snapshot, err := b.Reader.Read(workCtx, uid)
				if err != nil {
					return b.failedSnapshot(account)
				}
				response, err := Aggregate(snapshot, accountSection(account, b.Now().UTC()))
				if err != nil {
					return b.failedSnapshot(account)
				}
				if _, err := readAccess(workCtx, b.Access, uid, checkpoint); err != nil {
					return Response{}, err
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
			}()
		}()
	}
	select {
	case <-waitCtx.Done():
		// A later caller can have less budget than the shared flight's first
		// caller. Deliver its already known terminal account before its deadline,
		// while the bounded shared operation continues for other callers.
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			b.mu.Lock()
			gateErr, terminal := flight.err, flight.terminal
			b.mu.Unlock()
			if metadataGateError(gateErr) {
				return Response{}, gateErr
			}
			if terminal {
				return b.failedSnapshot(WebAccount{})
			}
			return b.failedSnapshot(account)
		}
		return Response{}, apiError("TEMPORARY_UNAVAILABLE")
	case <-flight.done:
		if flight.err != nil {
			if metadataGateError(flight.err) {
				return Response{}, flight.err
			}
			return Response{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		return flight.response, nil
	}
}

// Registration and completion share one lock so every joiner sees the same
// authenticated metadata verdict. This is a bounded singleflight registry;
// callback completion publishes the result and removes the slot atomically.
func (b *BFF) acquireFlight(key string) (*metadataFlight, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if flight := b.flights[key]; flight != nil {
		return flight, false
	}
	if len(b.flights) >= 1024 {
		return nil, false
	}
	if b.flights == nil {
		b.flights = make(map[string]*metadataFlight)
	}
	flight := &metadataFlight{done: make(chan struct{})}
	b.flights[key] = flight
	return flight, true
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
	var gate *accessGateError
	if errors.As(err, &gate) {
		return true
	}
	switch errorCode(err) {
	case "AUTH_REQUIRED", "WEB_ACCOUNT_REQUIRED", "PRIVACY_RECONSENT_REQUIRED", "SERVICE_ACCESS_RESTRICTED", "DATA_DELETION_IN_PROGRESS":
		return true
	}
	return false
}

func metadataAccountVersion(account WebAccount) string {
	present := "empty"
	if account.DisplayName != "" && !account.MetadataFetchedAt.IsZero() {
		present = "present"
	}
	return account.AccessCheckpoint + ":" + account.Revision.UTC().Format(time.RFC3339Nano) + ":" + present
}
