package myfirestore

import (
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
	client, err = firestore.NewClient(ctx, projectId, clientOption)
	if err != nil {
		return nil, err
	}

	return &FirestoreController{
		FirestoreClient: client,
	}, nil
}

func (controller *FirestoreController) RetrieveYoutubeLiveConfig(ctx context.Context) (YoutubeLiveConfigDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(YouTubeLiveConfigDocName).Get(ctx)
	if err != nil {
		return YoutubeLiveConfigDoc{}, err
	}
	var youtubeLiveData YoutubeLiveConfigDoc
	err = doc.DataTo(&youtubeLiveData)
	if err != nil {
		return YoutubeLiveConfigDoc{}, err
	}
	return youtubeLiveData, nil
}

func (controller *FirestoreController) RetrieveYoutubeBotCredentialConfig(ctx context.Context) (YoutubeCredentialDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(YoutubeBotCredentialDocName).Get(ctx)
	if err != nil {
		return YoutubeCredentialDoc{}, err
	}
	var youtubeBotCredential YoutubeCredentialDoc
	err = doc.DataTo(&youtubeBotCredential)
	if err != nil {
		return YoutubeCredentialDoc{}, err
	}
	return youtubeBotCredential, nil
}

func (controller *FirestoreController) RetrieveYoutubeChannelCredentialConfig(ctx context.Context) (YoutubeCredentialDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(YoutubeChannelCredentialDocName).Get(ctx)
	if err != nil {
		return YoutubeCredentialDoc{}, err
	}
	var youtubeChannelCredential YoutubeCredentialDoc
	err = doc.DataTo(&youtubeChannelCredential)
	if err != nil {
		return YoutubeCredentialDoc{}, err
	}
	return youtubeChannelCredential, nil
}

func (controller *FirestoreController) RetrieveLineBotConfig(ctx context.Context) (LineBotConfigDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(LineBotConfigDocName).Get(ctx)
	if err != nil {
		return LineBotConfigDoc{}, err
	}
	var lineBotConfigData LineBotConfigDoc
	err = doc.DataTo(&lineBotConfigData)
	if err != nil {
		return LineBotConfigDoc{}, err
	}
	return lineBotConfigData, nil
}

func (controller *FirestoreController) RetrieveSystemConstantsConfig(ctx context.Context) (ConstantsConfigDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName).Get(ctx)
	if err != nil {
		return ConstantsConfigDoc{}, err
	}
	var constantsConfig ConstantsConfigDoc
	err = doc.DataTo(&constantsConfig)
	if err != nil {
		return ConstantsConfigDoc{}, err
	}
	return constantsConfig, nil
}

func (controller *FirestoreController) RetrieveLiveChatId(ctx context.Context) (string, error) {
	youtubeLiveDoc, err := controller.RetrieveYoutubeLiveConfig(ctx)
	if err != nil {
		return "", err
	}
	return youtubeLiveDoc.LiveChatId, nil
}

func (controller *FirestoreController) RetrieveNextPageToken(ctx context.Context) (string, error) {
	youtubeLiveDoc, err := controller.RetrieveYoutubeLiveConfig(ctx)
	if err != nil {
		return "", err
	}
	return youtubeLiveDoc.NextPageToken, nil
}

func (controller *FirestoreController) SaveNextPageToken(nextPageToken string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(YouTubeLiveConfigDocName).Set(ctx, map[string]interface{}{
		NextPageTokenFirestore: nextPageToken,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveDefaultRoom(ctx context.Context) (DefaultRoomDoc, error) {
	defaultRoomData := NewDefaultRoomDoc()
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
	noSeatRoomData := NewNoSeatRoomDoc()
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

func (controller *FirestoreController) SetSeatInDefaultRoom(seatId int, workName string, exitDate time.Time, userId string, userDisplayName string, ctx context.Context) (Seat, error) {
	seat := Seat{
		SeatId: seatId,
		UserId: userId,
		UserDisplayName: userDisplayName,
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

func (controller *FirestoreController) SetSeatInNoSeatRoom(workName string, exitDate time.Time, userId string, userDisplayName string, ctx context.Context) (Seat, error) {
	seat := Seat{
		UserId: userId,
		UserDisplayName: userDisplayName,
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
	//_seat := Seat{
	//	SeatId:   seat.SeatId,
	//	UserId:   seat.UserId,
	//	WorkName: seat.WorkName,
	//	Until:    seat.Until,
	//}
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(NoSeatRoomDocName).Set(ctx, map[string]interface{}{
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
	roomLayoutData := NewRoomLayoutDoc()
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
	_, _, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Collection(HISTORY).Add(ctx, history)
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
	userData := UserDoc{}
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

func (controller *FirestoreController) AddRoomLayoutHistory(data interface{}, ctx context.Context) error {
	_, _, err := controller.FirestoreClient.Collection(CONFIG).Doc(DefaultRoomLayoutDocName).Collection(HISTORY).Add(ctx, data)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SaveRoomLayout(roomLayoutData RoomLayoutDoc, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(DefaultRoomLayoutDocName).Set(ctx, roomLayoutData)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SaveLiveChatId(liveChatId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(YouTubeLiveConfigDocName).Set(ctx, map[string]interface{}{
		LiveChatIdFirestore: liveChatId,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) InitializeUser(userId string, userData UserDoc, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, userData)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
	return controller.FirestoreClient.Collection(USERS).DocumentRefs(ctx).GetAll()
}

func (controller *FirestoreController) ResetDailyTotalStudyTime(userRef *firestore.DocumentRef, ctx context.Context) error {
	_, err := userRef.Set(ctx, map[string]interface{}{
		DailyTotalStudySecFirestore: 0,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetLastResetDailyTotalStudyTime(date time.Time, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName).Set(ctx, map[string]interface{}{
		LastResetDailyTotalStudySecFirestore: date,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}
