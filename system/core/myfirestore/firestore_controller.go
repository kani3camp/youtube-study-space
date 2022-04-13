package myfirestore

import (
	"cloud.google.com/go/firestore"
	"context"
	"google.golang.org/api/option"
	"strconv"
	"time"
)

type FirestoreController struct {
	FirestoreClient *firestore.Client
}

func NewFirestoreController(ctx context.Context, clientOption option.ClientOption) (*FirestoreController, error) {
	client, err := firestore.NewClient(ctx, firestore.DetectProjectID, clientOption)
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

func (controller *FirestoreController) set(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data interface{}, opts ...firestore.SetOption) error {
	if tx != nil {
		return tx.Set(ref, data, opts...)
	} else {
		_, err := ref.Set(ctx, data, opts...)
		return err
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
	ref := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName)
	err := controller.set(nil, tx, ref, map[string]interface{}{
		SeatsFirestore: firestore.ArrayUnion(seat),
	}, firestore.MergeAll)
	if err != nil {
		return Seat{}, err
	}
	return seat, nil
}

func (controller *FirestoreController) SetLastEnteredDate(tx *firestore.Transaction, userId string, enteredDate time.Time) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return controller.set(nil, tx, ref, map[string]interface{}{
		LastEnteredFirestore: enteredDate,
	}, firestore.MergeAll)
}

func (controller *FirestoreController) SetLastExitedDate(tx *firestore.Transaction, userId string, exitedDate time.Time) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return controller.set(nil, tx, ref, map[string]interface{}{
		LastExitedFirestore: exitedDate,
	}, firestore.MergeAll)
}

func (controller *FirestoreController) AddUserActivityLog(tx *firestore.Transaction, activity UserActivityDoc) error {
	ref := controller.FirestoreClient.Collection(UserActivities).NewDoc()
	return controller.set(nil, tx, ref, activity)
}

func (controller *FirestoreController) SetMyRankVisible(tx *firestore.Transaction, userId string,
	rankVisible bool) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return controller.set(nil, tx, ref, map[string]interface{}{
		RankVisibleFirestore: rankVisible,
	}, firestore.MergeAll)
}

func (controller *FirestoreController) SetMyDefaultStudyMin(tx *firestore.Transaction, userId string, defaultStudyMin int) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return controller.set(nil, tx, ref, map[string]interface{}{
		DefaultStudyMinFirestore: defaultStudyMin,
	}, firestore.MergeAll)
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
	tx *firestore.Transaction,
	userId string,
	newTotalTimeSec int,
	newDailyTotalTimeSec int,
) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return controller.set(nil, tx, ref, map[string]interface{}{
		DailyTotalStudySecFirestore: newDailyTotalTimeSec,
		TotalStudySecFirestore:      newTotalTimeSec,
	}, firestore.MergeAll)
}

func (controller *FirestoreController) SaveLiveChatId(ctx context.Context, tx *firestore.Transaction, liveChatId string) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	err := controller.set(ctx, tx, ref, map[string]interface{}{
		LiveChatIdFirestore: liveChatId,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) InitializeUser(tx *firestore.Transaction, userId string, userData UserDoc) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return controller.set(nil, tx, ref, userData)
}

func (controller *FirestoreController) RetrieveAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
	return controller.FirestoreClient.Collection(USERS).DocumentRefs(ctx).GetAll()
}

func (controller *FirestoreController) RetrieveAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(USERS).Where(DailyTotalStudySecFirestore, "!=", 0).Documents(ctx)
}

func (controller *FirestoreController) ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error {
	//return tx.Set(userRef, map[string]interface{}{
	//	DailyTotalStudySecFirestore: 0,
	//}, firestore.MergeAll)
	_, err := userRef.Set(ctx, map[string]interface{}{
		DailyTotalStudySecFirestore: 0,
	}, firestore.MergeAll)
	return err
}

func (controller *FirestoreController) SetLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error {
	//return tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName),
	//	map[string]interface{}{
	//	LastResetDailyTotalStudySecFirestore: timestamp,
	//}, firestore.MergeAll)
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName).Set(ctx,
		map[string]interface{}{
			LastResetDailyTotalStudySecFirestore: timestamp,
		}, firestore.MergeAll)
	return err
}

func (controller *FirestoreController) SetLastTransferLiveChatHistoryBigquery(ctx context.Context,
	timestamp time.Time) error {
	//return tx.Set(controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName),
	//	map[string]interface{}{
	//	LastTransferLiveChatHistoryBigquery: timestamp,
	//}, firestore.MergeAll)
	_, err := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName).Set(ctx,
		map[string]interface{}{
			LastTransferLiveChatHistoryBigquery: timestamp,
		}, firestore.MergeAll)
	return err
}

func (controller *FirestoreController) SetDesiredMaxSeats(ctx context.Context, tx *firestore.Transaction,
	desiredMaxSeats int) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	return controller.set(ctx, tx, ref, map[string]interface{}{
		DesiredMaxSeatsFirestore: desiredMaxSeats,
	}, firestore.MergeAll)
}

func (controller *FirestoreController) SetMaxSeats(ctx context.Context, tx *firestore.Transaction, maxSeats int) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	return controller.set(ctx, tx, ref, map[string]interface{}{
		MaxSeatsFirestore: maxSeats,
	}, firestore.MergeAll)
}

func (controller *FirestoreController) SetAccessTokenOfChannelCredential(tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	return controller.set(nil, tx, ref, map[string]interface{}{
		YoutubeChannelAccessTokenFirestore: accessToken,
		YoutubeChannelExpirationDate:       expireDate,
	}, firestore.MergeAll)
}

func (controller *FirestoreController) SetAccessTokenOfBotCredential(ctx context.Context, tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	err := controller.set(ctx, tx, ref, map[string]interface{}{
		YoutubeBotAccessTokenFirestore:    accessToken,
		YoutubeBotExpirationDateFirestore: expireDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) UpdateSeats(tx *firestore.Transaction, seats []Seat) error {
	ref := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName)
	return tx.Update(ref, []firestore.Update{
		{Path: SeatsFirestore, Value: seats},
	})
}

func (controller *FirestoreController) AddLiveChatHistoryDoc(ctx context.Context, tx *firestore.Transaction,
	liveChatHistoryDoc LiveChatHistoryDoc) error {
	ref := controller.FirestoreClient.Collection(LiveChatHistory).NewDoc()
	return controller.set(ctx, tx, ref, liveChatHistoryDoc)
}

func (controller *FirestoreController) RetrieveAllLiveChatHistoryDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(LiveChatHistory).Where(PublishedAtDocName, "<", date).Documents(ctx)
}

func (controller *FirestoreController) DeleteLiveChatHistoryDoc(tx *firestore.Transaction, docId string) error {
	// TODO: 時間かかりそう。txじゃないほうがいい？
	ref := controller.FirestoreClient.Collection(LiveChatHistory).Doc(docId)
	return tx.Delete(ref)
}

func (controller *FirestoreController) AddUserActivityDoc(ctx context.Context, tx *firestore.Transaction,
	userActivityDoc UserActivityDoc) error {
	docId := "user-activity_" + userActivityDoc.Timestamp.Format("2006-01-02_15-04-05_") + strconv.Itoa(userActivityDoc.Timestamp.Nanosecond())
	ref := controller.FirestoreClient.Collection(UserActivities).Doc(docId)
	return controller.set(ctx, tx, ref, userActivityDoc)
}

func (controller *FirestoreController) RetrieveAllUserActivityDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(UserActivities).Where(TimestampDocName, "<", date).Documents(ctx)
}

func (controller *FirestoreController) RetrieveAllUserActivityDocIdsAfterDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(UserActivities).Where(TimestampDocName, ">=", date).Documents(ctx)
}

func (controller *FirestoreController) DeleteUserActivityDoc(tx *firestore.Transaction, docId string) error {
	ref := controller.FirestoreClient.Collection(UserActivities).Doc(docId)
	return tx.Delete(ref)
}

func (controller *FirestoreController) RetrieveAllUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(UserActivities).Where(TimestampDocName, ">=",
		date).Where(UserIdDocName, "==", userId).Where(SeatIdDocName, "==", seatId).OrderBy(TimestampDocName,
		firestore.Asc).Documents(ctx)
}

func (controller *FirestoreController) RetrieveUserActivity(ctx context.Context, tx *firestore.Transaction,
	docRef *firestore.DocumentRef) (UserActivityDoc, error) {
	doc, err := controller.get(ctx, tx, docRef)
	if err != nil {
		return UserActivityDoc{}, err
	}
	userActivity := UserActivityDoc{}
	err = doc.DataTo(&userActivity)
	if err != nil {
		return UserActivityDoc{}, err
	}
	return userActivity, nil
}
