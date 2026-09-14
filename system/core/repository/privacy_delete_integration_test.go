//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	"app.modules/internal/integrationtest"
	"app.modules/internal/privacyops"
)

func TestPrivacyDeleteFirestoreUserData_DeletesOnlyTargetUser(t *testing.T) {
	integrationtest.ResetFirestore(t)
	controller := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	client := controller.FirestoreClient()

	const (
		targetChannelID = "privacy-delete-target"
		otherChannelID  = "privacy-delete-other"
		targetFirebase  = "firebase-target"
		otherFirebase   = "firebase-other"
	)

	seedDocument(t, ctx, client.Collection(repository.USERS).Doc(targetChannelID), map[string]any{"marker": "target"})
	seedDocument(t, ctx, client.Collection(repository.USERS).Doc(otherChannelID), map[string]any{"marker": "other"})

	userIDCollections := []string{
		repository.SEATS,
		repository.MemberSeats,
		repository.UserActivities,
		repository.WorkSegments,
		repository.OrderHistory,
		repository.SeatLimitsBlackList,
		repository.SeatLimitsWhiteList,
		repository.MemberSeatLimitsBlackList,
		repository.MemberSeatLimitsWhiteList,
	}
	for index, collection := range userIDCollections {
		seedDocument(
			t,
			ctx,
			client.Collection(collection).Doc(fmt.Sprintf("target-%02d", index)),
			map[string]any{repository.UserIDDocProperty: targetChannelID, "marker": "target"},
		)
		seedDocument(
			t,
			ctx,
			client.Collection(collection).Doc(fmt.Sprintf("other-%02d", index)),
			map[string]any{repository.UserIDDocProperty: otherChannelID, "marker": "other"},
		)
	}

	seedDocument(
		t,
		ctx,
		client.Collection(repository.LiveChatHistory).Doc("target-message"),
		map[string]any{"author-channel-id": targetChannelID, "message-text": "target message"},
	)
	seedDocument(
		t,
		ctx,
		client.Collection(repository.LiveChatHistory).Doc("other-message"),
		map[string]any{"author-channel-id": otherChannelID, "message-text": "other message"},
	)

	seedDocument(
		t,
		ctx,
		client.Collection("mypage-youtube-channel-owners").Doc(targetChannelID),
		map[string]any{"firebase-uid": targetFirebase},
	)
	seedDocument(
		t,
		ctx,
		client.Collection("mypage-users").Doc(targetFirebase),
		map[string]any{"youtube-channel-id": targetChannelID},
	)
	seedDocument(
		t,
		ctx,
		client.Collection("mypage-youtube-channel-owners").Doc(otherChannelID),
		map[string]any{"firebase-uid": otherFirebase},
	)
	seedDocument(
		t,
		ctx,
		client.Collection("mypage-users").Doc(otherFirebase),
		map[string]any{"youtube-channel-id": otherChannelID},
	)

	result, err := privacyops.DeleteFirestoreUserData(ctx, client, targetChannelID)
	require.NoError(t, err)
	assert.Equal(t, targetFirebase, result.FirebaseUID)
	assert.Equal(t, 2, result.MyPage)

	assertDocumentMissing(t, ctx, client.Collection(repository.USERS).Doc(targetChannelID))
	assertDocumentExists(t, ctx, client.Collection(repository.USERS).Doc(otherChannelID))

	for _, collection := range userIDCollections {
		assertQueryCount(t, ctx, client, collection, repository.UserIDDocProperty, targetChannelID, 0)
		assertQueryCount(t, ctx, client, collection, repository.UserIDDocProperty, otherChannelID, 1)
	}
	assertQueryCount(t, ctx, client, repository.LiveChatHistory, "author-channel-id", targetChannelID, 0)
	assertQueryCount(t, ctx, client, repository.LiveChatHistory, "author-channel-id", otherChannelID, 1)

	assertDocumentMissing(t, ctx, client.Collection("mypage-youtube-channel-owners").Doc(targetChannelID))
	assertDocumentMissing(t, ctx, client.Collection("mypage-users").Doc(targetFirebase))
	assertDocumentExists(t, ctx, client.Collection("mypage-youtube-channel-owners").Doc(otherChannelID))
	assertDocumentExists(t, ctx, client.Collection("mypage-users").Doc(otherFirebase))

	_, err = privacyops.DeleteFirestoreUserData(ctx, client, targetChannelID)
	require.NoError(t, err, "privacy deletion must be safe to retry")
}

func seedDocument(
	t *testing.T,
	ctx context.Context,
	ref *firestore.DocumentRef,
	data map[string]any,
) {
	t.Helper()
	_, err := ref.Set(ctx, data)
	require.NoError(t, err)
}

func assertDocumentMissing(t *testing.T, ctx context.Context, ref *firestore.DocumentRef) {
	t.Helper()
	_, err := ref.Get(ctx)
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func assertDocumentExists(t *testing.T, ctx context.Context, ref *firestore.DocumentRef) {
	t.Helper()
	_, err := ref.Get(ctx)
	require.NoError(t, err)
}

func assertQueryCount(
	t *testing.T,
	ctx context.Context,
	client repository.DBClient,
	collection string,
	field string,
	value string,
	want int,
) {
	t.Helper()
	docs, err := client.Collection(collection).Where(field, "==", value).Documents(ctx).GetAll()
	require.NoError(t, err)
	assert.Len(t, docs, want)
}
