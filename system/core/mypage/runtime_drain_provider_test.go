package mypage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type runtimeProviderDeadlineReader struct{}

func (runtimeProviderDeadlineReader) Read([]byte) (int, error) { return 0, context.DeadlineExceeded }

func TestRuntimeActualGoogleProviderDistinguishesDependencyTimeoutFromDefinitiveGrantRejection(t *testing.T) {
	for _, scenario := range []string{"exchange timeout", "channel timeout", "body timeout", "invalid grant"} {
		t.Run(scenario, func(t *testing.T) {
			now := workFixture().AsOf
			r := NewRuntimeRegistry("development", "demo-mypage")
			store := &runtimeAuthStore{tx: OAuthTransaction{CreatedAt: now}}
			s := runtimeAuth(r, now, store)
			calls := 0
			provider, err := NewGoogleYouTubeOAuth("synthetic-client", "synthetic-secret", "https://example.invalid", &http.Client{Transport: providerTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				_, hasChildDeadline := request.Context().Deadline()
				require.True(t, hasChildDeadline)
				switch scenario {
				case "invalid grant":
					return providerResponse(400, `{"error":"invalid_grant","error_description":"synthetic-private-provider-detail"}`), nil
				case "channel timeout", "body timeout":
					if calls == 1 {
						return providerResponse(200, syntheticGrant), nil
					}
					if scenario == "body timeout" {
						response := providerResponse(200, "")
						response.Body = io.NopCloser(runtimeProviderDeadlineReader{})
						return response, nil
					}
				}
				// Deterministically model the transport's shorter child budget;
				// the enclosing Callback context remains live throughout.
				return nil, context.DeadlineExceeded
			})})
			require.NoError(t, err)
			s.Provider = provider
			outer := context.Background()
			err = s.Callback(outer, "synthetic", "synthetic", "synthetic-code", false)
			require.Error(t, err)
			require.NoError(t, outer.Err())
			require.EqualValues(t, 1, store.failed.Load(), "failure bookkeeping must run before retaining pending")
			require.Zero(t, store.verified.Load())
			require.NotContains(t, err.Error(), "synthetic-private-provider-detail")
			require.NotContains(t, err.Error(), "synthetic-secret")
			require.NotContains(t, err.Error(), "https://")
			if scenario == "invalid grant" {
				require.False(t, errors.Is(err, ErrRuntimeOutcomeUnknown))
				require.Equal(t, "OAUTH_FAILED", errorCode(err))
				require.Zero(t, r.Observe(runtimeChannel).Pending)
			} else {
				require.ErrorIs(t, err, ErrRuntimeOutcomeUnknown)
				require.EqualValues(t, 1, r.Observe(runtimeChannel).Pending)
				a, e := runtimeAdapter(r, now), runtimeExecution(now)
				runtimeApply(t, a, e, "pause")
				_, err := a.Apply(outer, runtimeOperation(e, "drain"))
				require.Error(t, err)
			}
			expectedCalls := 1
			if scenario == "channel timeout" || scenario == "body timeout" {
				expectedCalls = 2
			}
			require.Equal(t, expectedCalls, calls)
		})
	}
}

func TestRuntimeActualPublicMetadataDistinguishesChildTimeoutFromCompleteReadVerdicts(t *testing.T) {
	for _, scenario := range []string{"request timeout", "body timeout", "complete failure", "malformed complete body", "explicit absence"} {
		t.Run(scenario, func(t *testing.T) {
			now := workFixture().AsOf
			r := NewRuntimeRegistry("development", "demo-mypage")
			provider, err := NewPublicYouTubeMetadata("synthetic-key", &http.Client{Transport: providerTransport(func(request *http.Request) (*http.Response, error) {
				_, hasChildDeadline := request.Context().Deadline()
				require.True(t, hasChildDeadline)
				switch scenario {
				case "request timeout":
					return nil, context.DeadlineExceeded
				case "body timeout":
					response := providerResponse(200, "")
					response.Body = io.NopCloser(runtimeProviderDeadlineReader{})
					return response, nil
				case "malformed complete body":
					return providerResponse(200, "invalid synthetic-private-detail"), nil
				case "explicit absence":
					return providerResponse(200, `{"items":[]}`), nil
				default:
					return providerResponse(503, "synthetic-private-detail"), nil
				}
			})})
			require.NoError(t, err)
			refresh := &AccountMetadataRefresh{Runtime: r, Access: allowedAccess(), Provider: provider, Policy: Policy{Privacy: "p1", Terms: "t1"}, Now: func() time.Time { return now }, Store: metadataWriterFunc(func(context.Context, string, WebAccount, Channel, Policy, time.Time) (WebAccount, error) {
				panic("failed metadata lookup must not write")
			})}
			account := healthyAccount(now)
			account.Revision = now
			outer := context.Background()
			_, err = refresh.Refresh(outer, runtimeChannel, account)
			require.Error(t, err)
			require.NoError(t, outer.Err())
			require.NotContains(t, err.Error(), "synthetic-private-detail")
			require.NotContains(t, err.Error(), "synthetic-key")
			if scenario == "request timeout" || scenario == "body timeout" {
				require.ErrorIs(t, err, ErrRuntimeOutcomeUnknown)
				require.EqualValues(t, 1, r.Observe(runtimeChannel).Pending)
			} else {
				require.False(t, errors.Is(err, ErrRuntimeOutcomeUnknown))
				require.Zero(t, r.Observe(runtimeChannel).Pending)
			}
		})
	}
}
