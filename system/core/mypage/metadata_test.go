package mypage

import (
	"context"
	"errors"
	"net/http"
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
