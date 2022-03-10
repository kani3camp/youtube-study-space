package myfirestore

import (
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"strconv"
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

func (controller *FirestoreController) SetLastEnteredDate(tx *firestore.Transaction, userId string, enteredDate time.Time) error {
	err := tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), map[string]interface{}{
		LastEnteredFirestore: enteredDate,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) SetLastExitedDate(tx *firestore.Transaction, userId string, exitedDate time.Time) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	err := tx.Set(ref, map[string]interface{}{
		LastExitedFirestore: exitedDate,
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

func (controller *FirestoreController) AddUserHistory(tx *firestore.Transaction, userId string, action string, details interface{}) error {
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
	tx *firestore.Transaction,
	userId string,
	newTotalTimeSec int,
	newDailyTotalTimeSec int,
) error {
	ref := controller.FirestoreClient.Collection(USERS).Doc(userId)
	err := tx.Set(ref, map[string]interface{}{
		DailyTotalStudySecFirestore: newDailyTotalTimeSec,
		TotalStudySecFirestore:      newTotalTimeSec,
	}, firestore.MergeAll)
	if err != nil {
		return err
	}
	return nil
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
	return tx.Set(controller.FirestoreClient.Collection(USERS).Doc(userId), userData)
}

func (controller *FirestoreController) RetrieveAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
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

func (controller *FirestoreController) SetMaxSeats(tx *firestore.Transaction, maxSeats int) error {
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
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) AddLiveChatHistoryDoc(liveChatHistoryDoc LiveChatHistoryDoc, ctx context.Context) error {
	docId := "live-chat_" + liveChatHistoryDoc.PublishedAt.Format("2006-01-02_15-04-05_") + strconv.Itoa(liveChatHistoryDoc.PublishedAt.Nanosecond())
	_, err := controller.FirestoreClient.Collection(LIVE_CHAT_HISTORY).Doc(docId).Set(ctx, liveChatHistoryDoc)
	if err != nil {
		return err
	}
	return nil
}

func (controller *FirestoreController) RetrieveAllLiveChatHistoryDocIdsBeforeDate(date time.Time, ctx context.Context) ([]string, error) {
	iter := controller.FirestoreClient.Collection(LIVE_CHAT_HISTORY).Where(PublishedAtDocName, "<", date).Documents(ctx)
	var docIds []string
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return []string{}, err
		}
		docIds = append(docIds, doc.Ref.ID)
	}
	return docIds, nil
}

func (controller *FirestoreController) DeleteLiveChatHistoryDoc(docId string, ctx context.Context) error {
	_, err := controller.FirestoreClient.Collection(LIVE_CHAT_HISTORY).Doc(docId).Delete(ctx)
	if err != nil {
		return err
	}
	return nil
}


