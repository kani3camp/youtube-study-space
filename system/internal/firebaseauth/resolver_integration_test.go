//go:build integration

package firebaseauth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"app.modules/core/mypage"
	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestResolver_LinkYouTubeAccount_ConcurrentRequestsKeepChannelUnique(t *testing.T) {
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is required")
	}

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "youtube-study-space-test", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	accountsCollection := "mypage-users-integration-" + suffix
	ownersCollection := accountsCollection + "-youtube-channel-owners"
	resolver := &Resolver{
		firestoreClient:          client,
		linkedAccountsCollection: accountsCollection,
		channelOwnersCollection:  ownersCollection,
		nowFunc:                  func() time.Time { return time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) },
	}
	t.Cleanup(func() {
		deleteTestCollection(t, client, accountsCollection)
		deleteTestCollection(t, client, ownersCollection)
	})

	viewer := mypage.Viewer{
		YouTubeChannelID: "UC-concurrent-link-test",
		DisplayName:      "concurrent test",
	}
	firebaseUIDs := []string{"firebase-user-a", "firebase-user-b"}
	start := make(chan struct{})
	errs := make([]error, len(firebaseUIDs))
	var wg sync.WaitGroup
	for i, firebaseUID := range firebaseUIDs {
		wg.Add(1)
		go func(index int, uid string) {
			defer wg.Done()
			<-start
			errs[index] = resolver.LinkYouTubeAccount(ctx, uid, viewer)
		}(i, firebaseUID)
	}
	close(start)
	wg.Wait()

	var successCount int
	var conflictCount int
	for _, linkErr := range errs {
		switch {
		case linkErr == nil:
			successCount++
		case errors.Is(linkErr, mypage.ErrYouTubeChannelAlreadyLinked):
			conflictCount++
		default:
			t.Fatalf("unexpected link error: %v", linkErr)
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, conflictCount)

	ownerSnapshot, err := client.Collection(ownersCollection).Doc(viewer.YouTubeChannelID).Get(ctx)
	require.NoError(t, err)
	var owner YouTubeChannelOwnerDoc
	require.NoError(t, ownerSnapshot.DataTo(&owner))
	assert.Contains(t, firebaseUIDs, owner.FirebaseUID)

	linkedCount := 0
	for _, firebaseUID := range firebaseUIDs {
		_, err := client.Collection(accountsCollection).Doc(firebaseUID).Get(ctx)
		switch status.Code(err) {
		case codes.OK:
			linkedCount++
		case codes.NotFound:
		default:
			require.NoError(t, err)
		}
	}
	assert.Equal(t, 1, linkedCount)
}

func deleteTestCollection(t *testing.T, client *firestore.Client, collection string) {
	t.Helper()

	docs, err := client.Collection(collection).Documents(context.Background()).GetAll()
	require.NoError(t, err)
	for _, doc := range docs {
		_, err := doc.Ref.Delete(context.Background())
		require.NoError(t, err)
	}
}
