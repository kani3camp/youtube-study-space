package mypage

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func TestKeylessFirebaseSDKUsesOnlyExplicitIAMSignerWithoutLocalKeyOrMetadata(t *testing.T) {
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	calls := 0
	client := &http.Client{Transport: providerTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/mypage-signer@demo-mypage.iam.gserviceaccount.com:signBlob", r.URL.String())
		require.Equal(t, http.MethodPost, r.Method)
		var body struct {
			Payload string `json:"payload"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		payload, err := base64.StdEncoding.DecodeString(body.Payload)
		require.NoError(t, err)
		hash := sha256.Sum256(payload)
		signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
		require.NoError(t, err)
		encoded, err := json.Marshal(map[string]string{"signedBlob": base64.StdEncoding.EncodeToString(signature)})
		require.NoError(t, err)
		return providerResponse(200, string(encoded)), nil
	})}
	creds := &google.Credentials{ProjectID: "demo-mypage", TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "synthetic-adc"})}
	sdk, err := NewKeylessFirebaseClient(context.Background(), "demo-mypage", "mypage-signer@demo-mypage.iam.gserviceaccount.com", creds, client)
	require.NoError(t, err)
	require.Zero(t, calls)
	boundary := &FirebaseBoundary{Client: sdk, ProjectID: "demo-mypage", Now: time.Now}
	raw, err := boundary.Mint(context.Background(), "UCsynthetic0000000000001")
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	signed, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
	require.NoError(t, err)
	var claims struct {
		Issuer   string `json:"iss"`
		Subject  string `json:"sub"`
		UID      string `json:"uid"`
		Audience string `json:"aud"`
	}
	require.NoError(t, signed.Claims(&key.PublicKey, &claims))
	require.Equal(t, "mypage-signer@demo-mypage.iam.gserviceaccount.com", claims.Issuer)
	require.Equal(t, claims.Issuer, claims.Subject)
	require.Equal(t, "UCsynthetic0000000000001", claims.UID)
	require.Equal(t, "https://identitytoolkit.googleapis.com/google.identity.identitytoolkit.v1.IdentityToolkit", claims.Audience)
}

func TestKeylessFirebaseRejectsKeysEmulatorsWrongProjectAndUnapprovedSignerBeforeIO(t *testing.T) {
	creds := &google.Credentials{ProjectID: "demo-mypage", TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "synthetic-adc"})}
	cases := []struct {
		name, signer string
		creds        *google.Credentials
		emulator     string
	}{
		{"missing credentials", "mypage-signer@demo-mypage.iam.gserviceaccount.com", nil, ""},
		{"wrong project", "mypage-signer@demo-other.iam.gserviceaccount.com", creds, ""},
		{"default compute signer", "synthetic@developer.gserviceaccount.com", creds, ""},
		{"emulator", "mypage-signer@demo-mypage.iam.gserviceaccount.com", creds, "127.0.0.1:9099"},
		{"credential JSON", "mypage-signer@demo-mypage.iam.gserviceaccount.com", &google.Credentials{JSON: []byte(`{"type":"service_account","private_key":"synthetic-never-valid-key"}`), TokenSource: creds.TokenSource}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", tc.emulator)
			calls := 0
			_, err := NewKeylessFirebaseClient(context.Background(), "demo-mypage", tc.signer, tc.creds, &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) { calls++; return providerResponse(503, "synthetic"), nil })})
			require.Error(t, err)
			require.Zero(t, calls)
		})
	}
}

type serverTestOAuth struct{}

func (serverTestOAuth) AuthorizationURL(string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth"
}

func (serverTestOAuth) Resolve(context.Context, string) ([]Channel, error) {
	return nil, apiError("OAUTH_FAILED")
}

func TestMyPageServerValidatesConfigurationAndConstructsWithoutBootstrapIO(t *testing.T) {
	config := ServerConfig{Environment: "development", ProjectID: "demo-mypage", ProjectNumber: "123456789", WebAppID: "1:123456789:web:synthetic", PublicOrigin: "https://example.invalid", Policy: Policy{Privacy: "synthetic-p", Terms: "synthetic-t"}}
	calls := 0
	deps := ServerDependencies{Firestore: &firestore.Client{}, Firebase: &fakeFirebaseClient{}, OAuth: serverTestOAuth{}, AppCheckHTTP: &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) { calls++; return providerResponse(503, "synthetic"), nil })}}
	server, err := NewMyPageServer(config, deps)
	require.NoError(t, err)
	require.Zero(t, calls)
	require.Equal(t, config.PublicOrigin, server.PublicOrigin)
	require.Equal(t, "development", server.Auth.Support.Environment)
	require.Same(t, server.Verifier, server.Auth.Minter)
	require.Nil(t, server.Intake)
	config.EnablePrivacyIntake = true
	_, err = NewMyPageServer(config, deps)
	require.Error(t, err)
	config.PrivacyIntakeSecret = []byte("synthetic-test-only-key-with-thirty-two-bytes")
	server, err = NewMyPageServer(config, deps)
	require.NoError(t, err)
	require.NotNil(t, server.Intake)
	config.EnablePrivacyIntake = false
	config.PrivacyIntakeSecret = nil
	config.Environment = "unknown"
	_, err = NewMyPageServer(config, deps)
	require.Error(t, err)
	config.Environment = "development"
	config.PublicOrigin = "http://example.invalid"
	_, err = NewMyPageServer(config, deps)
	require.Error(t, err)
	require.Zero(t, calls)
}
