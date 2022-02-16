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

func (controller *FirestoreController) get(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef) (*firestore.DocumentSnapshot, error) {
	if tx != nil {
		return tx.Get(ref)
	} else {
		return ref.Get(ctx)
	}
}

func (controller *FirestoreController) RetrieveCredentialsConfig(ctx context.Context, tx *firestore.Transaction) (CredentialsConfigDoc, error) {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	doc, err := controller.get(ctx, tx, ref)
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

func (controller *FirestoreController) RetrieveSystemConstantsConfig(ctx context.Context, tx *firestore.Transaction) (ConstantsConfigDoc, error) {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	doc, err := controller.get(ctx, tx, ref)
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

func (controller *FirestoreController) RetrieveLiveChatId(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := controller.RetrieveCredentialsConfig(ctx, tx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatId, nil
}

func (controller *FirestoreController) RetrieveNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := controller.RetrieveCredentialsConfig(ctx, tx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatNextPageToken, nil
}

func (controller *FirestoreController) SaveNextPageToken(ctx context.Context, nextPageToken string) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	_, err := ref.Set(ctx, map[string]interface{}{
		NextPageTokenFirestore: nextPageToken,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveRoom(ctx context.Context, tx *firestore.Transaction) (RoomDoc, error) {
	roomData := NewRoomDoc()
	ref := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName)
	doc, err := controller.get(ctx, tx, ref)
	if err != nil {
		return RoomDoc{}, err
	}
	err = doc.DataTo(&roomData)
	if err != nil {
		return RoomDoc{}, err
	}
	return roomData, nil
}

func (controller *FirestoreController) SetSeat(
	_ context.Context,
	tx *firestore.Transaction,
	seatId int,
	workName string,
	enterDate time.Time,
	exitDate time.Time,
	seatColorCode string,
	userId string,
	userDisplayName string,
) (Seat, error) {
	// TODO {Path: , Val: }形式に書き直せないかな？
	seat := Seat{
		SeatId:          seatId,
		UserId:          userId,
		UserDisplayName: userDisplayName,
		WorkName:        workName,
		EnteredAt:       enterDate,
		Until:           exitDate,
		ColorCode:       seatColorCode,
	}
	err := tx.Set(controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName), map[string]interface{}{
		SeatsFirestore: firestore.ArrayUnion(seat),
	}, firestore.MergeAll)
	if err != nil {
		return Seat{}, err
	}
	return seat, nil
}

func (controller *FirestoreController) SetLastEnteredDate(_ context.Context, tx *firestore.Transaction, userId string, enteredDate time.Time) error {
	err := tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), map[string]interface{}{
		LastEnteredFirestore: enteredDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetLastExitedDate(_ context.Context, tx *firestore.Transaction, userId string, exitedDate time.Time) error {
	err := tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), map[string]interface{}{
		LastExitedFirestore: exitedDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UnSetSeatInRoom(_ context.Context, tx *firestore.Transaction, seat Seat) error {
	err := tx.Set(controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName), map[string]interface{}{
		SeatsFirestore: firestore.ArrayRemove(seat),
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetMyRankVisible(_ context.Context, tx *firestore.Transaction, userId string, rankVisible bool) error {
	err := tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), map[string]interface{}{
		RankVisibleFirestore: rankVisible,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetMyDefaultStudyMin(tx *firestore.Transaction, userId string, defaultStudyMin int) error {
	err := tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), map[string]interface{}{
		DefaultStudyMinFirestore: defaultStudyMin,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) AddUserHistory(_ context.Context, tx *firestore.Transaction, userId string, action string, details interface{}) error {
	history := UserHistoryDoc{
		Action:  action,
		Date:    utils.JstNow(),
		Details: details,
	}
	newDocRef := controller.FirestoreClient.Collection(USERS).Doc(userId).Collection(HISTORY).NewDoc()
	err := tx.Set(newDocRef, history)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveUser(ctx context.Context, tx *firestore.Transaction, userId string) (UserDoc, error) {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	doc, err := controller.get(ctx, tx, ref)
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

func (controller *FirestoreController) UpdateTotalTime(
	_ context.Context,
	tx *firestore.Transaction,
	userId string,
	newTotalTimeSec int,
	newDailyTotalTimeSec int,
) error {
	err := tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), map[string]interface{}{
		DailyTotalStudySecFirestore: newDailyTotalTimeSec,
		TotalStudySecFirestore:      newTotalTimeSec,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SaveLiveChatId(_ context.Context, tx *firestore.Transaction, liveChatId string) error {
	err := tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName), map[string]interface{}{
		LiveChatIdFirestore: liveChatId,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) InitializeUser(tx *firestore.Transaction, userId string, userData UserDoc) error {
	return tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), userData)
}

func (controller *FirestoreController) RetrieveAllUserDocRefs(ctx context.Context, _ *firestore.Transaction) ([]*firestore.DocumentRef, error) {
	return controller.FirestoreClient.Collection(USERS).DocumentRefs(ctx).GetAll()
}

func (controller *FirestoreController) RetrieveAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(USERS).Where(DailyTotalStudySecFirestore, "!=", 0).Documents(ctx)
}

func (controller *FirestoreController) ResetDailyTotalStudyTime(tx *firestore.Transaction, userRef *firestore.DocumentRef) error {
	err := tx.Set(userRef, map[string]interface{}{
		DailyTotalStudySecFirestore: 0,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetLastResetDailyTotalStudyTime(tx *firestore.Transaction, date time.Time) error {
	err := tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName), map[string]interface{}{
		LastResetDailyTotalStudySecFirestore: date,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetDesiredMaxSeats(tx *firestore.Transaction, desiredMaxSeats int) error {
	err := tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName), map[string]interface{}{
		DesiredMaxSeatsFirestore: desiredMaxSeats,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetMaxSeats(_ context.Context, tx *firestore.Transaction, maxSeats int) error {
	err := tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName), map[string]interface{}{
		MaxSeatsFirestore: maxSeats,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetAccessTokenOfChannelCredential(tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	err := tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName), map[string]interface{}{
		YoutubeChannelAccessTokenFirestore: accessToken,
		YoutubeChannelExpirationDate:       expireDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetAccessTokenOfBotCredential(_ context.Context, tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	err := tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName), map[string]interface{}{
		YoutubeBotAccessTokenFirestore:    accessToken,
		YoutubeBotExpirationDateFirestore: expireDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

// UpdateSeatWorkName 入室中のユーザーの作業名を更新する。入室中かどうかはチェック済みとする。
func (controller *FirestoreController) UpdateSeatWorkName(ctx context.Context, tx *firestore.Transaction, workName string, userId string) error {
	// seatsを取得
	roomDoc, err := controller.RetrieveRoom(ctx, tx)
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
	err = tx.Update(controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName), []firestore.Update{
		{Path: SeatsFirestore, Value: seats},
	})
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UpdateSeatColorCode(ctx context.Context, tx *firestore.Transaction, colorCode string, userId string) error {
	// seatsを取得
	roomDoc, err := controller.RetrieveRoom(ctx, tx)
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
	err = tx.Update(controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName), []firestore.Update{
		{Path: SeatsFirestore, Value: seats},
	})
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UpdateSeatUntil(ctx context.Context, tx *firestore.Transaction, newUntil time.Time, userId string) error {
	// seatsを取得
	roomDoc, err := controller.RetrieveRoom(ctx, tx)
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
	err = tx.Update(controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName), []firestore.Update{
		{Path: SeatsFirestore, Value: seats},
	})
	if err != nil {
		return err
	}
	return nil
}
