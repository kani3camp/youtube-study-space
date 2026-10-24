package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type providerTransport func(*http.Request) (*http.Response, error)

func (f providerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func providerResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

const (
	syntheticGrant   = `{"access_token":"synthetic-access","refresh_token":"unsolicited-synthetic-refresh","expires_in":3600,"token_type":"Bearer","scope":"https://www.googleapis.com/auth/youtube.readonly"}`
	syntheticChannel = `{"items":[{"id":"UCsynthetic0000000000001","snippet":{"title":"Synthetic Channel","customUrl":"@synthetic","thumbnails":{"default":{"url":"https://example.invalid/avatar"}}}}]}`
)

func TestYouTubeAuthorizationOnlyRequestsOnlineReadOnly(t *testing.T) {
	p, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", nil)
	require.NoError(t, err)
	target, err := url.Parse(p.AuthorizationURL("synthetic-state"))
	require.NoError(t, err)
	require.Equal(t, "https://accounts.google.com/o/oauth2/v2/auth", target.Scheme+"://"+target.Host+target.Path)
	require.Equal(t, url.Values{"client_id": {"synthetic-client"}, "redirect_uri": {"https://example.invalid/api/auth/youtube/callback"}, "response_type": {"code"}, "scope": {youtubeReadOnly}, "state": {"synthetic-state"}, "access_type": {"online"}, "prompt": {"select_account"}}, target.Query())
	require.NotContains(t, target.String(), "synthetic-secret")
}

func TestYouTubeResolveSingleExchangeAndNoReturnedCredentials(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: providerTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		deadline, ok := r.Context().Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 8*time.Second)
		if calls == 1 {
			require.Equal(t, "https://oauth2.googleapis.com/token", r.URL.String())
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			require.Equal(t, url.Values{"code": {"synthetic-code"}, "client_id": {"synthetic-client"}, "client_secret": {"synthetic-secret"}, "redirect_uri": {"https://example.invalid/api/auth/youtube/callback"}, "grant_type": {"authorization_code"}}, r.PostForm)
			return providerResponse(200, syntheticGrant), nil
		}
		require.Equal(t, 2, calls)
		require.Equal(t, youtubeChannelsURL, r.URL.Scheme+"://"+r.URL.Host+r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer synthetic-access", r.Header.Get("Authorization"))
		require.Equal(t, "true", r.URL.Query().Get("mine"))
		require.Equal(t, "snippet", r.URL.Query().Get("part"))
		require.Equal(t, "2", r.URL.Query().Get("maxResults"))
		require.Empty(t, r.URL.Query().Get("id"))
		require.NotContains(t, r.URL.String(), "synthetic-access")
		require.NotContains(t, r.URL.String(), "synthetic-code")
		return providerResponse(200, syntheticChannel), nil
	})}
	p, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", client)
	require.NoError(t, err)
	channels, err := p.Resolve(context.Background(), "synthetic-code")
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Equal(t, "UCsynthetic0000000000001", channels[0].ID)
	require.Equal(t, "Synthetic Channel", channels[0].DisplayName)
	require.Equal(t, "@synthetic", *channels[0].Handle)
	require.Equal(t, "https://example.invalid/avatar", *channels[0].AvatarURL)
	encoded, err := json.Marshal(channels)
	require.NoError(t, err)
	for _, secret := range []string{"synthetic-access", "synthetic-secret", "synthetic-code", "unsolicited-synthetic-refresh"} {
		require.NotContains(t, string(encoded), secret)
	}
	require.Equal(t, 2, calls)
}

func TestYouTubeGrantFailuresNeverRetryOrReadChannels(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"provider error", `{"error":"invalid_grant","error_description":"synthetic-secret synthetic-code"}`, "OAUTH_FAILED", 400},
		{"missing scope", `{"access_token":"synthetic-access","expires_in":3600,"token_type":"Bearer"}`, "OAUTH_SCOPE_INSUFFICIENT", 200},
		{"insufficient scope", strings.ReplaceAll(syntheticGrant, youtubeReadOnly, "https://www.googleapis.com/auth/youtube"), "OAUTH_SCOPE_INSUFFICIENT", 200},
		{"lookalike scope", strings.ReplaceAll(syntheticGrant, youtubeReadOnly, youtubeReadOnly+".invalid"), "OAUTH_SCOPE_INSUFFICIENT", 200},
		{"missing token", strings.ReplaceAll(syntheticGrant, "synthetic-access", ""), "OAUTH_FAILED", 200},
		{"wrong token type", strings.ReplaceAll(syntheticGrant, "Bearer", "Basic"), "OAUTH_FAILED", 200},
		{"no expiry", strings.ReplaceAll(syntheticGrant, `"expires_in":3600,`, ""), "OAUTH_FAILED", 200},
		{"expired", strings.ReplaceAll(syntheticGrant, `3600`, `-1`), "OAUTH_FAILED", 200},
		{"malformed", `invalid synthetic-secret`, "OAUTH_FAILED", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			p, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) { calls++; return providerResponse(tc.status, tc.body), nil })})
			require.NoError(t, err)
			result, err := p.Resolve(context.Background(), "synthetic-code")
			require.Nil(t, result)
			require.Equal(t, tc.code, err.Error())
			require.Equal(t, 1, calls)
		})
	}
}

func TestYouTubeUnavailableAmbiguousAndMalformedChannels(t *testing.T) {
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"none", `{"items":[]}`, "CHANNEL_UNAVAILABLE", 200},
		{"two", `{"items":[{},{}]}`, "CHANNEL_AMBIGUOUS", 200},
		{"more pages", `{"nextPageToken":"synthetic-next","items":[{}]}`, "CHANNEL_AMBIGUOUS", 200},
		{"bad id", strings.ReplaceAll(syntheticChannel, "UCsynthetic0000000000001", "../../synthetic"), "CHANNEL_UNAVAILABLE", 200},
		{"empty title", strings.ReplaceAll(syntheticChannel, "Synthetic Channel", "  "), "CHANNEL_UNAVAILABLE", 200},
		{"malformed", `invalid synthetic-access`, "TEMPORARY_UNAVAILABLE", 200},
		{"oversized", strings.Repeat(" ", 65537), "TEMPORARY_UNAVAILABLE", 200},
		{"upstream failure", `{"error":"synthetic-access synthetic-code"}`, "CHANNEL_UNAVAILABLE", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			p, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return providerResponse(200, syntheticGrant), nil
				}
				return providerResponse(tc.status, tc.body), nil
			})})
			require.NoError(t, err)
			result, err := p.Resolve(context.Background(), "synthetic-code")
			require.Nil(t, result)
			require.Equal(t, tc.code, err.Error())
			require.Equal(t, 2, calls)
		})
	}
}

func TestYouTubeNoRedirectAndNoRawNetworkError(t *testing.T) {
	calls := 0
	client := &http.Client{Timeout: time.Minute, Transport: providerTransport(func(*http.Request) (*http.Response, error) {
		calls++
		r := providerResponse(302, `synthetic-secret`)
		r.Header.Set("Location", "https://example.invalid/steal")
		return r, nil
	})}
	p, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", client)
	require.NoError(t, err)
	require.Equal(t, time.Minute, client.Timeout)
	require.Equal(t, 8*time.Second, p.client.Timeout)
	_, err = p.Resolve(context.Background(), "synthetic-code")
	require.Equal(t, "OAUTH_FAILED", err.Error())
	require.Equal(t, 1, calls)
	p.client.Transport = providerTransport(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("synthetic-access synthetic-secret synthetic-code")
	})
	_, err = p.Resolve(context.Background(), "synthetic-code")
	require.Equal(t, "OAUTH_FAILED", err.Error())
}

func TestYouTubeCanceledExchangeNeverReadsChannels(t *testing.T) {
	calls := 0
	p, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", &http.Client{Transport: providerTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.Resolve(ctx, "synthetic-code")
	require.Equal(t, "OAUTH_FAILED", err.Error())
	require.LessOrEqual(t, calls, 1)
}

func TestYouTubeUnsafeConfigurationFailsClosed(t *testing.T) {
	for _, origin := range []string{"http://example.invalid", "https://user:pass@example.invalid", "https://example.invalid/", "https://example.invalid?", "https://example.invalid?x=1", "https://example.invalid#fragment", "https:example.invalid"} {
		_, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", origin, nil)
		require.Error(t, err, origin)
	}
	for _, missing := range []struct{ id, secret string }{{"", "synthetic-secret"}, {"synthetic-client", ""}} {
		_, err := NewGoogleYouTubeOAuth(missing.id, missing.secret, "https://example.invalid", nil)
		require.Error(t, err)
	}
}

func TestYouTubeLegacyHandleAndUnsafeAvatarAreNotExposed(t *testing.T) {
	calls := 0
	body := strings.ReplaceAll(strings.ReplaceAll(syntheticChannel, "@synthetic", "legacy-custom-name"), "https://example.invalid/avatar", "http://example.invalid/avatar")
	p, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return providerResponse(200, syntheticGrant), nil
		}
		return providerResponse(200, body), nil
	})})
	require.NoError(t, err)
	channels, err := p.Resolve(context.Background(), "synthetic-code")
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Nil(t, channels[0].Handle)
	require.Nil(t, channels[0].AvatarURL)
}
