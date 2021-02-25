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

func (controller *FirestoreController) SetSeatInDefaultRoom(seatId int, workName string, exitDate time.Time, userId string, ctx context.Context) (Seat, error) {
	seat := Seat{
		SeatId: seatId,
		UserId: userId,
		WorkName: workName,
		Until: exitDate,
	}
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Set(ctx, map[string]interface{}{
		SeatsFirestore: firestore.ArrayUnion(seat),
	}, firestore.MergeAll)
	if err != nil {
		return Seat{}, err
	}
	return seat, nil
}

func (controller *FirestoreController) SetSeatInNoSeatRoom(workName string, exitDate time.Time, userId string, ctx context.Context) (Seat, error) {
	seat := Seat{
		UserId: userId,
		WorkName: workName,
		Until: exitDate,
	}
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(NoSeatRoomDocName).Set(ctx, map[string]interface{}{
		SeatsFirestore: firestore.ArrayUnion(seat),
	}, firestore.MergeAll)
	if err != nil {
		return Seat{}, err
	}
	return seat, nil
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

func (controller *FirestoreController) SetLastExitedDate(userId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, map[string]interface{}{
		LastExitedFirestore: time.Now(),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UnSetSeatInDefaultRoom(seat Seat, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Set(ctx, map[string]interface{}{
		SeatsFirestore: firestore.ArrayRemove(seat),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UnSetSeatInNoSeatRoom(seat Seat, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Set(ctx, map[string]interface{}{
		SeatsFirestore: firestore.ArrayRemove(seat),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveDefaultRoomLayout(ctx context.Context) (RoomLayoutDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(DefaultRoomLayoutDocName).Get(ctx)
	if err != nil {
		return RoomLayoutDoc{}, err
	}
	var roomLayoutData RoomLayoutDoc
	err = doc.DataTo(&roomLayoutData)
	if err != nil {
		return RoomLayoutDoc{}, err
	}
	return roomLayoutData, nil
}

func (controller *FirestoreController) AddUserHistory(userId string, action string, details interface{}, ctx context.Context) error {
	history := UserHistoryDoc{
		Action:  action,
		Date:    time.Now(),
		Details: details,
	}
	_, _, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Collection(UserHistory).Add(ctx, history)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveUser(userId string, ctx context.Context) (UserDoc, error) {
	doc, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Get(ctx)
	if err != nil {
		return UserDoc{}, err
	}
	var userData UserDoc
	err = doc.DataTo(&userData)
	if err != nil {
		return UserDoc{}, err
	}
	return userData, nil
}

func (controller *FirestoreController) UpdateTotalTime(userId string, newTotalTimeSec int, newDailyTotalTimeSec int, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, map[string]interface{}{
		DailyTotalStudySecFirestore: newDailyTotalTimeSec,
		TotalStudySecFirestore: newTotalTimeSec,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

