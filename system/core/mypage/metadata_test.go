package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublicMetadataUsesApplicationKeyAndExactIDWithoutOAuthOrRawErrors(t *testing.T) {
	calls := 0
	p, err := NewPublicYouTubeMetadata("synthetic-api-key", &http.Client{Transport: providerTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Empty(t, r.Header.Get("Authorization"))
		require.Equal(t, "synthetic-api-key", r.URL.Query().Get("key"))
		require.Equal(t, "UCsynthetic0000000000001", r.URL.Query().Get("id"))
		require.Empty(t, r.URL.Query().Get("mine"))
		return providerResponse(200, syntheticChannel), nil
	})})
	require.NoError(t, err)
	channel, err := p.Read(context.Background(), "UCsynthetic0000000000001")
	require.NoError(t, err)
	require.Equal(t, "Synthetic Channel", channel.DisplayName)
	require.Equal(t, 1, calls)
	_, err = p.Read(context.Background(), "../../synthetic")
	require.Error(t, err)
	require.Equal(t, 1, calls)
	p.client.Transport = providerTransport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("https://synthetic-private?key=synthetic-api-key")
	})
	_, err = p.Read(context.Background(), "UCsynthetic0000000000001")
	require.Equal(t, "TEMPORARY_UNAVAILABLE", err.Error())
}

func TestPublicMetadataRejectsOtherChannelAndRedirect(t *testing.T) {
	for _, body := range []string{`{"items":[]}`, `{"nextPageToken":"synthetic-more","items":[]}`, `{"items":[{"id":"UCsynthetic0000000000002","snippet":{"title":"Other"}}]}`, "invalid"} {
		p, err := NewPublicYouTubeMetadata("synthetic-api-key", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) { return providerResponse(200, body), nil })})
		require.NoError(t, err)
		_, err = p.Read(context.Background(), "UCsynthetic0000000000001")
		require.Error(t, err)
	}
	calls := 0
	p, err := NewPublicYouTubeMetadata("synthetic-api-key", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) {
		calls++
		r := providerResponse(302, "")
		r.Header.Set("Location", "https://example.invalid/steal")
		return r, nil
	})})
	require.NoError(t, err)
	_, err = p.Read(context.Background(), "UCsynthetic0000000000001")
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

type metadataReaderFunc func(context.Context, string) (Channel, error)

func (f metadataReaderFunc) Read(ctx context.Context, uid string) (Channel, error) {
	return f(ctx, uid)
}

func TestBFFMetadataGateChangesAbortBeforeSnapshotAndAreNeverCached(t *testing.T) {
	now := time.Now().UTC()
	account := healthyAccount(now.Add(-25 * time.Hour))
	for _, code := range []string{"AUTH_REQUIRED", "WEB_ACCOUNT_REQUIRED", "PRIVACY_RECONSENT_REQUIRED"} {
		reads := 0
		b := BFF{Environment: "development", Now: func() time.Time { return now }, Metadata: metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) { return WebAccount{}, apiError(code) }), Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { reads++; return workFixture(), nil })}
		for i := 0; i < 2; i++ {
			response, err := b.get(context.Background(), testAuthorizedWebAccount("synthetic", account))
			require.Equal(t, code, errorCode(err))
			require.True(t, response.GeneratedAt.IsZero())
		}
		require.Zero(t, reads)
	}
}

type metadataFunc func(context.Context, string, WebAccount) (WebAccount, error)

func (f metadataFunc) Refresh(ctx context.Context, uid string, account WebAccount) (WebAccount, error) {
	return f(ctx, uid, account)
}

type metadataWriterFunc func(context.Context, string, WebAccount, Channel, Policy, time.Time) (WebAccount, error)

func (f metadataWriterFunc) SaveMetadata(ctx context.Context, uid string, a WebAccount, c Channel, p Policy, now time.Time) (WebAccount, error) {
	return f(ctx, uid, a, c, p, now)
}

func TestMetadataRefreshChecksAccountBeforeProviderAndDoesNotWriteFailedLookup(t *testing.T) {
	now := time.Now().UTC()
	reads, writes := 0, 0
	refresh := AccountMetadataRefresh{Access: allowedAccess(), Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now }, Provider: metadataReaderFunc(func(context.Context, string) (Channel, error) {
		reads++
		return Channel{}, apiError("TEMPORARY_UNAVAILABLE")
	}), Store: metadataWriterFunc(func(context.Context, string, WebAccount, Channel, Policy, time.Time) (WebAccount, error) {
		writes++
		return WebAccount{}, nil
	})}
	account := healthyAccount(now)
	account.Revision = now
	refresh.Access = restrictedAccess(now, false)
	_, err := refresh.Refresh(context.Background(), "UCsynthetic0000000000001", account)
	require.Equal(t, "SERVICE_ACCESS_RESTRICTED", errorCode(err))
	refresh.Access = allowedAccess()
	require.Zero(t, reads)
	outdated := account
	outdated.TermsVersion = "old"
	_, err = refresh.Refresh(context.Background(), "UCsynthetic0000000000001", outdated)
	require.Equal(t, "PRIVACY_RECONSENT_REQUIRED", errorCode(err))
	require.Zero(t, reads)
	_, err = refresh.Refresh(context.Background(), "UCsynthetic0000000000001", account)
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(err))
	require.Equal(t, 1, reads)
	require.Zero(t, writes)
}

func TestPublicMetadataOnlySuccessfulExplicitEmptyItemsIsTerminalAbsence(t *testing.T) {
	for _, test := range []struct {
		status  int
		body    string
		missing bool
	}{{200, `{"items":[]}`, true}, {503, `{"items":[]}`, false}, {200, `{}`, false}, {200, `{"items":null}`, false}, {200, `invalid`, false}, {200, `{"nextPageToken":"synthetic","items":[]}`, false}, {200, `{"items":[{"id":"UCsynthetic0000000000002","snippet":{"title":"Other"}}]}`, false}} {
		p, err := NewPublicYouTubeMetadata("synthetic-key", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) { return providerResponse(test.status, test.body), nil })})
		require.NoError(t, err)
		_, err = p.Read(context.Background(), "UCsynthetic0000000000001")
		require.Error(t, err)
		require.Equal(t, test.missing, errors.Is(err, ErrPublicChannelMissing))
	}
}

func TestBFFConfirmedMissingNeverRetainsAccountEvenWhenDurableClearFails(t *testing.T) {
	now := time.Now().UTC()
	account := healthyAccount(now.Add(-25 * time.Hour))
	for _, err := range []error{ErrPublicChannelMissing, errors.Join(ErrPublicChannelMissing, errors.New("synthetic store failure"))} {
		b := BFF{Environment: "demo", Now: func() time.Time { return now }, Metadata: metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) { return WebAccount{}, err }), Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { s := workFixture(); s.AsOf = now; return s, nil })}
		response, err := b.get(context.Background(), testAuthorizedWebAccount("synthetic", account))
		require.NoError(t, err)
		require.Nil(t, response.Account.Data)
		require.Equal(t, MetadataTooOld, *response.Account.ReasonCode)
	}
	// 一時的な検索エラーでは許可されたメタデータを保持し、表示停止状態にはしない。
	b := BFF{Environment: "demo", Now: func() time.Time { return now }, Metadata: metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) {
		return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
	}), Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { s := workFixture(); s.AsOf = now; return s, nil })}
	response, err := b.get(context.Background(), testAuthorizedWebAccount("synthetic", account))
	require.NoError(t, err)
	require.NotNil(t, response.Account.Data)
	require.Equal(t, MetadataRefreshFailed, *response.Account.ReasonCode)
}

func TestBFFNewRevisionDoesNotJoinOldFlightOrReuseLateOldCache(t *testing.T) {
	now := time.Now().UTC()
	old := healthyAccount(now)
	old.Revision = now.Add(-time.Second)
	cleared := WebAccount{Revision: now}
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	b := BFF{Environment: "demo", Now: func() time.Time { return now }, Reader: readerFunc(func(ctx context.Context, _ string) (WorkSnapshot, error) {
		if calls.Add(1) == 1 {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return WorkSnapshot{}, ctx.Err()
			}
		}
		s := workFixture()
		s.AsOf = now
		return s, nil
	})}
	done := make(chan error, 1)
	go func() { _, err := b.get(context.Background(), testAuthorizedWebAccount("synthetic", old)); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	next, err := b.get(ctx, testAuthorizedWebAccount("synthetic", cleared))
	close(release)
	require.NoError(t, err)
	require.Nil(t, next.Account.Data)
	require.NoError(t, <-done)
	// 旧呼び出し元が最後に完了しても、キャッシュした旧リビジョンは拒否する。
	next, err = b.get(context.Background(), testAuthorizedWebAccount("synthetic", cleared))
	require.NoError(t, err)
	require.Nil(t, next.Account.Data)
	require.Equal(t, MetadataTooOld, *next.Account.ReasonCode)
}

func TestTerminalMetadataSurvivesSnapshotFailuresThroughHTTPWithoutCachingFailure(t *testing.T) {
	expected, err := os.ReadFile("../../../docs/mypage/fixtures/terminal-account-source-unavailable.json")
	require.NoError(t, err)
	for _, failure := range []string{"read", "aggregate"} {
		for _, state := range []string{"confirmed-missing", "already-cleared", "ordinary-failure"} {
			t.Run(failure+"/"+state, func(t *testing.T) {
				h, store, _, _ := boundaryFixture()
				now := workFixture().AsOf
				fail := true
				reads := 0
				h.BFF.Reader = readerFunc(func(context.Context, string) (WorkSnapshot, error) {
					reads++
					if !fail {
						return workFixture(), nil
					}
					if failure == "read" {
						return WorkSnapshot{}, errors.New("synthetic private dependency detail")
					}
					return WorkSnapshot{}, nil
				})
				store.account.MetadataFetchedAt = now.Add(-25 * time.Hour)
				if state == "already-cleared" {
					store.account.DisplayName = ""
					store.account.MetadataFetchedAt = time.Time{}
				} else {
					h.BFF.Metadata = metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) {
						if state == "confirmed-missing" {
							return WebAccount{}, ErrPublicChannelMissing
						}
						return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
					})
				}
				for range 2 {
					response := boundaryRequest(h, "GET", "/api/mypage", "")
					if state == "ordinary-failure" {
						require.Equal(t, 503, response.Code)
						require.Equal(t, "TEMPORARY_UNAVAILABLE", responseCode(t, response))
					} else {
						require.Equal(t, 200, response.Code)
						require.JSONEq(t, string(expected), response.Body.String())
					}
					require.NotContains(t, response.Body.String(), "private")
				}
				require.Equal(t, 2, reads, "failed snapshots must not enter aggregate cache")
				fail = false
				response := boundaryRequest(h, "GET", "/api/mypage", "")
				require.Equal(t, 200, response.Code)
				require.Contains(t, response.Body.String(), `"workSec":6000`)
				require.Equal(t, 3, reads)
			})
		}
	}
}

func TestTerminalAccountSnapshotDeadlineLeavesTimeForHTTPResponse(t *testing.T) {
	for _, state := range []string{"already-cleared", "confirmed-missing", "ordinary-failure"} {
		t.Run(state, func(t *testing.T) {
			h, store, _, _ := boundaryFixture()
			now := workFixture().AsOf
			store.account.MetadataFetchedAt = now.Add(-25 * time.Hour)
			if state == "already-cleared" {
				store.account.DisplayName = ""
				store.account.MetadataFetchedAt = time.Time{}
			}
			if state == "confirmed-missing" {
				h.BFF.Metadata = metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) {
					return WebAccount{}, ErrPublicChannelMissing
				})
			}
			var innerDeadline time.Time
			h.BFF.Reader = readerFunc(func(ctx context.Context, _ string) (WorkSnapshot, error) {
				innerDeadline, _ = ctx.Deadline()
				<-ctx.Done()
				return WorkSnapshot{}, ctx.Err()
			})
			ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
			defer cancel()
			outerDeadline, _ := ctx.Deadline()
			request := httptest.NewRequest("GET", "/api/mypage", nil).WithContext(ctx)
			request.Host = "mypage.example.test"
			request.Header.Set("Authorization", "Bearer synthetic-id-proof")
			request.Header.Set("X-Firebase-AppCheck", "synthetic-app-proof")
			response := httptest.NewRecorder()
			h.ServeHTTP(response, request)
			require.NoError(t, ctx.Err(), "fallback must arrive before the outer/client deadline")
			require.True(t, innerDeadline.Before(outerDeadline))
			if state == "ordinary-failure" {
				require.Equal(t, 503, response.Code)
			} else {
				require.Equal(t, 200, response.Code)
				var result Response
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
				require.Equal(t, MetadataTooOld, *result.Account.ReasonCode)
				require.Nil(t, result.Account.Data)
				require.Nil(t, result.Summary.Data)
				require.Nil(t, result.Current.Data)
				require.Len(t, result.Recent7Days.Data, 7)
			}
		})
	}
}

func TestTerminalLaterCallerDoesNotWaitForLongerSharedFlightDeadline(t *testing.T) {
	now := workFixture().AsOf
	account := WebAccount{Revision: now}
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	b := BFF{Environment: "demo", Now: func() time.Time { return now }, Reader: readerFunc(func(ctx context.Context, _ string) (WorkSnapshot, error) {
		calls.Add(1)
		close(entered)
		defer close(finished)
		select {
		case <-release:
			return workFixture(), nil
		case <-ctx.Done():
			return WorkSnapshot{}, ctx.Err()
		}
	})}
	firstCtx, stopFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() { _, err := b.get(firstCtx, testAuthorizedWebAccount("synthetic", account)); firstDone <- err }()
	<-entered
	stopFirst()
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(<-firstDone))
	laterCtx, stopLater := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer stopLater()
	result, err := b.get(laterCtx, testAuthorizedWebAccount("synthetic", account))
	close(release)
	<-finished
	require.NoError(t, err)
	require.NoError(t, laterCtx.Err())
	require.Equal(t, MetadataTooOld, *result.Account.ReasonCode)
	require.Nil(t, result.Account.Data)
	require.Nil(t, result.Current.Data)
	require.EqualValues(t, 1, calls.Load(), "shorter caller must not cancel or duplicate the shared read")
}

func TestConfirmedMissingFlightVerdictReachesShorterPresentCaller(t *testing.T) {
	now := workFixture().AsOf
	account := healthyAccount(now.Add(-25 * time.Hour))
	account.Revision = now.Add(-time.Second)
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	b := BFF{Environment: "demo", Now: func() time.Time { return now }, Metadata: metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) {
		return WebAccount{}, errors.Join(ErrPublicChannelMissing, errors.New("synthetic failed clear"))
	}), Reader: readerFunc(func(ctx context.Context, _ string) (WorkSnapshot, error) {
		calls.Add(1)
		close(entered)
		defer close(finished)
		select {
		case <-release:
			return workFixture(), nil
		case <-ctx.Done():
			return WorkSnapshot{}, ctx.Err()
		}
	})}
	firstCtx, stopFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() { _, err := b.get(firstCtx, testAuthorizedWebAccount("synthetic", account)); firstDone <- err }()
	<-entered
	stopFirst()
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(<-firstDone))
	laterCtx, stopLater := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer stopLater()
	result, err := b.get(laterCtx, testAuthorizedWebAccount("synthetic", account))
	close(release)
	<-finished
	require.NoError(t, err)
	require.NoError(t, laterCtx.Err())
	require.Equal(t, MetadataTooOld, *result.Account.ReasonCode)
	require.Nil(t, result.Account.Data)
	require.Nil(t, result.Current.Data)
	require.EqualValues(t, 1, calls.Load())
}

// simulatedLatencyは実時間を待たずに、依存先が消費する時間を残りの期限から差し引く。
type simulatedLatency struct {
	mu    sync.Mutex
	spent time.Duration
}

func (l *simulatedLatency) spend(ctx context.Context, d time.Duration) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		return errors.New("metadata refresh must be bounded")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Until(deadline) < l.spent+d {
		return context.DeadlineExceeded
	}
	l.spent += d
	return nil
}

func TestMetadataRefreshAnsweredWithinProviderTimeoutIsPersistedAndServed(t *testing.T) {
	h, store, _, _ := boundaryFixture()
	now := workFixture().AsOf
	store.account.MetadataFetchedAt = now.Add(-25 * time.Hour)
	store.account.Revision = now.Add(-time.Hour)
	latency := &simulatedLatency{}
	var lookups, saves atomic.Int32
	h.BFF.Metadata = &AccountMetadataRefresh{
		Access: allowedAccess(), Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now },
		Provider: metadataReaderFunc(func(ctx context.Context, uid string) (Channel, error) {
			lookups.Add(1)
			// 旧来の1秒全体予算の大半を、YouTube自身の1秒制限内で消費する応答。
			if err := latency.spend(ctx, 900*time.Millisecond); err != nil {
				return Channel{}, apiError("TEMPORARY_UNAVAILABLE")
			}
			return Channel{ID: uid, DisplayName: "Refreshed"}, nil
		}),
		Store: metadataWriterFunc(func(ctx context.Context, _ string, expected WebAccount, channel Channel, _ Policy, at time.Time) (WebAccount, error) {
			// 保存トランザクション、保存後の再読み取り、アクセス再確認。
			if err := latency.spend(ctx, 300*time.Millisecond); err != nil {
				return WebAccount{}, err
			}
			saves.Add(1)
			persisted := store.account
			persisted.DisplayName, persisted.MetadataFetchedAt, persisted.Revision = channel.DisplayName, at, at
			store.account = persisted
			persisted.AccessCheckpoint = expected.AccessCheckpoint
			return persisted, nil
		}),
	}
	for range 2 {
		response := boundaryRequest(h, "GET", "/api/mypage", "")
		require.Equal(t, 200, response.Code)
		var result Response
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.Equal(t, Available, result.Account.Availability)
		require.Equal(t, "Refreshed", result.Account.Data.DisplayName)
	}
	require.EqualValues(t, 1, lookups.Load(), "the persisted metadata must satisfy the next request")
	require.EqualValues(t, 1, saves.Load())
	require.Equal(t, "Refreshed", store.account.DisplayName)
	require.True(t, store.account.MetadataFetchedAt.Equal(now))
}

func TestSlowMetadataRefreshNeitherDelaysResponseNorStarvesSnapshot(t *testing.T) {
	now := workFixture().AsOf
	account := healthyAccount(now.Add(-25 * time.Hour))
	account.Revision = now.Add(-time.Hour)
	var refreshDeadline time.Time
	var snapshotErr error
	b := BFF{
		Environment: "demo", Now: func() time.Time { return now },
		Metadata: metadataFunc(func(ctx context.Context, _ string, _ WebAccount) (WebAccount, error) {
			refreshDeadline, _ = ctx.Deadline()
			<-ctx.Done()
			return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
		}),
		Reader: readerFunc(func(ctx context.Context, _ string) (WorkSnapshot, error) {
			snapshotErr = ctx.Err()
			return workFixture(), nil
		}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	outerDeadline, _ := ctx.Deadline()
	response, err := b.get(ctx, testAuthorizedWebAccount("synthetic", account))
	require.NoError(t, err)
	require.NoError(t, ctx.Err(), "response must arrive before the caller deadline")
	require.True(t, refreshDeadline.Before(outerDeadline))
	require.NoError(t, snapshotErr, "a slow refresh must leave time for the work snapshot")
	require.Equal(t, Partial, response.Account.Availability)
	require.Equal(t, MetadataRefreshFailed, *response.Account.ReasonCode)
	require.Equal(t, "Sample", response.Account.Data.DisplayName)
}

func TestFailedMetadataRefreshBacksOffPerAccountState(t *testing.T) {
	start := workFixture().AsOf
	now := start
	account := healthyAccount(now.Add(-25 * time.Hour))
	account.Revision = now.Add(-time.Hour)
	refreshes, reads := 0, 0
	b := BFF{
		Environment: "demo", Now: func() time.Time { return now },
		Metadata: metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) {
			refreshes++
			return WebAccount{}, fmt.Errorf("refresh public metadata: %w", apiError("TEMPORARY_UNAVAILABLE"))
		}),
		Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) {
			reads++
			s := workFixture()
			s.AsOf = now
			return s, nil
		}),
	}
	get := func(at time.Duration, account WebAccount) {
		t.Helper()
		now = start.Add(at)
		response, err := b.get(context.Background(), testAuthorizedWebAccount("synthetic", account))
		require.NoError(t, err)
		require.Equal(t, MetadataRefreshFailed, *response.Account.ReasonCode)
		require.Equal(t, "Sample", response.Account.Data.DisplayName)
	}
	get(0, account)
	require.Equal(t, 1, refreshes)
	get(31*time.Second, account)
	get(metadataRefreshBackoff-31*time.Second, account)
	require.Equal(t, 3, reads, "uncached requests still read the work snapshot")
	require.Equal(t, 1, refreshes, "a failed refresh must not be retried before the backoff elapses")
	get(metadataRefreshBackoff, account)
	require.Equal(t, 2, refreshes)
	changed := account
	changed.Revision = account.Revision.Add(time.Second)
	get(metadataRefreshBackoff+31*time.Second, changed)
	require.Equal(t, 3, refreshes, "a changed account state must be refreshed immediately")
}

func TestMetadataGateAndConfirmedMissingAreRecheckedOnEveryUncachedRequest(t *testing.T) {
	start := workFixture().AsOf
	for _, test := range []struct {
		name string
		err  error
	}{
		{"restricted", &accessGateError{cause: apiError("SERVICE_ACCESS_RESTRICTED")}},
		{"gate-unavailable", fmt.Errorf("save public metadata: %w", &accessGateError{cause: apiError("TEMPORARY_UNAVAILABLE")})},
		{"reconsent", apiError("PRIVACY_RECONSENT_REQUIRED")},
		{"confirmed-missing", errors.Join(ErrPublicChannelMissing, errors.New("synthetic failed clear"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := start
			account := healthyAccount(now.Add(-25 * time.Hour))
			account.Revision = now.Add(-time.Hour)
			refreshes, reads := 0, 0
			b := BFF{
				Environment: "demo", Now: func() time.Time { return now },
				Metadata: metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) {
					refreshes++
					return WebAccount{}, test.err
				}),
				Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) {
					reads++
					s := workFixture()
					s.AsOf = now
					return s, nil
				}),
			}
			for i := range 2 {
				now = start.Add(time.Duration(i) * 31 * time.Second)
				response, err := b.get(context.Background(), testAuthorizedWebAccount("synthetic", account))
				if test.name == "confirmed-missing" {
					require.NoError(t, err)
					require.Nil(t, response.Account.Data)
					require.Equal(t, MetadataTooOld, *response.Account.ReasonCode)
					continue
				}
				require.True(t, metadataGateError(err), "error=%v", err)
				require.True(t, response.GeneratedAt.IsZero())
			}
			require.Equal(t, 2, refreshes, "gate and absence verdicts must never be skipped by the failure backoff")
			if test.name != "confirmed-missing" {
				require.Zero(t, reads)
			}
		})
	}
}

func TestConcurrentStaleMetadataRequestsShareOneRefresh(t *testing.T) {
	now := workFixture().AsOf
	account := healthyAccount(now.Add(-25 * time.Hour))
	account.Revision = now.Add(-time.Hour)
	entered, release := make(chan struct{}), make(chan struct{})
	var refreshes atomic.Int32
	b := BFF{
		Environment: "demo", Now: func() time.Time { return now },
		Metadata: metadataFunc(func(ctx context.Context, _ string, _ WebAccount) (WebAccount, error) {
			if refreshes.Add(1) == 1 {
				close(entered)
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
		}),
		Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { return workFixture(), nil }),
	}
	const callers = 8
	results := make(chan error, callers)
	for range callers {
		go func() {
			response, err := b.get(context.Background(), testAuthorizedWebAccount("synthetic", account))
			if err == nil && (response.Account.ReasonCode == nil || *response.Account.ReasonCode != MetadataRefreshFailed) {
				err = errors.New("stale metadata was not reported as refresh failed")
			}
			results <- err
		}()
	}
	<-entered
	close(release)
	for range callers {
		require.NoError(t, <-results)
	}
	require.EqualValues(t, 1, refreshes.Load())
}
