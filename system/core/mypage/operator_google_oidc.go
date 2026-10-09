package mypage

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const googleOperatorChallengeLifetime = 5 * time.Minute

var (
	googleOperatorClientID = regexp.MustCompile(`^[A-Za-z0-9-]{1,128}\.apps\.googleusercontent\.com$`)
	googleOperatorSubject  = regexp.MustCompile(`^[\x21-\x7e]{1,255}$`)
	pkceS256Challenge      = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

// GoogleOperatorJWKS and GoogleOperatorIDTokenSource have no live implementation.
// A future operator-only desktop code+PKCE client and bounded Google key fetch
// require separate review. Neither interface grants Firestore permission.
type GoogleOperatorJWKS interface {
	Keys(context.Context) (jose.JSONWebKeySet, error)
}

type GoogleOperatorIDTokenSource interface {
	IDToken(context.Context) (string, error)
}

// GoogleOperatorChallengeStore must atomically consume one matching nonce.
// A process-local fake is sufficient only for pure tests, never for live use.
type GoogleOperatorChallengeStore interface {
	Put(context.Context, GoogleOperatorChallengeRecord) error
	Consume(context.Context, string, string, time.Time, time.Time) error
}

type GoogleOperatorChallengeRecord struct {
	NonceHash string
	Binding   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type GoogleOperatorChallenge struct {
	Nonce     string
	State     string
	ExpiresAt time.Time
	seal      string
}

// GoogleHumanOperatorAuthority proves one allowlisted human Google subject for
// one operation. It is deliberately not wired into an operator command or
// Firestore writer. Completion evidence remains independently unavailable.
type GoogleHumanOperatorAuthority struct {
	ClientID       string
	AllowedSubject string
	Environment    string
	ProjectID      string
	RedirectURI    string
	ActorKey       []byte
	Keys           GoogleOperatorJWKS
	Tokens         GoogleOperatorIDTokenSource
	Challenges     GoogleOperatorChallengeStore
	Clock          func() time.Time
}

func (a *GoogleHumanOperatorAuthority) ready() bool {
	return a != nil && googleOperatorClientID.MatchString(a.ClientID) && googleOperatorSubject.MatchString(a.AllowedSubject) &&
		validSupportEnvironment(a.Environment) && a.ProjectID != "" && len(a.ActorKey) >= 32 && a.Keys != nil && a.Tokens != nil && a.Challenges != nil && a.Clock != nil
}

func (a *GoogleHumanOperatorAuthority) validIntent(intent OperatorIntent) bool {
	return a.ready() && intent.Environment == a.Environment && intent.ProjectID == a.ProjectID &&
		(intent.Action == "reply" || intent.Action == "complete") && validSupportPurpose(intent.Purpose) &&
		(intent.Action != "complete" || intent.Purpose != SupportDelete) && validOpaque(intent.RequestRef) &&
		validOpaque(intent.ProofRef) && validOpaque(intent.OperationID) && validOpaque(intent.ReplyDigest) &&
		validOpaque(intent.OperationDigest) && intent.ExpectedRevision >= 0 &&
		(intent.Action != "reply" || intent.ReplyDigest == intent.OperationDigest) &&
		(intent.Action != "complete" || intent.ExpectedRevision >= 1)
}

func (a *GoogleHumanOperatorAuthority) mac(parts ...string) string {
	h := hmac.New(sha256.New, a.ActorKey)
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (a *GoogleHumanOperatorAuthority) binding(intent OperatorIntent) string {
	return a.mac("operator-oidc-v1", intent.Action, intent.Environment, intent.ProjectID, string(intent.Purpose),
		intent.RequestRef, intent.ProofRef, strconv.FormatInt(intent.ExpectedRevision, 10), intent.OperationID,
		intent.ReplyDigest, intent.OperationDigest)
}

// Begin stores only hashes and returns fresh opaque state/nonce. It does not
// receive a receipt body/channel, create an OAuth client or contact Google.
func (a *GoogleHumanOperatorAuthority) Begin(ctx context.Context, intent OperatorIntent) (GoogleOperatorChallenge, error) {
	if ctx.Err() != nil || !a.validIntent(intent) {
		return GoogleOperatorChallenge{}, ErrOperatorDenied
	}
	var random [64]byte
	if _, err := rand.Read(random[:]); err != nil {
		return GoogleOperatorChallenge{}, ErrOperatorDenied
	}
	now := a.Clock().UTC()
	if now.IsZero() {
		return GoogleOperatorChallenge{}, ErrOperatorDenied
	}
	challenge := GoogleOperatorChallenge{
		Nonce:     base64.RawURLEncoding.EncodeToString(random[:32]),
		State:     base64.RawURLEncoding.EncodeToString(random[32:]),
		ExpiresAt: now.Add(googleOperatorChallengeLifetime),
	}
	challenge.seal = a.mac("operator-url-v1", challenge.Nonce, challenge.State, challenge.ExpiresAt.Format(time.RFC3339Nano))
	record := GoogleOperatorChallengeRecord{NonceHash: digest(challenge.Nonce), Binding: a.binding(intent), IssuedAt: now, ExpiresAt: challenge.ExpiresAt}
	if a.Challenges.Put(ctx, record) != nil {
		return GoogleOperatorChallenge{}, ErrOperatorDenied
	}
	return challenge, nil
}

// AuthorizationURL contains only the OAuth client, loopback callback, PKCE,
// state and nonce. No receipt, proof, channel, body or operator email enters it.
// State and code exchange verification are intentionally not implemented here.
func (a *GoogleHumanOperatorAuthority) AuthorizationURL(challenge GoogleOperatorChallenge, codeChallenge string) (string, error) {
	if !a.ready() || !pkceS256Challenge.MatchString(codeChallenge) || len(challenge.State) != 43 || len(challenge.Nonce) != 43 ||
		!a.Clock().Before(challenge.ExpiresAt) || challenge.ExpiresAt.After(a.Clock().Add(googleOperatorChallengeLifetime)) || a.RedirectURI == "" {
		return "", ErrOperatorDenied
	}
	wantSeal := a.mac("operator-url-v1", challenge.Nonce, challenge.State, challenge.ExpiresAt.Format(time.RFC3339Nano))
	if !hmac.Equal([]byte(challenge.seal), []byte(wantSeal)) {
		return "", ErrOperatorDenied
	}
	redirect, err := url.Parse(a.RedirectURI)
	if err != nil || redirect.Scheme != "http" || redirect.Hostname() != "127.0.0.1" || redirect.Port() == "" || redirect.Path != "/callback" || redirect.RawQuery != "" || redirect.Fragment != "" || redirect.User != nil {
		return "", ErrOperatorDenied
	}
	port, err := strconv.Atoi(redirect.Port())
	if err != nil || port < 1 || port > 65535 {
		return "", ErrOperatorDenied
	}
	u := url.URL{Scheme: "https", Host: "accounts.google.com", Path: "/o/oauth2/v2/auth"}
	query := url.Values{"client_id": {a.ClientID}, "redirect_uri": {a.RedirectURI}, "response_type": {"code"}, "scope": {"openid email"}, "state": {challenge.State}, "nonce": {challenge.Nonce}, "code_challenge": {codeChallenge}, "code_challenge_method": {"S256"}}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

type googleOperatorClaims struct {
	jwt.Claims
	AuthorizedParty string          `json:"azp"`
	Nonce           string          `json:"nonce"`
	Email           string          `json:"email"`
	EmailVerified   json.RawMessage `json:"email_verified"`
}

func (a *GoogleHumanOperatorAuthority) verifiedClaims(ctx context.Context, raw string) (googleOperatorClaims, error) {
	if ctx.Err() != nil || raw == "" || len(raw) > 16384 {
		return googleOperatorClaims{}, ErrOperatorDenied
	}
	token, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil || len(token.Headers) != 1 {
		return googleOperatorClaims{}, ErrOperatorDenied
	}
	header := token.Headers[0]
	if header.KeyID == "" || len(header.KeyID) > 256 || header.JSONWebKey != nil ||
		(header.ExtraHeaders[jose.HeaderType] != nil && header.ExtraHeaders[jose.HeaderType] != "JWT") {
		return googleOperatorClaims{}, ErrOperatorDenied
	}
	keys, err := a.Keys.Keys(ctx)
	if err != nil || len(keys.Keys) == 0 || len(keys.Keys) > 32 {
		return googleOperatorClaims{}, ErrOperatorDenied
	}
	seen := map[string]bool{}
	var publicKey *rsa.PublicKey
	for _, key := range keys.Keys {
		rsaKey, ok := key.Key.(*rsa.PublicKey)
		if !ok || rsaKey.N.BitLen() < 2048 || key.KeyID == "" || seen[key.KeyID] || !key.Valid() || !key.IsPublic() ||
			(key.Algorithm != "" && key.Algorithm != "RS256") || (key.Use != "" && key.Use != "sig") {
			return googleOperatorClaims{}, ErrOperatorDenied
		}
		seen[key.KeyID] = true
		if key.KeyID == header.KeyID {
			publicKey = rsaKey
		}
	}
	if publicKey == nil {
		return googleOperatorClaims{}, ErrOperatorDenied
	}
	var claims googleOperatorClaims
	if token.Claims(publicKey, &claims) != nil {
		return googleOperatorClaims{}, ErrOperatorDenied
	}
	now := a.Clock()
	verified := string(claims.EmailVerified)
	if (claims.Issuer != "https://accounts.google.com" && claims.Issuer != "accounts.google.com") ||
		len(claims.Audience) != 1 || claims.Audience[0] != a.ClientID ||
		(claims.AuthorizedParty != "" && claims.AuthorizedParty != a.ClientID) ||
		claims.Subject != a.AllowedSubject || claims.Nonce == "" || len(claims.Nonce) != 43 ||
		claims.Expiry == nil || claims.IssuedAt == nil || !now.Before(claims.Expiry.Time()) || claims.IssuedAt.Time().After(now) ||
		claims.Expiry.Time().Before(claims.IssuedAt.Time()) || claims.Email == "" || !strings.Contains(claims.Email, "@") ||
		(verified != "true" && verified != `"true"`) || ctx.Err() != nil {
		return googleOperatorClaims{}, ErrOperatorDenied
	}
	return claims, nil
}

func (a *GoogleHumanOperatorAuthority) Authorize(ctx context.Context, intent OperatorIntent) (OperatorIdentity, error) {
	if ctx.Err() != nil || !a.validIntent(intent) {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	raw, err := a.Tokens.IDToken(ctx)
	if err != nil {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	claims, err := a.verifiedClaims(ctx, raw)
	if err != nil || a.Challenges.Consume(ctx, digest(claims.Nonce), a.binding(intent), claims.IssuedAt.Time(), a.Clock()) != nil {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	return OperatorIdentity{Subject: "google-operator-" + a.mac("subject", claims.Subject)[:32], Environment: intent.Environment, ProjectID: intent.ProjectID}, nil
}

func (*GoogleHumanOperatorAuthority) VerifyEvidence(context.Context, OperatorIdentity, CompletionOperation) (VerifiedCompletionEvidence, error) {
	return VerifiedCompletionEvidence{}, ErrOperatorDenied
}
