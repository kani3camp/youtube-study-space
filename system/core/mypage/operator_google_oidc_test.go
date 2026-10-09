package mypage

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"
)

type fakeGoogleOperatorKeys struct {
	set jose.JSONWebKeySet
}

func (f fakeGoogleOperatorKeys) Keys(context.Context) (jose.JSONWebKeySet, error) { return f.set, nil }

type fakeGoogleOperatorToken struct {
	mu  sync.Mutex
	raw string
}

func (f *fakeGoogleOperatorToken) IDToken(context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.raw, nil
}

func (f *fakeGoogleOperatorToken) Set(raw string) {
	f.mu.Lock()
	f.raw = raw
	f.mu.Unlock()
}

// The fake's lock makes consuming a nonce an indivisible state transition.
type fakeGoogleOperatorChallenges struct {
	mu      sync.Mutex
	records map[string]GoogleOperatorChallengeRecord
}

func (f *fakeGoogleOperatorChallenges) Put(_ context.Context, record GoogleOperatorChallengeRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.records == nil {
		f.records = map[string]GoogleOperatorChallengeRecord{}
	}
	if _, exists := f.records[record.NonceHash]; exists {
		return ErrOperatorDenied
	}
	f.records[record.NonceHash] = record
	return nil
}

func (f *fakeGoogleOperatorChallenges) Consume(_ context.Context, nonceHash, binding string) (GoogleOperatorChallengeRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	record, exists := f.records[nonceHash]
	if !exists || record.Binding != binding {
		return GoogleOperatorChallengeRecord{}, ErrOperatorDenied
	}
	delete(f.records, nonceHash)
	return record, nil
}

// Reuse this contract with every future durable challenge-store implementation.
// Time validation belongs to the authority; the store owns atomic match/take.
func testGoogleOperatorChallengeStoreContract(t *testing.T, newStore func() GoogleOperatorChallengeStore) {
	t.Helper()
	t.Run("exact record and binding mismatch", func(t *testing.T) {
		store := newStore()
		record := GoogleOperatorChallengeRecord{NonceHash: digest("contract-nonce"), Binding: digest("contract-binding"), IssuedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 10, 9, 0, 5, 0, 0, time.UTC)}
		require.NoError(t, store.Put(context.Background(), record))
		_, err := store.Consume(context.Background(), record.NonceHash, digest("other-binding"))
		require.Error(t, err)
		got, err := store.Consume(context.Background(), record.NonceHash, record.Binding)
		require.NoError(t, err, "a mismatch must not consume the challenge")
		require.Equal(t, record, got)
		_, err = store.Consume(context.Background(), record.NonceHash, record.Binding)
		require.Error(t, err, "replay must fail")
	})
	t.Run("atomic single winner", func(t *testing.T) {
		store := newStore()
		record := GoogleOperatorChallengeRecord{NonceHash: digest("parallel-nonce"), Binding: digest("parallel-binding"), IssuedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 10, 9, 0, 5, 0, 0, time.UTC)}
		require.NoError(t, store.Put(context.Background(), record))
		start := make(chan struct{})
		var wg sync.WaitGroup
		var winners atomic.Int32
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				got, err := store.Consume(context.Background(), record.NonceHash, record.Binding)
				if err == nil {
					if got != record {
						t.Errorf("consume returned another challenge record")
					}
					winners.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
		require.Equal(t, int32(1), winners.Load())
	})
}

func TestGoogleOperatorChallengeStoreContract(t *testing.T) {
	testGoogleOperatorChallengeStoreContract(t, func() GoogleOperatorChallengeStore { return &fakeGoogleOperatorChallenges{} })
}

type googleOperatorFixture struct {
	authority *GoogleHumanOperatorAuthority
	operator  *FirestoreSupportOperator
	token     *fakeGoogleOperatorToken
	store     *fakeGoogleOperatorChallenges
	key       *rsa.PrivateKey
	now       time.Time
	intent    OperatorIntent
	request   ReplyOperation
}

func newGoogleOperatorFixture(t *testing.T) *googleOperatorFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	token := &fakeGoogleOperatorToken{}
	store := &fakeGoogleOperatorChallenges{}
	a := &GoogleHumanOperatorAuthority{
		ClientID:       "synthetic-operator.apps.googleusercontent.com",
		AllowedSubject: "123456789012345678901",
		Environment:    "development",
		ProjectID:      "demo-youtube-study-space-ci",
		RedirectURI:    "http://127.0.0.1:18083/callback",
		ActorKey:       []byte("synthetic-human-actor-key-thirty-two-bytes"),
		Keys: fakeGoogleOperatorKeys{set: jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "synthetic-google-kid", Algorithm: "RS256", Use: "sig"},
		}}},
		Tokens: token, Challenges: store, Clock: func() time.Time { return now },
	}
	op := &FirestoreSupportOperator{AuditKey: []byte("synthetic-operator-audit-key-thirty-two-bytes"), Authority: a, Clock: a.Clock}
	request := ReplyOperation{Intent: OperatorIntent{Environment: a.Environment, ProjectID: a.ProjectID, Purpose: SupportDisclosure, RequestRef: digest(t.Name() + ":request"), OperationID: digest(t.Name() + ":operation"), Action: "reply"}, ProofRef: digest(t.Name() + ":proof"), ExpectedRevision: 0, Body: "Synthetic reviewed private reply", At: now}
	intent, err := op.ReplyAuthorizationIntent(request)
	require.NoError(t, err)
	return &googleOperatorFixture{authority: a, operator: op, token: token, store: store, key: key, now: now, intent: intent, request: request}
}

func (f *googleOperatorFixture) claims(challenge GoogleOperatorChallenge) map[string]any {
	return map[string]any{
		"iss": "https://accounts.google.com", "aud": f.authority.ClientID, "azp": f.authority.ClientID,
		"sub": f.authority.AllowedSubject, "exp": f.now.Add(time.Hour).Unix(), "iat": f.now.Unix(),
		"nonce": challenge.Nonce, "email": "synthetic-operator@example.invalid", "email_verified": true,
	}
}

func signedGoogleOperatorToken(t *testing.T, key any, alg jose.SignatureAlgorithm, kid string, claims map[string]any, typ string) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, (&jose.SignerOptions{}).WithType(jose.ContentType(typ)).WithHeader(jose.HeaderKey("kid"), kid))
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func (f *googleOperatorFixture) signed(t *testing.T, claims map[string]any) string {
	t.Helper()
	return signedGoogleOperatorToken(t, f.key, jose.RS256, "synthetic-google-kid", claims, "JWT")
}

func TestGoogleHumanOperatorOIDCOneUseIdentityAndFreshRetry(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	ctx := context.Background()
	challenge, err := f.authority.Begin(ctx, f.intent)
	require.NoError(t, err)
	f.token.Set(f.signed(t, f.claims(challenge)))
	identity, err := f.authority.Authorize(ctx, f.intent)
	require.NoError(t, err)
	require.Equal(t, f.authority.Environment, identity.Environment)
	require.Equal(t, f.authority.ProjectID, identity.ProjectID)
	require.NotContains(t, identity.Subject, f.authority.AllowedSubject)
	require.NotContains(t, identity.Subject, "example.invalid")
	_, err = f.authority.Authorize(ctx, f.intent)
	require.ErrorIs(t, err, ErrOperatorDenied, "the nonce was consumed")
	retry, err := f.authority.Begin(ctx, f.intent)
	require.NoError(t, err)
	require.NotEqual(t, challenge.Nonce, retry.Nonce)
	_, err = f.authority.Authorize(ctx, f.intent)
	require.ErrorIs(t, err, ErrOperatorDenied, "an old ID token cannot satisfy a new challenge")
	f.token.Set(f.signed(t, f.claims(retry)))
	retried, err := f.authority.Authorize(ctx, f.intent)
	require.NoError(t, err, "same operation may retry with a fresh human proof")
	require.Equal(t, identity, retried)
	require.Equal(t, f.now.Add(googleOperatorChallengeLifetime), retry.ExpiresAt)
}

func TestGoogleHumanOperatorOIDCRejectsWrongOperationBinding(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	ctx := context.Background()
	challenge, err := f.authority.Begin(ctx, f.intent)
	require.NoError(t, err)
	f.token.Set(f.signed(t, f.claims(challenge)))
	for name, change := range map[string]func(*OperatorIntent){
		"action":      func(i *OperatorIntent) { i.Action = "complete"; i.ExpectedRevision = 1 },
		"environment": func(i *OperatorIntent) { i.Environment = "production" },
		"project":     func(i *OperatorIntent) { i.ProjectID = "demo-other-project" },
		"purpose":     func(i *OperatorIntent) { i.Purpose = SupportRevoke },
		"request":     func(i *OperatorIntent) { i.RequestRef = digest("other-request") },
		"proof":       func(i *OperatorIntent) { i.ProofRef = digest("other-proof") },
		"revision":    func(i *OperatorIntent) { i.ExpectedRevision++ },
		"operation":   func(i *OperatorIntent) { i.OperationID = digest("other-operation") },
		"digest": func(i *OperatorIntent) {
			i.ReplyDigest, i.OperationDigest = digest("other-payload"), digest("other-payload")
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := f.intent
			change(&changed)
			_, err := f.authority.Authorize(ctx, changed)
			require.ErrorIs(t, err, ErrOperatorDenied)
		})
	}
	changedBody := f.request
	changedBody.Body = "Other synthetic reviewed reply"
	otherIntent, err := f.operator.ReplyAuthorizationIntent(changedBody)
	require.NoError(t, err)
	require.NotEqual(t, f.intent.ReplyDigest, otherIntent.ReplyDigest)
	_, err = f.authority.Authorize(ctx, otherIntent)
	require.ErrorIs(t, err, ErrOperatorDenied, "a changed body cannot reuse the human proof")
	_, err = f.authority.Authorize(ctx, f.intent)
	require.NoError(t, err, "mismatched attempts cannot consume the correct challenge")
}

func TestGoogleHumanOperatorOIDCRejectsTokenConfusionAndClaimFailures(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for name, mutate := range map[string]func(*testing.T, map[string]any, *googleOperatorFixture) string{
		"wrong issuer": func(_ *testing.T, c map[string]any, f *googleOperatorFixture) string {
			c["iss"] = "https://securetoken.google.com/" + f.authority.ProjectID
			return ""
		},
		"issuer lookalike": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			c["iss"] = "https://accounts.google.com.attacker.invalid"
			return ""
		},
		"wrong audience": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			c["aud"] = "other.apps.googleusercontent.com"
			return ""
		},
		"multiple audiences": func(_ *testing.T, c map[string]any, f *googleOperatorFixture) string {
			c["aud"] = []string{f.authority.ClientID, "other.apps.googleusercontent.com"}
			return ""
		},
		"wrong azp": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			c["azp"] = "other.apps.googleusercontent.com"
			return ""
		},
		"expired": func(_ *testing.T, c map[string]any, f *googleOperatorFixture) string {
			c["exp"] = f.now.Unix()
			return ""
		},
		"future iat": func(_ *testing.T, c map[string]any, f *googleOperatorFixture) string {
			c["iat"] = f.now.Add(time.Second).Unix()
			return ""
		},
		"missing exp": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string { delete(c, "exp"); return "" },
		"missing iat": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string { delete(c, "iat"); return "" },
		"wrong nonce": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			c["nonce"] = base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))
			return ""
		},
		"missing nonce": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string { delete(c, "nonce"); return "" },
		"unverified email": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			c["email_verified"] = false
			return ""
		},
		"missing email": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string { delete(c, "email"); return "" },
		"account switch": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			c["sub"] = "987654321098765432109"
			return ""
		},
		"service account": func(_ *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			c["sub"] = "synthetic-service-account"
			delete(c, "nonce")
			return ""
		},
		"wrong key": func(t *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			return signedGoogleOperatorToken(t, otherKey, jose.RS256, "synthetic-google-kid", c, "JWT")
		},
		"unknown key ID": func(t *testing.T, c map[string]any, f *googleOperatorFixture) string {
			return signedGoogleOperatorToken(t, f.key, jose.RS256, "unknown-google-kid", c, "JWT")
		},
		"wrong algorithm": func(t *testing.T, c map[string]any, _ *googleOperatorFixture) string {
			return signedGoogleOperatorToken(t, []byte("synthetic-incorrect-algorithm-secret"), jose.HS256, "synthetic-google-kid", c, "JWT")
		},
		"wrong type": func(t *testing.T, c map[string]any, f *googleOperatorFixture) string {
			return signedGoogleOperatorToken(t, f.key, jose.RS256, "synthetic-google-kid", c, "not-JWT")
		},
		"unsigned": func(_ *testing.T, _ map[string]any, _ *googleOperatorFixture) string {
			return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"123456789012345678901"}`)) + "."
		},
		"gcloud access token": func(_ *testing.T, _ map[string]any, _ *googleOperatorFixture) string {
			return "ya29.synthetic-gcloud-access-token"
		},
		"ADC access token": func(_ *testing.T, _ map[string]any, _ *googleOperatorFixture) string {
			return "ya29.synthetic-ADC-access-token"
		},
	} {
		t.Run(name, func(t *testing.T) {
			challenge, err := f.authority.Begin(context.Background(), f.intent)
			require.NoError(t, err)
			claims := f.claims(challenge)
			raw := mutate(t, claims, f)
			if raw == "" {
				raw = f.signed(t, claims)
			}
			f.token.Set(raw)
			_, err = f.authority.Authorize(context.Background(), f.intent)
			require.ErrorIs(t, err, ErrOperatorDenied)
			require.Equal(t, ErrOperatorDenied.Error(), err.Error(), "errors cannot contain a token, body or email")
			f.token.Set(f.signed(t, f.claims(challenge)))
			_, err = f.authority.Authorize(context.Background(), f.intent)
			require.NoError(t, err, "invalid tokens must not consume a challenge")
		})
	}
	challenge, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	claims := f.claims(challenge)
	claims["iss"] = "accounts.google.com"
	claims["email_verified"] = "true"
	f.token.Set(f.signed(t, claims))
	_, err = f.authority.Authorize(context.Background(), f.intent)
	require.NoError(t, err, "both documented Google issuer forms and email_verified forms are accepted")
}

func TestGoogleHumanOperatorOIDCRejectsUntrustedJWKSAndUnsafeRedirect(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	challenge, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	f.token.Set(f.signed(t, f.claims(challenge)))
	goodKeys := f.authority.Keys
	for name, set := range map[string]jose.JSONWebKeySet{
		"empty":         {},
		"duplicate kid": {Keys: []jose.JSONWebKey{{Key: &f.key.PublicKey, KeyID: "synthetic-google-kid", Algorithm: "RS256", Use: "sig"}, {Key: &f.key.PublicKey, KeyID: "synthetic-google-kid", Algorithm: "RS256", Use: "sig"}}},
		"wrong key use": {Keys: []jose.JSONWebKey{{Key: &f.key.PublicKey, KeyID: "synthetic-google-kid", Algorithm: "RS256", Use: "enc"}}},
		"typed nil RSA": {Keys: []jose.JSONWebKey{{Key: (*rsa.PublicKey)(nil), KeyID: "synthetic-google-kid", Algorithm: "RS256", Use: "sig"}}},
		"nil RSA N":     {Keys: []jose.JSONWebKey{{Key: &rsa.PublicKey{}, KeyID: "synthetic-google-kid", Algorithm: "RS256", Use: "sig"}}},
	} {
		t.Run(name, func(t *testing.T) {
			f.authority.Keys = fakeGoogleOperatorKeys{set: set}
			_, err := f.authority.Authorize(context.Background(), f.intent)
			require.ErrorIs(t, err, ErrOperatorDenied)
		})
	}
	f.authority.Keys = goodKeys
	_, err = f.authority.Authorize(context.Background(), f.intent)
	require.NoError(t, err, "bad keys cannot consume the valid challenge")
	for _, redirect := range []string{"https://example.invalid/callback", "http://127.0.0.1:18083/callback?request=private", "http://127.0.0.1:18083/private", "http://127.0.0.1:99999/callback"} {
		f.authority.RedirectURI = redirect
		_, err = f.authority.AuthorizationURL(challenge, base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("p", 32))))
		require.ErrorIs(t, err, ErrOperatorDenied)
	}
	f.authority.RedirectURI = "http://127.0.0.1:18083/callback"
	forged := challenge
	forged.State = base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("r", 32)))
	_, err = f.authority.AuthorizationURL(forged, base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("p", 32))))
	require.ErrorIs(t, err, ErrOperatorDenied, "only Begin-issued opaque state can enter the URL")
}

func TestGoogleHumanOperatorOIDCChecksChallengeTimeAfterAtomicTake(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	for name, change := range map[string]func(*GoogleOperatorChallengeRecord){
		"expired at boundary": func(r *GoogleOperatorChallengeRecord) {
			r.IssuedAt = f.now.Add(-googleOperatorChallengeLifetime)
			r.ExpiresAt = f.now
		},
		"future issue": func(r *GoogleOperatorChallengeRecord) {
			r.IssuedAt = f.now.Add(time.Second)
			r.ExpiresAt = r.IssuedAt.Add(googleOperatorChallengeLifetime)
		},
		"extended lifetime": func(r *GoogleOperatorChallengeRecord) {
			r.ExpiresAt = r.ExpiresAt.Add(time.Second)
		},
		"wrong returned nonce": func(r *GoogleOperatorChallengeRecord) {
			r.NonceHash = digest("other-nonce")
		},
	} {
		t.Run(name, func(t *testing.T) {
			challenge, err := f.authority.Begin(context.Background(), f.intent)
			require.NoError(t, err)
			f.store.mu.Lock()
			record := f.store.records[digest(challenge.Nonce)]
			change(&record)
			f.store.records[digest(challenge.Nonce)] = record
			f.store.mu.Unlock()
			f.token.Set(f.signed(t, f.claims(challenge)))
			_, err = f.authority.Authorize(context.Background(), f.intent)
			require.ErrorIs(t, err, ErrOperatorDenied, "verifier checks time and record contents even when store takes the nonce")
		})
	}
	challenge, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	oldClaims := f.claims(challenge)
	oldClaims["iat"] = f.now.Add(-time.Second).Unix()
	f.token.Set(f.signed(t, oldClaims))
	_, err = f.authority.Authorize(context.Background(), f.intent)
	require.ErrorIs(t, err, ErrOperatorDenied, "ID token must be issued after this challenge")
	valid, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	f.token.Set(f.signed(t, f.claims(valid)))
	var clockCalls atomic.Int32
	f.authority.Clock = func() time.Time { clockCalls.Add(1); return f.now }
	_, err = f.authority.Authorize(context.Background(), f.intent)
	require.NoError(t, err)
	require.Equal(t, int32(1), clockCalls.Load(), "token and challenge use one trusted clock snapshot")
}

func TestGoogleHumanOperatorOIDCNonceConsumeIsAtomic(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	challenge, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	f.token.Set(f.signed(t, f.claims(challenge)))
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := f.authority.Authorize(context.Background(), f.intent)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrOperatorDenied) {
				t.Errorf("unexpected authorization error: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
}

func TestGoogleHumanOperatorOIDCShortLifetimeSafeURLAndPrivateAudit(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	challenge, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	u, err := f.authority.AuthorizationURL(challenge, base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("p", 32))))
	require.NoError(t, err)
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	require.Equal(t, "https://accounts.google.com/o/oauth2/v2/auth", parsed.Scheme+"://"+parsed.Host+parsed.Path)
	require.Equal(t, "openid email", parsed.Query().Get("scope"))
	require.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
	for _, private := range []string{f.request.Body, f.intent.RequestRef, f.intent.ProofRef, f.intent.ReplyDigest, "synthetic-operator@example.invalid"} {
		require.NotContains(t, u, private)
	}
	f.token.Set(f.signed(t, f.claims(challenge)))
	identity, err := f.authority.Authorize(context.Background(), f.intent)
	require.NoError(t, err)
	audit := f.operator.audit(f.intent, identity.Subject, f.intent.OperationDigest, f.now)
	encoded := fmt.Sprint(audit)
	for _, private := range []string{f.token.raw, f.request.Body, "synthetic-operator@example.invalid", f.authority.AllowedSubject, f.intent.RequestRef, f.intent.ProofRef} {
		require.NotContains(t, encoded, private)
	}
	_, err = f.authority.VerifyEvidence(context.Background(), identity, CompletionOperation{})
	require.ErrorIs(t, err, ErrOperatorDenied, "human ID alone cannot attest an action or delivery")
	expired, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	f.authority.Clock = func() time.Time { return f.now.Add(googleOperatorChallengeLifetime) }
	f.token.Set(f.signed(t, f.claims(expired)))
	_, err = f.authority.Authorize(context.Background(), f.intent)
	require.ErrorIs(t, err, ErrOperatorDenied, "an unconsumed challenge expires at five minutes")
	trusted, err := f.operator.operationTime(f.now.Add(-24 * time.Hour))
	require.NoError(t, err)
	require.Equal(t, f.now, trusted, "caller At cannot backdate a future human operation")
	f.operator.Clock = nil
	_, err = f.operator.operationTime(f.now)
	require.ErrorIs(t, err, ErrOperatorDenied)
}

func TestGoogleHumanOperatorOIDCCompletionIntentBindsReplyAndEvidence(t *testing.T) {
	f := newGoogleOperatorFixture(t)
	op := CompletionOperation{
		Intent:   OperatorIntent{Environment: f.authority.Environment, ProjectID: f.authority.ProjectID, RequestRef: f.intent.RequestRef, Purpose: SupportDisclosure, OperationID: digest("synthetic-completion-operation"), Action: "complete"},
		ProofRef: f.intent.ProofRef, ReplyOperationID: f.intent.OperationID, ExpectedRevision: 1,
		ActionEvidence: digest("synthetic-action-evidence"), DeliveryAcknowledgement: digest("synthetic-delivery-ack"), ReplyDigest: f.intent.ReplyDigest, At: f.now,
	}
	intent, err := f.operator.CompletionAuthorizationIntent(op)
	require.NoError(t, err)
	challenge, err := f.authority.Begin(context.Background(), intent)
	require.NoError(t, err)
	f.token.Set(f.signed(t, f.claims(challenge)))
	for _, change := range []func(*CompletionOperation){
		func(v *CompletionOperation) { v.ReplyDigest = digest("other-reply") },
		func(v *CompletionOperation) { v.ActionEvidence = digest("other-action") },
		func(v *CompletionOperation) { v.DeliveryAcknowledgement = digest("other-delivery") },
		func(v *CompletionOperation) { v.ReplyOperationID = digest("other-reply-operation") },
	} {
		changed := op
		change(&changed)
		other, buildErr := f.operator.CompletionAuthorizationIntent(changed)
		require.NoError(t, buildErr)
		_, err = f.authority.Authorize(context.Background(), other)
		require.ErrorIs(t, err, ErrOperatorDenied)
	}
	_, err = f.authority.Authorize(context.Background(), intent)
	require.NoError(t, err)
}
