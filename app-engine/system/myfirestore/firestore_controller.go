package myfirestore

import (
	"app.modules/system"
	"cloud.google.com/go/firestore"
	"context"
	"google.golang.org/api/option"
	"time"
)

type FirestoreController struct {
	FirestoreClient *firestore.Client
}

func NewFirestoreController(ctx context.Context, projectId string, clientOption option.ClientOption) (*FirestoreController, error) {
	var client *firestore.Client
	var err error
	client, err = firestore.NewClient(ctx, system.ProjectId, clientOption)
	if err != nil {
		return nil, err
	}

	return &FirestoreController{
		FirestoreClient: client,
	}, nil
}

func (controller *FirestoreController) RetrieveYoutubeLiveInfo(ctx context.Context) (YoutubeLiveDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(YouTubeLiveDocName).Get(ctx)
	if err != nil {
		return YoutubeLiveDoc{}, err
	}
	var youtubeLiveData YoutubeLiveDoc
	err = doc.DataTo(&youtubeLiveData)
	if err != nil {
		return YoutubeLiveDoc{}, err
	}
	return youtubeLiveData, nil
}

func (controller *FirestoreController) RetrieveLiveChatId(ctx context.Context) (string, error) {
	youtubeLiveDoc, err := controller.RetrieveYoutubeLiveInfo(ctx)
	if err != nil {
		return "", err
	}
	return youtubeLiveDoc.LiveChatId, nil
}

func (controller *FirestoreController) RetrieveNextPageToken(ctx context.Context) (string, error) {
	youtubeLiveDoc, err := controller.RetrieveYoutubeLiveInfo(ctx)
	if err != nil {
		return "", err
	}
	return youtubeLiveDoc.NextPageToken, nil
}

func (controller *FirestoreController) SaveNextPageToken(nextPageToken string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(YouTubeLiveDocName).Set(ctx, map[string]interface{}{
		NextPageTokenFirestore: nextPageToken,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveDefaultRoom(ctx context.Context) (DefaultRoomDoc, error) {
	var defaultRoomData DefaultRoomDoc
	doc, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Get(ctx)
	if err != nil {
		return DefaultRoomDoc{}, err
	}
	err = doc.DataTo(&defaultRoomData)
	if err != nil {
		return DefaultRoomDoc{}, err
	}
	return defaultRoomData, nil
}

func (controller *FirestoreController) RetrieveNoSeatRoom(ctx context.Context) (NoSeatRoomDoc, error) {
	var noSeatRoomData NoSeatRoomDoc
	doc, err := controller.FirestoreClient.Collection(ROOMS).Doc(NoSeatRoomDocName).Get(ctx)
	if err != nil {
		return NoSeatRoomDoc{}, err
	}
	err = doc.DataTo(&noSeatRoomData)
	if err != nil {
		return NoSeatRoomDoc{}, err
	}
	return noSeatRoomData, nil
}

func (controller *FirestoreController) SetUserInDefaultRoom(seatId int, userId string, ctx context.Context) error {
	seat := Seat{
		SeatId: seatId,
		UserId: userId,
	}
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Set(ctx, map[string]interface{}{
		SeatsFirestore: firestore.ArrayUnion(seat),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetUserInNoSeatRoom(userId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(NoSeatRoomDocName).Set(ctx, map[string]interface{}{
		UsersFirestore: firestore.ArrayUnion(userId),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetLastEnteredDate(userId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, map[string]interface{}{
		LastEnteredFirestore: time.Now(),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}





