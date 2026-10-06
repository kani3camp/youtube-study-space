package mypage

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"golang.org/x/sync/singleflight"
)

const appCheckJWKSURL = "https://firebaseappcheck.googleapis.com/v1/jwks"

var projectNumberPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

// AppCheckVerifier follows Firebase's documented JWT verification steps with
// exact environment issuer/audience/AppID, bounded lazy JWKS refresh, and no
// credentials. Its constructor does not fetch keys or start a background loop.
type AppCheckVerifier struct {
	projectNumber, appID   string
	client                 *http.Client
	now                    func() time.Time
	mu                     sync.Mutex
	keys                   jose.JSONWebKeySet
	fetchedAt, attemptedAt time.Time
	group                  singleflight.Group
}

func NewAppCheckVerifier(projectNumber, appID string, client *http.Client, now func() time.Time) (*AppCheckVerifier, error) {
	if !projectNumberPattern.MatchString(projectNumber) || len(appID) > 256 || len(appID) <= len("1:"+projectNumber+":web:") || appID[:len("1:"+projectNumber+":web:")] != "1:"+projectNumber+":web:" || now == nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	bounded := http.Client{Timeout: 2 * time.Second}
	if client != nil {
		bounded = *client
	}
	if bounded.Timeout <= 0 || bounded.Timeout > 2*time.Second {
		bounded.Timeout = 2 * time.Second
	}
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &AppCheckVerifier{projectNumber: projectNumber, appID: appID, client: &bounded, now: now}, nil
}

func (v *AppCheckVerifier) cachedKey(kid string) (*rsa.PublicKey, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.fetchedAt.IsZero() || v.now().Before(v.fetchedAt) || v.now().Sub(v.fetchedAt) >= 6*time.Hour {
		return nil, false
	}
	keys := v.keys.Key(kid)
	if len(keys) != 1 {
		return nil, false
	}
	key, ok := keys[0].Key.(*rsa.PublicKey)
	return key, ok
}

func (v *AppCheckVerifier) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if key, ok := v.cachedKey(kid); ok {
		return key, nil
	}
	result := v.group.DoChan("jwks", func() (any, error) {
		if key, ok := v.cachedKey(kid); ok {
			return key, nil
		}
		v.mu.Lock()
		now := v.now()
		// Unknown kid / failed fetch cannot turn every rejected token into a fetch.
		if !v.attemptedAt.IsZero() && !now.Before(v.attemptedAt) && now.Sub(v.attemptedAt) < time.Minute {
			v.mu.Unlock()
			return nil, apiError("APP_CHECK_REQUIRED")
		}
		v.attemptedAt = now
		v.mu.Unlock()
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, appCheckJWKSURL, nil)
		if err != nil {
			return nil, apiError("APP_CHECK_REQUIRED")
		}
		response, err := v.client.Do(request)
		if err != nil {
			return nil, apiError("APP_CHECK_REQUIRED")
		}
		defer response.Body.Close() //nolint:errcheck // Read-only JWKS close errors cannot alter validation and must not be logged.
		if response.StatusCode != 200 {
			return nil, apiError("APP_CHECK_REQUIRED")
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(data) > 65536 {
			return nil, apiError("APP_CHECK_REQUIRED")
		}
		var keys jose.JSONWebKeySet
		if json.Unmarshal(data, &keys) != nil || len(keys.Keys) == 0 || len(keys.Keys) > 32 {
			return nil, apiError("APP_CHECK_REQUIRED")
		}
		seen := map[string]bool{}
		for _, key := range keys.Keys {
			rsaKey, ok := key.Key.(*rsa.PublicKey)
			if !ok || rsaKey.N.BitLen() < 2048 || key.KeyID == "" || seen[key.KeyID] || !key.Valid() || !key.IsPublic() || (key.Algorithm != "" && key.Algorithm != "RS256") || (key.Use != "" && key.Use != "sig") {
				return nil, apiError("APP_CHECK_REQUIRED")
			}
			seen[key.KeyID] = true
		}
		v.mu.Lock()
		v.keys = keys
		v.fetchedAt = v.now()
		v.mu.Unlock()
		if key, ok := v.cachedKey(kid); ok {
			return key, nil
		}
		return nil, apiError("APP_CHECK_REQUIRED")
	})
	select {
	case <-ctx.Done():
		return nil, apiError("APP_CHECK_REQUIRED")
	case <-result:
		// A concurrent caller can be waiting for another kid: inspect its own key.
		if key, ok := v.cachedKey(kid); ok {
			return key, nil
		}
		return nil, apiError("APP_CHECK_REQUIRED")
	}
}

func (v *AppCheckVerifier) Verify(ctx context.Context, raw string) error {
	if ctx.Err() != nil || raw == "" || len(raw) > 16384 {
		return apiError("APP_CHECK_REQUIRED")
	}
	token, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil || len(token.Headers) != 1 {
		return apiError("APP_CHECK_REQUIRED")
	}
	header := token.Headers[0]
	if header.KeyID == "" || len(header.KeyID) > 256 || header.ExtraHeaders[jose.HeaderType] != "JWT" || header.JSONWebKey != nil {
		return apiError("APP_CHECK_REQUIRED")
	}
	key, err := v.publicKey(ctx, header.KeyID)
	if err != nil {
		return apiError("APP_CHECK_REQUIRED")
	}
	var claims jwt.Claims
	if token.Claims(key, &claims) != nil || claims.Expiry == nil || claims.IssuedAt == nil || !v.now().Before(claims.Expiry.Time()) || ctx.Err() != nil || claims.ValidateWithLeeway(jwt.Expected{Issuer: "https://firebaseappcheck.googleapis.com/" + v.projectNumber, AnyAudience: jwt.Audience{"projects/" + v.projectNumber}, Subject: v.appID, Time: v.now()}, 0) != nil {
		return apiError("APP_CHECK_REQUIRED")
	}
	return nil
}
