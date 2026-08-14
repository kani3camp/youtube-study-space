package privacyops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
)

type FirestoreDeleteResult struct {
	Collections map[string]int `json:"collections"`
	MyPage      int            `json:"mypage_mapping_documents"`
	FirebaseUID string         `json:"firebase_uid,omitempty"`
}

var deleteLookups = []collectionLookup{
	// Active seats are removed first so the seat becomes available even if a
	// later historical-data deletion fails and the operation has to be retried.
	{collection: repository.SEATS, field: repository.UserIDDocProperty},
	{collection: repository.MemberSeats, field: repository.UserIDDocProperty},
	{collection: repository.UserActivities, field: repository.UserIDDocProperty},
	{collection: repository.WorkSegments, field: repository.UserIDDocProperty},
	{collection: repository.OrderHistory, field: repository.UserIDDocProperty},
	{collection: repository.SeatLimitsBlackList, field: repository.UserIDDocProperty},
	{collection: repository.SeatLimitsWhiteList, field: repository.UserIDDocProperty},
	{collection: repository.MemberSeatLimitsBlackList, field: repository.UserIDDocProperty},
	{collection: repository.MemberSeatLimitsWhiteList, field: repository.UserIDDocProperty},
	{collection: repository.LiveChatHistory, field: authorChannelIDField},
}

// DeleteFirestoreUserData deletes primary Firestore documents attributable to a
// YouTube channel ID. The operation is idempotent and intentionally excludes
// GCS export snapshots and Firebase Authentication user records.
//
// Seat documents are deleted directly instead of executing the ordinary !out
// flow. A privacy deletion must not create a new exit activity/work segment or
// preserve the user's accumulated Study Space state merely to close the seat.
func DeleteFirestoreUserData(
	ctx context.Context,
	client repository.DBClient,
	youtubeChannelID string,
) (FirestoreDeleteResult, error) {
	youtubeChannelID = strings.TrimSpace(youtubeChannelID)
	if youtubeChannelID == "" {
		return FirestoreDeleteResult{}, errors.New("youtube channel id is empty")
	}
	if client == nil {
		return FirestoreDeleteResult{}, errors.New("firestore client is nil")
	}

	result := FirestoreDeleteResult{
		Collections: make(map[string]int, len(deleteLookups)+1),
	}

	for _, lookup := range deleteLookups {
		deleted, err := deleteDocumentsByField(
			ctx,
			client,
			lookup.collection,
			lookup.field,
			youtubeChannelID,
		)
		if err != nil {
			return result, fmt.Errorf("delete collection %s: %w", lookup.collection, err)
		}
		result.Collections[lookup.collection] = deleted
	}

	mypageDeleted, firebaseUID, err := deleteMyPageMappings(ctx, client, youtubeChannelID)
	if err != nil {
		return result, fmt.Errorf("delete mypage mappings: %w", err)
	}
	result.MyPage = mypageDeleted
	result.FirebaseUID = firebaseUID

	userDeleted, err := deleteDocumentIfExists(ctx, client.Collection(repository.USERS).Doc(youtubeChannelID))
	if err != nil {
		return result, fmt.Errorf("delete users document: %w", err)
	}
	if userDeleted {
		result.Collections[repository.USERS] = 1
	} else {
		result.Collections[repository.USERS] = 0
	}

	return result, nil
}

func deleteDocumentsByField(
	ctx context.Context,
	client repository.DBClient,
	collection string,
	field string,
	value string,
) (int, error) {
	iter := client.Collection(collection).Where(field, "==", value).Documents(ctx)
	defer iter.Stop()

	deleted := 0
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			return deleted, nil
		}
		if err != nil {
			return deleted, fmt.Errorf("iterate matching documents: %w", err)
		}
		if _, err := doc.Ref.Delete(ctx); err != nil {
			return deleted, fmt.Errorf("delete document %q: %w", doc.Ref.Path, err)
		}
		deleted++
	}
}

func deleteMyPageMappings(
	ctx context.Context,
	client repository.DBClient,
	youtubeChannelID string,
) (int, string, error) {
	ownerRef := client.Collection(mypageChannelOwnersCollection).Doc(youtubeChannelID)
	ownerSnapshot, err := ownerRef.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", fmt.Errorf("get mypage channel owner: %w", err)
	}

	rawFirebaseUID, ok := ownerSnapshot.Data()[firebaseUIDField]
	if !ok {
		return 0, "", errors.New("mypage channel owner is missing firebase uid")
	}
	firebaseUID, ok := rawFirebaseUID.(string)
	if !ok {
		return 0, "", fmt.Errorf("mypage channel owner firebase uid has unexpected type %T", rawFirebaseUID)
	}
	firebaseUID = strings.TrimSpace(firebaseUID)
	if firebaseUID == "" {
		return 0, "", errors.New("mypage channel owner firebase uid is empty")
	}

	deleted := 0
	accountRef := client.Collection(mypageUsersCollection).Doc(firebaseUID)
	accountDeleted, err := deleteDocumentIfExists(ctx, accountRef)
	if err != nil {
		return deleted, firebaseUID, fmt.Errorf("delete mypage linked account: %w", err)
	}
	if accountDeleted {
		deleted++
	}

	if _, err := ownerRef.Delete(ctx); err != nil {
		return deleted, firebaseUID, fmt.Errorf("delete mypage channel owner: %w", err)
	}
	deleted++

	return deleted, firebaseUID, nil
}

func deleteDocumentIfExists(ctx context.Context, ref interface {
	Get(context.Context) (*firestore.DocumentSnapshot, error)
	Delete(context.Context, ...firestore.Precondition) (*firestore.WriteResult, error)
}) (bool, error) {
	_, err := ref.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get document before delete: %w", err)
	}
	if _, err := ref.Delete(ctx); err != nil {
		return false, fmt.Errorf("delete document: %w", err)
	}
	return true, nil
}
