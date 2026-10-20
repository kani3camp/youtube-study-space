//go:build integration

package mypage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

type trackedGoogleChallenges struct {
	*FirestoreGoogleOperatorChallenges
	t *testing.T
}

func googleChallengeEmulator(t *testing.T) *trackedGoogleChallenges {
	t.Helper()
	integrationtest.RequireFirestoreEmulator(t)
	client, err := firestore.NewClient(context.Background(), "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store, err := NewFirestoreGoogleOperatorChallenges(client, "demo-youtube-study-space-ci")
	require.NoError(t, err)
	return &trackedGoogleChallenges{FirestoreGoogleOperatorChallenges: store, t: t}
}

func (s *trackedGoogleChallenges) Put(ctx context.Context, record GoogleOperatorChallengeRecord) error {
	if canonicalOperatorHash(record.NonceHash) {
		ref := s.client.Collection(googleOperatorChallenges).Doc(record.NonceHash)
		s.t.Cleanup(func() {
			_, err := ref.Delete(context.Background())
			require.NoError(s.t, err)
		})
	}
	return s.FirestoreGoogleOperatorChallenges.Put(ctx, record)
}

func googleChallengeRecord(t *testing.T) GoogleOperatorChallengeRecord {
	t.Helper()
	namespace, err := opaque()
	require.NoError(t, err)
	issued := time.Date(2026, 10, 15, 0, 0, 0, 123456789, time.UTC)
	return GoogleOperatorChallengeRecord{NonceHash: digest(namespace), Binding: digest(namespace + ":binding"), IssuedAt: issued, ExpiresAt: issued.Add(googleOperatorChallengeLifetime)}
}

func TestGoogleOperatorFirestoreChallengeSharedContract(t *testing.T) {
	testGoogleOperatorChallengeStoreContract(t, func() GoogleOperatorChallengeStore { return googleChallengeEmulator(t) })
}

func TestGoogleOperatorFirestoreChallengeRestartAndParallelTakeNeverResurrect(t *testing.T) {
	first, restarted := googleChallengeEmulator(t), googleChallengeEmulator(t)
	record := googleChallengeRecord(t)
	ctx := context.Background()
	require.NoError(t, first.Put(ctx, record))
	require.ErrorIs(t, restarted.FirestoreGoogleOperatorChallenges.Put(ctx, record), ErrOperatorDenied, "duplicate issuance cannot overwrite a challenge")
	var winners atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range 20 {
		store := first.FirestoreGoogleOperatorChallenges
		if i%2 == 0 {
			store = restarted.FirestoreGoogleOperatorChallenges
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := store.Consume(ctx, record.NonceHash, record.Binding, func() time.Time { return record.ExpiresAt.Add(-time.Nanosecond) })
			if err == nil {
				if got != record {
					t.Errorf("consume returned another record")
				}
				winners.Add(1)
			} else if !errors.Is(err, ErrOperatorDenied) {
				t.Errorf("consume returned an unexpected error")
			}
		}()
	}
	close(start)
	wg.Wait()
	require.Equal(t, int32(1), winners.Load())
	// Model an ACK lost after the successful atomic take: another process must
	// neither replay the take nor recreate the same nonce with a delayed Put.
	_, err := restarted.Consume(ctx, record.NonceHash, record.Binding, func() time.Time { return record.IssuedAt.Add(time.Minute) })
	require.ErrorIs(t, err, ErrOperatorDenied)
	require.ErrorIs(t, restarted.FirestoreGoogleOperatorChallenges.Put(ctx, record), ErrOperatorDenied)
	doc, err := first.client.Collection(googleOperatorChallenges).Doc(record.NonceHash).Get(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"schemaVersion": int64(1), "consumed": true}, doc.Data())
	fresh := googleChallengeRecord(t)
	require.NoError(t, first.Put(ctx, fresh))
	_, err = restarted.Consume(ctx, fresh.NonceHash, fresh.Binding, func() time.Time { return fresh.IssuedAt })
	require.NoError(t, err, "a fresh challenge can proceed after a consumed or unknown prior attempt")
}

func TestGoogleOperatorFirestoreChallengeClockAndCancellationBoundaries(t *testing.T) {
	for _, when := range []string{"before take expiry", "after take expiry", "before take cancel", "after take cancel"} {
		t.Run(when, func(t *testing.T) {
			store := googleChallengeEmulator(t)
			record := googleChallengeRecord(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			require.NoError(t, store.Put(ctx, record))
			ref := store.client.Collection(googleOperatorChallenges).Doc(record.NonceHash)
			before, err := ref.Get(ctx)
			require.NoError(t, err)
			calls := 0
			_, err = store.Consume(ctx, record.NonceHash, record.Binding, func() time.Time {
				calls++
				trigger := calls == 1 && strings.HasPrefix(when, "before") || calls == 2 && strings.HasPrefix(when, "after")
				if trigger && strings.HasSuffix(when, "cancel") {
					cancel()
				}
				if trigger && strings.HasSuffix(when, "expiry") {
					return record.ExpiresAt
				}
				return record.IssuedAt.Add(time.Minute)
			})
			require.ErrorIs(t, err, ErrOperatorDenied)
			after, err := ref.Get(context.Background())
			require.NoError(t, err)
			if strings.HasPrefix(when, "before") {
				require.Equal(t, before.Data(), after.Data())
				require.Equal(t, before.UpdateTime, after.UpdateTime)
			} else {
				require.Equal(t, map[string]any{"schemaVersion": int64(1), "consumed": true}, after.Data())
			}
		})
	}
}

func TestGoogleOperatorFirestoreChallengeMalformedAndWrongTargetsDoNotMutate(t *testing.T) {
	store := googleChallengeEmulator(t)
	for _, project := range []string{"", "demo-other", "production-space", "demo-space/invalid"} {
		wrong, err := NewFirestoreGoogleOperatorChallenges(store.client, project)
		require.Nil(t, wrong)
		require.ErrorIs(t, err, ErrOperatorDenied)
	}
	for _, field := range []string{"extra", "nonceHash", "binding", "issuedAt", "expiresAt", "consumed", "schemaVersion"} {
		t.Run(field, func(t *testing.T) {
			record := googleChallengeRecord(t)
			require.NoError(t, store.Put(context.Background(), record))
			ref := store.client.Collection(googleOperatorChallenges).Doc(record.NonceHash)
			_, err := ref.Update(context.Background(), []firestore.Update{{Path: field, Value: "synthetic-invalid"}})
			require.NoError(t, err)
			before, err := ref.Get(context.Background())
			require.NoError(t, err)
			_, err = store.Consume(context.Background(), record.NonceHash, record.Binding, func() time.Time { return record.IssuedAt })
			require.ErrorIs(t, err, ErrOperatorDenied)
			require.ErrorIs(t, store.FirestoreGoogleOperatorChallenges.Put(context.Background(), record), ErrOperatorDenied)
			after, err := ref.Get(context.Background())
			require.NoError(t, err)
			require.Equal(t, before.Data(), after.Data())
			require.Equal(t, before.UpdateTime, after.UpdateTime)
		})
	}
}

func TestGoogleOperatorFirestoreChallengeSignedAuthorityRemainsBoundAndPrivate(t *testing.T) {
	store := googleChallengeEmulator(t)
	f := newGoogleOperatorFixture(t)
	f.authority.Challenges = store
	challenge, err := f.authority.Begin(context.Background(), f.intent)
	require.NoError(t, err)
	f.token.Set(f.signed(t, f.claims(challenge)))
	wrong := f.intent
	wrong.ProofRef = digest("swapped-proof")
	_, err = f.authority.Authorize(context.Background(), wrong)
	require.ErrorIs(t, err, ErrOperatorDenied)
	doc, err := store.client.Collection(googleOperatorChallenges).Doc(digest(challenge.Nonce)).Get(context.Background())
	require.NoError(t, err)
	require.Len(t, doc.Data(), 6)
	require.Equal(t, digest(challenge.Nonce), doc.Data()["nonceHash"])
	require.Equal(t, f.authority.binding(f.intent), doc.Data()["binding"])
	raw, err := json.Marshal(doc.Data())
	require.NoError(t, err)
	for _, private := range []string{challenge.Nonce, challenge.State, f.authority.AllowedSubject, f.intent.RequestRef, f.intent.ProofRef, f.request.Body, "synthetic-operator@example.invalid"} {
		require.NotContains(t, string(raw), private)
	}
	for _, field := range []string{"token", "nonce", "state", "sub", "email", "proofRef", "requestRef", "body"} {
		require.NotContains(t, doc.Data(), field)
	}
	// A separately constructed client represents the next process invocation.
	f.authority.Challenges = googleChallengeEmulator(t)
	identity, err := f.authority.Authorize(context.Background(), f.intent)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(identity.Subject, "google-operator-"))
	_, err = f.authority.Authorize(context.Background(), f.intent)
	require.ErrorIs(t, err, ErrOperatorDenied)
	_, err = f.authority.VerifyEvidence(context.Background(), identity, CompletionOperation{})
	require.ErrorIs(t, err, ErrOperatorDenied, "identity proof alone cannot authorize completion")
}

func TestGoogleOperatorFirestoreChallengeClientRulesDeny(t *testing.T) {
	store := googleChallengeEmulator(t)
	record := googleChallengeRecord(t)
	require.NoError(t, store.Put(context.Background(), record))
	encode := base64.RawURLEncoding.EncodeToString
	token := encode([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + encode([]byte(`{"iss":"https://securetoken.google.com/demo-youtube-study-space-ci","aud":"demo-youtube-study-space-ci","sub":"synthetic-rules-user","user_id":"synthetic-rules-user","iat":1780000000,"exp":2090000000,"firebase":{"sign_in_provider":"custom"}}`)) + "."
	endpoint := "http://" + os.Getenv("FIRESTORE_EMULATOR_HOST") + "/v1/projects/demo-youtube-study-space-ci/databases/(default)/documents/" + googleOperatorChallenges + "/" + record.NonceHash
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, authorization := range []string{"", "Bearer " + token} {
		for _, method := range []string{http.MethodGet, http.MethodPatch} {
			r, err := http.NewRequestWithContext(context.Background(), method, endpoint, strings.NewReader(`{"fields":{"consumed":{"booleanValue":true}}}`))
			require.NoError(t, err)
			r.Header.Set("Content-Type", "application/json")
			if authorization != "" {
				r.Header.Set("Authorization", authorization)
			}
			response, err := client.Do(r)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusForbidden, response.StatusCode)
		}
	}
}
