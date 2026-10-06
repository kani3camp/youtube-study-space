package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
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
			response, err := b.Get(context.Background(), "synthetic", account)
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
	refresh := AccountMetadataRefresh{Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now }, Provider: metadataReaderFunc(func(context.Context, string) (Channel, error) {
		reads++
		return Channel{}, apiError("TEMPORARY_UNAVAILABLE")
	}), Store: metadataWriterFunc(func(context.Context, string, WebAccount, Channel, Policy, time.Time) (WebAccount, error) {
		writes++
		return WebAccount{}, nil
	})}
	account := healthyAccount(now)
	account.Revision = now
	blocked := account
	blocked.AccessBlocked = true
	_, err := refresh.Refresh(context.Background(), "UCsynthetic0000000000001", blocked)
	require.Equal(t, "AUTH_REQUIRED", errorCode(err))
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
		response, err := b.Get(context.Background(), "synthetic", account)
		require.NoError(t, err)
		require.Nil(t, response.Account.Data)
		require.Equal(t, MetadataTooOld, *response.Account.ReasonCode)
	}
	// A transient lookup error retains permitted metadata, not a terminal state.
	b := BFF{Environment: "demo", Now: func() time.Time { return now }, Metadata: metadataFunc(func(context.Context, string, WebAccount) (WebAccount, error) {
		return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
	}), Reader: readerFunc(func(context.Context, string) (WorkSnapshot, error) { s := workFixture(); s.AsOf = now; return s, nil })}
	response, err := b.Get(context.Background(), "synthetic", account)
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
	go func() { _, err := b.Get(context.Background(), "synthetic", old); done <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	next, err := b.Get(ctx, "synthetic", cleared)
	close(release)
	require.NoError(t, err)
	require.Nil(t, next.Account.Data)
	require.NoError(t, <-done)
	// The old caller may finish last; its cached revision is still rejected.
	next, err = b.Get(context.Background(), "synthetic", cleared)
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
	go func() { _, err := b.Get(firstCtx, "synthetic", account); firstDone <- err }()
	<-entered
	stopFirst()
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(<-firstDone))
	laterCtx, stopLater := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer stopLater()
	result, err := b.Get(laterCtx, "synthetic", account)
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
	go func() { _, err := b.Get(firstCtx, "synthetic", account); firstDone <- err }()
	<-entered
	stopFirst()
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(<-firstDone))
	laterCtx, stopLater := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer stopLater()
	result, err := b.Get(laterCtx, "synthetic", account)
	close(release)
	<-finished
	require.NoError(t, err)
	require.NoError(t, laterCtx.Err())
	require.Equal(t, MetadataTooOld, *result.Account.ReasonCode)
	require.Nil(t, result.Account.Data)
	require.Nil(t, result.Current.Data)
	require.EqualValues(t, 1, calls.Load())
}
