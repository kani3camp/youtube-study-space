package mypage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"firebase.google.com/go/v4/appcheck"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

// グローバルJWKSUrlはSDK公開のテスト境界。並列化せず、必ず戻す。
func signedAppCheckSDK(t *testing.T, key *rsa.PrivateKey) FirebaseAppCheckClient {
	t.Helper()
	body := publicJWKS(t, key, "synthetic-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
	t.Cleanup(server.Close)
	previous := appcheck.JWKSUrl
	appcheck.JWKSUrl = server.URL
	t.Cleanup(func() { appcheck.JWKSUrl = previous })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	sdk, err := NewFirebaseAppCheckClient(ctx, "123456789")
	require.NoError(t, err)
	return sdk
}
func TestSDKAppCheckPreservesEnvironmentAppExpiryAndMalformedClaimBoundary(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	v, err := NewAppCheckVerifier("123456789", "1:123456789:web:synthetic", signedAppCheckSDK(t, key), func() time.Time { return now })
	require.NoError(t, err)
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
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	require.EqualError(t, v.Verify(context.Background(), syntheticAppCheck(t, other, "synthetic-key", syntheticAppClaims(now), "JWT")), "APP_CHECK_REQUIRED")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.EqualError(t, v.Verify(ctx, "private-token"), "APP_CHECK_REQUIRED")
}

type rejectingAppCheck struct{}

func (rejectingAppCheck) VerifyToken(string) (*appcheck.DecodedAppCheckToken, error) {
	return nil, apiError("APP_CHECK_REQUIRED")
}
