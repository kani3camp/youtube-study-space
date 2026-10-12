//go:build integration

package serviceaccess

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"app.modules/internal/integrationtest"
)

func testStore(t *testing.T) (*FirestoreStore, string) {
	t.Helper()
	integrationtest.RequireFirestoreEmulator(t)
	client, err := firestore.NewClient(context.Background(), "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	id := "UCserviceaccess000000001"
	ref := client.Collection(Collection).Doc(id)
	_, err = ref.Delete(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _, err := ref.Delete(context.Background()); require.NoError(t, err) })
	return &FirestoreStore{Client: client}, id
}

func TestFirestoreReasonsConcurrentChangesAndMalformedRecord(t *testing.T) {
	store, id := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	value, err := store.Read(ctx, id)
	require.NoError(t, err)
	require.False(t, value.Exists)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, reason := range []Reason{Moderation, PrivacyDeletion} {
		wg.Go(func() { _, err := store.Change(ctx, id, sampleChange(reason), now); results <- err })
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	value, err = store.Read(ctx, id)
	require.NoError(t, err)
	require.True(t, value.Control.Moderation.Active)
	require.True(t, value.Control.PrivacyDeletion.Active)
	require.Equal(t, int64(2), value.Control.Generation)
	repeated, err := store.Change(ctx, id, sampleChange(Moderation), now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, value.Checkpoint(), repeated.Checkpoint())
	value, err = store.Change(ctx, id, Change{Reason: Moderation}, now.Add(time.Second))
	require.NoError(t, err)
	require.ErrorIs(t, value.Allowed(), ErrDeletion)
	require.Equal(t, int64(3), value.Control.Generation)
	_, err = store.Change(ctx, id, Change{Reason: "invalid"}, now)
	require.ErrorIs(t, err, ErrInvalidChange)
	_, err = store.Client.Collection(Collection).Doc(id).Update(ctx, []firestore.Update{{Path: "displayName", Value: "forbidden synthetic personal field"}})
	require.NoError(t, err)
	_, err = store.Read(ctx, id)
	require.ErrorIs(t, err, ErrUnavailable)
	_, err = store.Change(ctx, id, Change{Reason: PrivacyDeletion}, now.Add(2*time.Second))
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestFirestoreClientRulesDenyControlReadAndWrite(t *testing.T) {
	store, id := testStore(t)
	ctx := context.Background()
	_, err := store.Change(ctx, id, sampleChange(Moderation), time.Now().UTC())
	require.NoError(t, err)
	endpoint := "http://" + os.Getenv("FIRESTORE_EMULATOR_HOST") + "/v1/projects/demo-youtube-study-space-ci/databases/(default)/documents/" + Collection + "/" + id
	// Firestore Emulator accepts unsigned Firebase identity tokens; these have
	// no real credentials and exercise rules independently of the Admin SDK.
	encode := base64.RawURLEncoding.EncodeToString
	token := encode([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + encode([]byte(`{"iss":"https://securetoken.google.com/demo-youtube-study-space-ci","aud":"demo-youtube-study-space-ci","sub":"synthetic-rules-user","user_id":"synthetic-rules-user","iat":1780000000,"exp":2090000000,"firebase":{"sign_in_provider":"custom"}}`)) + "."
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, auth := range []string{"", "Bearer " + token} {
		for _, method := range []string{http.MethodGet, http.MethodPatch} {
			req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(`{"fields":{"generation":{"integerValue":"99"}}}`))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if auth != "" {
				req.Header.Set("Authorization", auth)
			}
			response, err := client.Do(req)
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, response.Body)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusForbidden, response.StatusCode, "client rules must deny control read/write")
		}
	}
	value, err := store.Read(ctx, id)
	require.NoError(t, err)
	require.Equal(t, int64(1), value.Control.Generation)
}
