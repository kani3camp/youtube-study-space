package privacyops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
)

const (
	mypageUsersCollection         = "mypage-users"
	mypageChannelOwnersCollection = "mypage-youtube-channel-owners"
	authorChannelIDField          = "author-channel-id"
	firebaseUIDField              = "firebase-uid"
)

type FirestoreInventory struct {
	Collections map[string]int         `json:"collections"`
	MyPage      MyPageMappingInventory `json:"mypage"`
}

type MyPageMappingInventory struct {
	ChannelOwnerDocument  int    `json:"channel_owner_document"`
	LinkedAccountDocument int    `json:"linked_account_document"`
	FirebaseUID           string `json:"firebase_uid,omitempty"`
}

type collectionLookup struct {
	collection string
	field      string
}

var userIDCollectionLookups = []collectionLookup{
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

// InspectFirestore returns counts of documents directly attributable to a
// YouTube channel ID. It is deliberately read-only and is intended to be run
// before any user-data deletion operation.
func InspectFirestore(
	ctx context.Context,
	client repository.DBClient,
	youtubeChannelID string,
) (FirestoreInventory, error) {
	youtubeChannelID = strings.TrimSpace(youtubeChannelID)
	if youtubeChannelID == "" {
		return FirestoreInventory{}, errors.New("youtube channel id is empty")
	}
	if client == nil {
		return FirestoreInventory{}, errors.New("firestore client is nil")
	}

	inventory := FirestoreInventory{
		Collections: make(map[string]int, len(userIDCollectionLookups)+1),
	}

	userExists, err := documentExists(ctx, client.Collection(repository.USERS).Doc(youtubeChannelID))
	if err != nil {
		return FirestoreInventory{}, fmt.Errorf("inspect users document: %w", err)
	}
	if userExists {
		inventory.Collections[repository.USERS] = 1
	} else {
		inventory.Collections[repository.USERS] = 0
	}

	for _, lookup := range userIDCollectionLookups {
		count, err := countDocumentsByField(ctx, client, lookup.collection, lookup.field, youtubeChannelID)
		if err != nil {
			return FirestoreInventory{}, fmt.Errorf("inspect collection %s: %w", lookup.collection, err)
		}
		inventory.Collections[lookup.collection] = count
	}

	mypageInventory, err := inspectMyPageMappings(ctx, client, youtubeChannelID)
	if err != nil {
		return FirestoreInventory{}, fmt.Errorf("inspect mypage mappings: %w", err)
	}
	inventory.MyPage = mypageInventory

	return inventory, nil
}

func countDocumentsByField(
	ctx context.Context,
	client repository.DBClient,
	collection string,
	field string,
	value string,
) (int, error) {
	docs, err := client.Collection(collection).
		Where(field, "==", value).
		Select().
		Documents(ctx).
		GetAll()
	if err != nil {
		return 0, err
	}
	return len(docs), nil
}

func inspectMyPageMappings(
	ctx context.Context,
	client repository.DBClient,
	youtubeChannelID string,
) (MyPageMappingInventory, error) {
	ownerRef := client.Collection(mypageChannelOwnersCollection).Doc(youtubeChannelID)
	ownerSnapshot, err := ownerRef.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return MyPageMappingInventory{}, nil
	}
	if err != nil {
		return MyPageMappingInventory{}, err
	}

	inventory := MyPageMappingInventory{ChannelOwnerDocument: 1}
	firebaseUID, _ := ownerSnapshot.Data()[firebaseUIDField].(string)
	firebaseUID = strings.TrimSpace(firebaseUID)
	if firebaseUID == "" {
		return inventory, nil
	}
	inventory.FirebaseUID = firebaseUID

	accountExists, err := documentExists(ctx, client.Collection(mypageUsersCollection).Doc(firebaseUID))
	if err != nil {
		return MyPageMappingInventory{}, err
	}
	if accountExists {
		inventory.LinkedAccountDocument = 1
	}

	return inventory, nil
}

func documentExists(ctx context.Context, ref *firestore.DocumentRef) (bool, error) {
	_, err := ref.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
