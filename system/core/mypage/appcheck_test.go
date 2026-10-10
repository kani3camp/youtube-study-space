package mypage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
)

func syntheticAppCheck(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any, typ string) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType(jose.ContentType(typ)).WithHeader(jose.HeaderKey("kid"), kid))
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func syntheticAppClaims(now time.Time) map[string]any {
	return map[string]any{"iss": "https://firebaseappcheck.googleapis.com/123456789", "aud": []string{"projects/123456789"}, "sub": "1:123456789:web:synthetic", "exp": now.Add(time.Hour).Unix(), "iat": now.Unix()}
}

func publicJWKS(t *testing.T, key *rsa.PrivateKey, kid string) string {
	t.Helper()
	data, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: kid, Algorithm: "RS256", Use: "sig"}}})
	require.NoError(t, err)
	return string(data)
}

func TestAppCheckSignedClaimsRejectWrongEnvironmentAppExpiryAndMalformedTypes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	body := publicJWKS(t, key, "synthetic-key")
	calls := 0
	v, err := NewAppCheckVerifier("123456789", "1:123456789:web:synthetic", &http.Client{Transport: providerTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, appCheckJWKSURL, r.URL.String())
		require.Empty(t, r.Header.Get("Authorization"))
		return providerResponse(200, body), nil
	})}, func() time.Time { return now })
	require.NoError(t, err)
	require.Zero(t, calls)
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		typ    string
	}{
		{"valid", func(map[string]any) {}, "JWT"},
		{"issuer prefix lookalike", func(c map[string]any) { c["iss"] = "https://firebaseappcheck.googleapis.com/987654321" }, "JWT"},
		{"wrong audience", func(c map[string]any) { c["aud"] = []string{"projects/987654321"} }, "JWT"},
		{"wrong app", func(c map[string]any) { c["sub"] = "1:123456789:web:other" }, "JWT"},
		{"expiry boundary", func(c map[string]any) { c["exp"] = now.Unix() }, "JWT"},
		{"expired", func(c map[string]any) { c["exp"] = now.Add(-time.Second).Unix() }, "JWT"},
		{"future issue", func(c map[string]any) { c["iat"] = now.Add(time.Second).Unix() }, "JWT"},
		{"missing exp", func(c map[string]any) { delete(c, "exp") }, "JWT"},
		{"missing iat", func(c map[string]any) { delete(c, "iat") }, "JWT"},
		{"numeric issuer", func(c map[string]any) { c["iss"] = 123 }, "JWT"},
		{"numeric audience", func(c map[string]any) { c["aud"] = 123 }, "JWT"},
		{"mixed audience", func(c map[string]any) { c["aud"] = []any{"projects/123456789", 123} }, "JWT"},
		{"wrong type", func(map[string]any) {}, "not-JWT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := syntheticAppClaims(now)
			tc.mutate(claims)
			raw := syntheticAppCheck(t, key, "synthetic-key", claims, tc.typ)
			err := v.Verify(context.Background(), raw)
			if tc.name == "valid" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Equal(t, "APP_CHECK_REQUIRED", err.Error())
			}
		})
	}
	require.Equal(t, 1, calls)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.Equal(t, "APP_CHECK_REQUIRED", v.Verify(context.Background(), syntheticAppCheck(t, other, "synthetic-key", syntheticAppClaims(now), "JWT")).Error())
	require.Equal(t, "APP_CHECK_REQUIRED", v.Verify(context.Background(), "synthetic-invalid-secret").Error())
	require.Equal(t, 1, calls)
}

func TestAppCheckConcurrentVerificationCachesKeysAndUnknownKidRefreshIsBounded(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	body := publicJWKS(t, key, "synthetic-key")
	var calls atomic.Int32
	v, err := NewAppCheckVerifier("123456789", "1:123456789:web:synthetic", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) { calls.Add(1); return providerResponse(200, body), nil })}, func() time.Time { return now })
	require.NoError(t, err)
	token := syntheticAppCheck(t, key, "synthetic-key", syntheticAppClaims(now), "JWT")
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- v.Verify(context.Background(), token) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), calls.Load())
	unknown := syntheticAppCheck(t, key, "unknown-key", syntheticAppClaims(now), "JWT")
	for i := 0; i < 20; i++ {
		require.Error(t, v.Verify(context.Background(), unknown))
	}
	require.Equal(t, int32(1), calls.Load())
	now = now.Add(time.Minute)
	require.Error(t, v.Verify(context.Background(), unknown))
	require.Equal(t, int32(2), calls.Load())
	body = publicJWKS(t, key, "rotated-key")
	now = now.Add(time.Minute)
	require.NoError(t, v.Verify(context.Background(), syntheticAppCheck(t, key, "rotated-key", syntheticAppClaims(now), "JWT")))
	require.Equal(t, int32(3), calls.Load())
	now = now.Add(6 * time.Hour)
	require.NoError(t, v.Verify(context.Background(), syntheticAppCheck(t, key, "rotated-key", syntheticAppClaims(now), "JWT")))
	require.Equal(t, int32(4), calls.Load())
}

func TestAppCheckInvalidHeadersAndFetchFailuresFailClosedWithoutRepeatedRequests(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	v, err := NewAppCheckVerifier("123456789", "1:123456789:web:synthetic", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return providerResponse(503, "synthetic-upstream-private-error"), nil
	})}, func() time.Time { return now })
	require.NoError(t, err)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("synthetic-32-byte-signing-key-only")}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "synthetic-key"))
	require.NoError(t, err)
	unsignedRSA, err := jwt.Signed(signer).Claims(syntheticAppClaims(now)).Serialize()
	require.NoError(t, err)
	require.Error(t, v.Verify(context.Background(), unsignedRSA))
	require.Error(t, v.Verify(context.Background(), syntheticAppCheck(t, key, "", syntheticAppClaims(now), "JWT")))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, v.Verify(canceled, syntheticAppCheck(t, key, "synthetic-key", syntheticAppClaims(now), "JWT")))
	require.Zero(t, calls)
	token := syntheticAppCheck(t, key, "synthetic-key", syntheticAppClaims(now), "JWT")
	for i := 0; i < 5; i++ {
		err = v.Verify(context.Background(), token)
		require.Equal(t, "APP_CHECK_REQUIRED", err.Error())
	}
	require.Equal(t, 1, calls)
	now = now.Add(time.Minute)
	require.Error(t, v.Verify(context.Background(), token))
	require.Equal(t, 2, calls)
	for _, tc := range []struct{ project, app string }{{"demo-mypage", "1:123456789:web:synthetic"}, {"123456789", "1:987654321:web:synthetic"}, {"123456789", ""}} {
		_, err := NewAppCheckVerifier(tc.project, tc.app, nil, time.Now)
		require.Error(t, err)
	}
}
