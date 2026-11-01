package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 実アカウント・認証情報を使わず、流出したら判別できる合成値だけを使う。
const diagnosticSentinel = "DO_NOT_LOG code-sentinel state-sentinel cookie-sentinel access-token-sentinel refresh-token-sentinel client-secret-sentinel person@example.invalid UCsynthetic0000000000001 oauth-transactions/document-sentinel https://example.invalid/?code=code-sentinel\ninjected-log-line"

type diagnosticStore struct {
	AuthStore
	purpose                               string
	readErr, claimErr, failErr, verifyErr error
	calls                                 []string
}

func (s *diagnosticStore) ReadPurpose(context.Context, string) (string, error) {
	s.calls = append(s.calls, "read")
	return s.purpose, s.readErr
}
func (s *diagnosticStore) Claim(context.Context, string, string, time.Time) error {
	s.calls = append(s.calls, "claim")
	return s.claimErr
}
func (s *diagnosticStore) Fail(context.Context, string) error {
	s.calls = append(s.calls, "fail")
	return s.failErr
}
func (s *diagnosticStore) Verify(context.Context, string, Channel, string, time.Time) error {
	s.calls = append(s.calls, "verify")
	return s.verifyErr
}

type diagnosticProvider struct {
	channels []Channel
	err      error
	calls    int
}

func (*diagnosticProvider) AuthorizationURL(string) string { return "https://example.invalid" }
func (p *diagnosticProvider) Resolve(context.Context, string) ([]Channel, error) {
	p.calls++
	return p.channels, p.err
}

type diagnosticMaintenance struct {
	MaintenanceReader
	err error
}

func (m diagnosticMaintenance) Check(context.Context) error { return m.err }

func diagnosticFixture() (*AuthService, *diagnosticStore, *diagnosticProvider, chan OAuthFailureObservation) {
	store := &diagnosticStore{purpose: "login"}
	provider := &diagnosticProvider{channels: []Channel{{ID: "UCsynthetic0000000000001", DisplayName: diagnosticSentinel}}}
	queue := make(chan OAuthFailureObservation, 4)
	return &AuthService{Store: store, Provider: provider, OAuthFailures: queue, Now: time.Now}, store, provider, queue
}

func requireDiagnostic(t *testing.T, queue chan OAuthFailureObservation, stage, code string) {
	t.Helper()
	require.Len(t, queue, 1)
	observation := <-queue
	// 完全一致は任意文字列・余分な出力・改行の混入も拒否する。
	require.Equal(t, stage, observation.Stage())
	require.Equal(t, code, observation.ErrorCode())
	encoded, err := json.Marshal(observation)
	require.NoError(t, err)
	require.Equal(t, "{}", string(encoded), "internal fields must never serialize")
	require.NotContains(t, fmt.Sprint(observation), "DO_NOT_LOG")
}

func TestOAuthDiagnosticsCallbackStagesAndExistingOperations(t *testing.T) {
	private := errors.New(diagnosticSentinel)
	for _, tc := range []struct {
		name, stage, code string
		setup             func(*AuthService, *diagnosticStore, *diagnosticProvider)
		denied            bool
		calls             []string
		providerCalls     int
	}{
		{"read", "purpose_read", "INTERNAL_ERROR", func(_ *AuthService, s *diagnosticStore, _ *diagnosticProvider) { s.readErr = private }, false, []string{"read"}, 0},
		{"maintenance", "maintenance_check", "MAINTENANCE_ACTIVE", func(s *AuthService, _ *diagnosticStore, _ *diagnosticProvider) {
			s.Maintenance = diagnosticMaintenance{err: apiError("MAINTENANCE_ACTIVE")}
		}, false, []string{"read"}, 0},
		{"claim dependency", "transaction_claim", "INTERNAL_ERROR", func(_ *AuthService, s *diagnosticStore, _ *diagnosticProvider) { s.claimErr = private }, false, []string{"read", "claim"}, 0},
		{"claim state", "transaction_claim", "OAUTH_FAILED", func(_ *AuthService, s *diagnosticStore, _ *diagnosticProvider) { s.claimErr = apiError("OAUTH_FAILED") }, false, []string{"read", "claim"}, 0},
		{"claim expired", "transaction_claim", "OAUTH_TRANSACTION_EXPIRED", func(_ *AuthService, s *diagnosticStore, _ *diagnosticProvider) {
			s.claimErr = apiError("OAUTH_TRANSACTION_EXPIRED")
		}, false, []string{"read", "claim"}, 0},
		{"denied", "provider_denied", "OAUTH_FAILED", func(*AuthService, *diagnosticStore, *diagnosticProvider) {}, true, []string{"read", "claim", "fail"}, 0},
		{"denied cleanup", "transaction_fail", "INTERNAL_ERROR", func(_ *AuthService, s *diagnosticStore, _ *diagnosticProvider) { s.failErr = private }, true, []string{"read", "claim", "fail"}, 0},
		{"provider dependency", "provider_resolve", "INTERNAL_ERROR", func(_ *AuthService, _ *diagnosticStore, p *diagnosticProvider) { p.err = private }, false, []string{"read", "claim", "fail"}, 1},
		{"provider cleanup", "transaction_fail", "INTERNAL_ERROR", func(_ *AuthService, s *diagnosticStore, p *diagnosticProvider) {
			s.failErr = private
			p.err = apiError("OAUTH_FAILED")
		}, false, []string{"read", "claim", "fail"}, 1},
		{"no channels", "channel_selection", "CHANNEL_UNAVAILABLE", func(_ *AuthService, _ *diagnosticStore, p *diagnosticProvider) { p.channels = nil }, false, []string{"read", "claim", "fail"}, 1},
		{"two channels", "channel_selection", "CHANNEL_AMBIGUOUS", func(_ *AuthService, _ *diagnosticStore, p *diagnosticProvider) {
			p.channels = append(p.channels, p.channels[0])
		}, false, []string{"read", "claim", "fail"}, 1},
		{"invalid channel", "channel_selection", "CHANNEL_UNAVAILABLE", func(_ *AuthService, _ *diagnosticStore, p *diagnosticProvider) { p.channels[0].ID = "" }, false, []string{"read", "claim", "fail"}, 1},
		{"channel cleanup", "transaction_fail", "INTERNAL_ERROR", func(_ *AuthService, s *diagnosticStore, p *diagnosticProvider) { s.failErr = private; p.channels = nil }, false, []string{"read", "claim", "fail"}, 1},
		{"verify", "transaction_verify", "INTERNAL_ERROR", func(_ *AuthService, s *diagnosticStore, _ *diagnosticProvider) { s.verifyErr = private }, false, []string{"read", "claim", "verify"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, store, provider, queue := diagnosticFixture()
			tc.setup(s, store, provider)
			_, err := s.CallbackWithPurpose(context.Background(), diagnosticSentinel, diagnosticSentinel, diagnosticSentinel, tc.denied)
			require.Error(t, err)
			require.Equal(t, tc.code, errorCode(err))
			requireDiagnostic(t, queue, tc.stage, tc.code)
			require.Equal(t, tc.calls, store.calls)
			require.Equal(t, tc.providerCalls, provider.calls)
			if tc.name == "claim dependency" {
				require.ErrorIs(t, err, private, "diagnostics must preserve existing error identity")
			}
		})
	}
}

func TestOAuthDiagnosticsSuccessAndUnavailableQueueCannotChangeAuthResult(t *testing.T) {
	for _, queueState := range []string{"nil", "unbuffered", "full", "closed", "ready"} {
		t.Run(queueState, func(t *testing.T) {
			s, store, _, queue := diagnosticFixture()
			switch queueState {
			case "nil":
				s.OAuthFailures = nil
			case "unbuffered":
				s.OAuthFailures = make(chan OAuthFailureObservation)
			case "full":
				for range cap(queue) {
					queue <- OAuthFailureObservation{}
				}
			case "closed":
				close(queue)
			}
			done := make(chan error, 1)
			go func() { done <- s.Callback(context.Background(), "", diagnosticSentinel, diagnosticSentinel, false) }()
			select {
			case err := <-done:
				require.Equal(t, "OAUTH_TRANSACTION_REQUIRED", errorCode(err))
			case <-time.After(time.Second):
				t.Fatal("diagnostic queue blocked authentication")
			}
			require.Empty(t, store.calls)
			if queueState == "ready" {
				requireDiagnostic(t, queue, "callback_input", "OAUTH_TRANSACTION_REQUIRED")
			}
		})
	}
	for _, withPurpose := range []bool{false, true} {
		s, _, _, queue := diagnosticFixture()
		var err error
		if withPurpose {
			_, err = s.CallbackWithPurpose(context.Background(), diagnosticSentinel, diagnosticSentinel, diagnosticSentinel, false)
		} else {
			err = s.Callback(context.Background(), diagnosticSentinel, diagnosticSentinel, diagnosticSentinel, false)
		}
		require.NoError(t, err)
		require.Empty(t, queue, "success must not emit identity or success events")
	}
}

func TestOAuthDiagnosticAllowlistRejectsArbitraryErrorAndUnknownStage(t *testing.T) {
	s, _, _, queue := diagnosticFixture()
	s.observeOAuthFailure(oauthFailure(oauthFailureStage(255), apiError(diagnosticSentinel)))
	requireDiagnostic(t, queue, "unknown", "INTERNAL_ERROR")
	// 将来内部の呼び出し側を誤って変えても、formatterのallowlistで防ぐ。
	observation := OAuthFailureObservation{stage: oauthStageUnknown, code: diagnosticSentinel}
	require.Equal(t, "INTERNAL_ERROR", observation.ErrorCode())
}

func TestOAuthDiagnosticsProviderStagesNeverRetainUpstreamPayload(t *testing.T) {
	for _, tc := range []struct {
		name, grant, body, stage, code string
		status, transportFailureAt     int
	}{
		{"exchange body", diagnosticSentinel, "", "token_exchange", "OAUTH_FAILED", 400, 0},
		{"exchange transport", "", "", "token_exchange", "OAUTH_FAILED", 0, 1},
		{"grant parse", diagnosticSentinel, "", "token_exchange", "OAUTH_FAILED", 200, 0},
		{"scope", strings.ReplaceAll(syntheticGrant, youtubeReadOnly, "unapproved-scope-sentinel"), "", "scope_validation", "OAUTH_SCOPE_INSUFFICIENT", 200, 0},
		{"token", strings.ReplaceAll(syntheticGrant, "Bearer", "Basic"), "", "token_validation", "OAUTH_FAILED", 200, 0},
		{"channel transport", syntheticGrant, "", "channel_transport", "TEMPORARY_UNAVAILABLE", 200, 2},
		{"channel status", syntheticGrant, diagnosticSentinel, "channel_status", "CHANNEL_UNAVAILABLE", 403, 0},
		{"channel parse", syntheticGrant, diagnosticSentinel, "channel_decode", "TEMPORARY_UNAVAILABLE", 200, 0},
		{"channel absent", syntheticGrant, `{"items":[]}`, "channel_decode", "CHANNEL_UNAVAILABLE", 200, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			p, err := NewGoogleYouTubeOAuth("client-sentinel", "client-secret-sentinel", "https://example.invalid", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == tc.transportFailureAt {
					return nil, errors.New(diagnosticSentinel)
				}
				if calls == 1 {
					status := 200
					if tc.stage == "token_exchange" {
						status = tc.status
					}
					return providerResponse(status, tc.grant), nil
				}
				return providerResponse(tc.status, tc.body), nil
			})})
			require.NoError(t, err)
			s, _, _, queue := diagnosticFixture()
			s.Provider = p
			err = s.Callback(context.Background(), diagnosticSentinel, diagnosticSentinel, diagnosticSentinel, false)
			require.Error(t, err)
			require.Equal(t, "resolve OAuth channel: "+tc.code, err.Error())
			requireDiagnostic(t, queue, tc.stage, tc.code)
			require.LessOrEqual(t, calls, 2, "no diagnostic retries")
		})
	}
}

func TestOAuthDiagnosticsHTTPRedirectDoesNotIncludePrivateError(t *testing.T) {
	s, store, _, queue := diagnosticFixture()
	store.claimErr = errors.New(diagnosticSentinel)
	h := &HTTPHandler{Auth: s}
	r := httptest.NewRequest("GET", "/api/auth/youtube/callback?state=state-sentinel&code=code-sentinel", nil)
	r.AddCookie(&http.Cookie{Name: "__session", Value: strings.Repeat("a", 64)})
	w := httptest.NewRecorder()
	h.callback(w, r, "synthetic-request")
	require.Equal(t, http.StatusFound, w.Code)
	require.Equal(t, "/login?error=oauth_failed", w.Header().Get("Location"))
	require.NotContains(t, w.Body.String()+fmt.Sprint(w.Header()), "DO_NOT_LOG")
	requireDiagnostic(t, queue, "transaction_claim", "INTERNAL_ERROR")
}
