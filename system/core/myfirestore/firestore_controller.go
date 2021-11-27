package myfirestore

import (
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"context"
	"google.golang.org/api/option"
	"time"
)

type FirestoreController struct {
	FirestoreClient *firestore.Client
}

func NewFirestoreController(ctx context.Context, clientOption option.ClientOption) (*FirestoreController, error) {
	var client *firestore.Client
	var err error
	client, err = firestore.NewClient(ctx, firestore.DetectProjectID, clientOption)
	if err != nil {
		return nil, err
	}

	return &FirestoreController{
		FirestoreClient: client,
	}, nil
}

func (controller *FirestoreController) RetrieveCredentialsConfig(ctx context.Context) (CredentialsConfigDoc, error) {
	doc, err := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName).Get(ctx)
	if err != nil {
		return CredentialsConfigDoc{}, err
	}
	var credentialsData CredentialsConfigDoc
	err = doc.DataTo(&credentialsData)
	if err != nil {
		return CredentialsConfigDoc{}, err
	}
	return credentialsData, nil
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
	credentialsDoc, err := controller.RetrieveCredentialsConfig(ctx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatId, nil
}

func (controller *FirestoreController) RetrieveNextPageToken(ctx context.Context) (string, error) {
	credentialsDoc, err := controller.RetrieveCredentialsConfig(ctx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatNextPageToken, nil
}

func (controller *FirestoreController) SaveNextPageToken(nextPageToken string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName).Set(ctx, map[string]interface{}{
		NextPageTokenFirestore: nextPageToken,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveRoom(ctx context.Context) (RoomDoc, error) {
	roomData := NewRoomDoc()
	doc, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Get(ctx)
	if err != nil {
		return RoomDoc{}, err
	}
	err = doc.DataTo(&roomData)
	if err != nil {
		return RoomDoc{}, err
	}
	return roomData, nil
}

func (controller *FirestoreController) SetSeat(seatId int, workName string, enterDate time.Time, exitDate time.Time, seatColorCode string, userId string, userDisplayName string, ctx context.Context) (Seat, error) {
	seat := Seat{
		SeatId: seatId,
		UserId: userId,
		UserDisplayName: userDisplayName,
		WorkName: workName,
		EnteredAt: enterDate,
		Until: exitDate,
		ColorCode: seatColorCode,
	}
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Set(ctx, map[string]interface{}{
		SeatsFirestore: firestore.ArrayUnion(seat),
	}, firestore.MergeAll)
	if err != nil {
		return Seat{}, err
	}
	return seat, nil
}

func (controller *FirestoreController) SetLastEnteredDate(userId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, map[string]interface{}{
		LastEnteredFirestore: utils.JstNow(),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetLastExitedDate(userId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, map[string]interface{}{
		LastExitedFirestore: utils.JstNow(),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UnSetSeatInRoom(seat Seat, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Set(ctx, map[string]interface{}{
		SeatsFirestore: firestore.ArrayRemove(seat),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetMyRankVisible(userId string, rankVisible bool, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, map[string]interface{}{
		RankVisibleFirestore: rankVisible,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetMyDefaultStudyMin(userId string, defaultStudyMin int, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(USERS).Doc(userId).Set(ctx, map[string]interface{}{
		DefaultStudyMinFirestore: defaultStudyMin,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) AddUserHistory(userId string, action string, details interface{}, ctx context.Context) error {
	history := UserHistoryDoc{
		Action:  action,
		Date:    utils.JstNow(),
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

func (controller *FirestoreController) UpdateSeatUntil(newUntil time.Time, userId string, ctx context.Context) error {
	// seatsを取得
	roomDoc, err := controller.RetrieveRoom(ctx)
	if err != nil {
		return err
	}
	seats := roomDoc.Seats
	
	// seatsを更新
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].Until = newUntil
			break
		}
	}
	
	// seatsをセット
	_, err = controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Update(ctx, []firestore.Update{
		{Path: SeatsFirestore, Value: seats},
	})
	if err != nil {
		return err
	}
	return nil
}


func (controller *FirestoreController) SaveLiveChatId(liveChatId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName).Set(ctx, map[string]interface{}{
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

func (controller *FirestoreController) SetDesiredMaxSeats(desiredMaxSeats int, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName).Set(ctx, map[string]interface{}{
		DesiredMaxSeatsFirestore: desiredMaxSeats,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetMaxSeats(maxSeats int, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName).Set(ctx, map[string]interface{}{
		MaxSeatsFirestore: maxSeats,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetAccessTokenOfChannelCredential(accessToken string, expireDate time.Time, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName).Set(ctx, map[string]interface{}{
		YoutubeChannelAccessTokenFirestore: accessToken,
		YoutubeChannelExpirationDate:  expireDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetAccessTokenOfBotCredential(accessToken string, expireDate time.Time, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName).Set(ctx, map[string]interface{}{
		YoutubeBotAccessTokenFirestore: accessToken,
		YoutubeBotExpirationDateFirestore:  expireDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UpdateWorkNameAtSeat(workName string, userId string, ctx context.Context) error {
	// seatsを取得
	roomDoc, err := controller.RetrieveRoom(ctx)
	if err != nil {
		return err
	}
	seats := roomDoc.Seats
	
	// seatsを更新
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].WorkName = workName
			break
		}
	}
	
	// seatsをセット
	_, err = controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Update(ctx, []firestore.Update{
		{Path: SeatsFirestore, Value: seats},
	})
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UpdateSeatColorCode(colorCode string, userId string, ctx context.Context) error {
	// seatsを取得
	roomDoc, err := controller.RetrieveRoom(ctx)
	if err != nil {
		return err
	}
	seats := roomDoc.Seats
	
	// seatsを更新
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].ColorCode = colorCode
			break
		}
	}
	
	// seatsをセット
	_, err = controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName).Update(ctx, []firestore.Update{
		{Path: SeatsFirestore, Value: seats},
	})
	if err != nil {
		return err
	}
	return nil
}


