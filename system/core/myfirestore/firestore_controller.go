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

func (controller *FirestoreController) update(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data []firestore.Update, opts ...firestore.Precondition) error {
	if tx != nil {
		return tx.Update(ref, data, opts...)
	} else {
		_, err := ref.Update(ctx, data, opts...)
		return err
	}
}

func (controller *FirestoreController) DeleteDocRef(ctx context.Context, tx *firestore.Transaction,
	ref *firestore.DocumentRef) error {
	if tx != nil {
		return tx.Delete(ref)
	} else {
		_, err := ref.Delete(ctx)
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
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: NextPageTokenDocProperty, Value: nextPageToken},
	})
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

func (controller *FirestoreController) SetLastEnteredDate(tx *firestore.Transaction, userId string, enteredDate time.Time) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: LastEnteredDocProperty, Value: enteredDate},
	})
}

func (controller *FirestoreController) SetLastExitedDate(tx *firestore.Transaction, userId string, exitedDate time.Time) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: LastExitedDocProperty, Value: exitedDate},
	})
}

func (controller *FirestoreController) SetMyRankVisible(tx *firestore.Transaction, userId string,
	rankVisible bool) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: RankVisibleDocProperty, Value: rankVisible},
	})
}

func (controller *FirestoreController) SetMyDefaultStudyMin(tx *firestore.Transaction, userId string, defaultStudyMin int) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: DefaultStudyMinDocProperty, Value: defaultStudyMin},
	})
}

func (controller *FirestoreController) SetMyFavoriteColor(tx *firestore.Transaction, userId string, colorCode string) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: FavoriteColorDocProperty, Value: colorCode},
	})
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
	return tx.Update(ref, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: newDailyTotalTimeSec},
		{Path: TotalStudySecDocProperty, Value: newTotalTimeSec},
	})
}

func (controller *FirestoreController) SaveLiveChatId(ctx context.Context, tx *firestore.Transaction, liveChatId string) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	return controller.update(ctx, tx, ref, []firestore.Update{
		{Path: LiveChatIdDocProperty, Value: liveChatId},
	})
}

func (controller *FirestoreController) InitializeUser(tx *firestore.Transaction, userId string, userData UserDoc) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	return controller.set(nil, tx, ref, userData)
}

func (controller *FirestoreController) RetrieveAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
	return controller.FirestoreClient.Collection(USERS).DocumentRefs(ctx).GetAll()
}

func (controller *FirestoreController) RetrieveAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(USERS).Where(DailyTotalStudySecDocProperty, "!=", 0).Documents(ctx)
}

func (controller *FirestoreController) ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error {
	_, err := userRef.Update(ctx, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: 0},
	})
	return err
}

func (controller *FirestoreController) SetLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastResetDailyTotalStudySecDocProperty, Value: timestamp},
	})
	return err
}

func (controller *FirestoreController) SetLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastLongTimeSittingCheckedDocProperty, Value: timestamp},
	})
	return err
}

func (controller *FirestoreController) SetLastTransferCollectionHistoryBigquery(ctx context.Context,
	timestamp time.Time) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastTransferCollectionHistoryBigqueryDocProperty, Value: timestamp},
	})
	return err
}

func (controller *FirestoreController) SetDesiredMaxSeats(ctx context.Context, tx *firestore.Transaction,
	desiredMaxSeats int) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	return controller.update(ctx, tx, ref, []firestore.Update{
		{Path: DesiredMaxSeatsDocProperty, Value: desiredMaxSeats},
	})
}

func (controller *FirestoreController) SetMaxSeats(ctx context.Context, tx *firestore.Transaction, maxSeats int) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(SystemConstantsConfigDocName)
	return controller.update(ctx, tx, ref, []firestore.Update{
		{Path: MaxSeatsDocProperty, Value: maxSeats},
	})
}

func (controller *FirestoreController) SetAccessTokenOfChannelCredential(tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	return controller.update(nil, tx, ref, []firestore.Update{
		{Path: YoutubeChannelAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeChannelExpirationDate, Value: expireDate},
	})
}

func (controller *FirestoreController) SetAccessTokenOfBotCredential(ctx context.Context, tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := controller.FirestoreClient.Collection(CONFIG).Doc(CredentialsConfigDocName)
	return controller.update(ctx, tx, ref, []firestore.Update{
		{Path: YoutubeBotAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeBotExpirationDateDocProperty, Value: expireDate},
	})
}

func (controller *FirestoreController) UpdateSeats(tx *firestore.Transaction, seats []Seat) error {
	ref := controller.FirestoreClient.Collection(ROOMS).Doc(DefaultRoomDocName)
	return tx.Update(ref, []firestore.Update{
		{Path: SeatsDocProperty, Value: seats},
	})
}

func (controller *FirestoreController) AddLiveChatHistoryDoc(ctx context.Context, tx *firestore.Transaction,
	liveChatHistoryDoc LiveChatHistoryDoc) error {
	ref := controller.FirestoreClient.Collection(LiveChatHistory).NewDoc()
	return controller.set(ctx, tx, ref, liveChatHistoryDoc)
}

func (controller *FirestoreController) Retrieve500LiveChatHistoryDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(LiveChatHistory).Where(PublishedAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (controller *FirestoreController) AddUserActivityDoc(tx *firestore.Transaction, activity UserActivityDoc) error {
	ref := controller.FirestoreClient.Collection(UserActivities).NewDoc()
	return controller.set(nil, tx, ref, activity)
}

func (controller *FirestoreController) Retrieve500UserActivityDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(UserActivities).Where(TakenAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (controller *FirestoreController) RetrieveAllUserActivityDocIdsAfterDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(UserActivities).Where(TakenAtDocProperty, ">=", date).Documents(ctx)
}

func (controller *FirestoreController) RetrieveAllUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int) *firestore.DocumentIterator {
	return controller.FirestoreClient.Collection(UserActivities).Where(TakenAtDocProperty, ">=",
		date).Where(UserIdDocProperty, "==", userId).Where(SeatIdDocProperty, "==", seatId).OrderBy(TakenAtDocProperty,
		firestore.Asc).Documents(ctx)
}
