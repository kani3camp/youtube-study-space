package myfirestore

import (
	"cloud.google.com/go/firestore"
	"context"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (c *FirestoreController) get(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef) (*firestore.DocumentSnapshot, error) {
	if tx != nil {
		return tx.Get(ref)
	} else {
		return ref.Get(ctx)
	}
}

func (c *FirestoreController) set(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data interface{}, opts ...firestore.SetOption) error {
	if tx != nil {
		return tx.Set(ref, data, opts...)
	} else {
		_, err := ref.Set(ctx, data, opts...)
		return err
	}
}

func (c *FirestoreController) update(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data []firestore.Update, opts ...firestore.Precondition) error {
	if tx != nil {
		return tx.Update(ref, data, opts...)
	} else {
		_, err := ref.Update(ctx, data, opts...)
		return err
	}
}

func (c *FirestoreController) delete(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, opts ...firestore.Precondition) error {
	// TODO: ドキュメントが元から存在しないときは明示的にエラーを返したい
	if tx != nil {
		return tx.Delete(ref, opts...)
	} else {
		_, err := ref.Delete(ctx, opts...)
		return err
	}
}

func (c *FirestoreController) configCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(CONFIG)
}

func (c *FirestoreController) usersCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(USERS)
}

func (c *FirestoreController) seatsCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(SEATS)
}

func (c *FirestoreController) liveChatHistoryCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(LiveChatHistory)
}

func (c *FirestoreController) userActivitiesCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(UserActivities)
}

func (c *FirestoreController) seatLimitsBlackListCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(SeatLimitsBlackList)
}

func (c *FirestoreController) seatLimitsWhiteListCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(SeatLimitsWhiteList)
}

func (c *FirestoreController) DeleteDocRef(ctx context.Context, tx *firestore.Transaction,
	ref *firestore.DocumentRef) error {
	if tx != nil {
		return tx.Delete(ref)
	} else {
		_, err := ref.Delete(ctx)
		return err
	}
}

func (c *FirestoreController) RetrieveCredentialsConfig(ctx context.Context, tx *firestore.Transaction) (CredentialsConfigDoc, error) {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	doc, err := c.get(ctx, tx, ref)
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

func (c *FirestoreController) RetrieveSystemConstantsConfig(ctx context.Context, tx *firestore.Transaction) (ConstantsConfigDoc, error) {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	doc, err := c.get(ctx, tx, ref)
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

func (c *FirestoreController) RetrieveLiveChatId(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.RetrieveCredentialsConfig(ctx, tx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatId, nil
}

func (c *FirestoreController) RetrieveNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.RetrieveCredentialsConfig(ctx, tx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatNextPageToken, nil
}

func (c *FirestoreController) SaveNextPageToken(ctx context.Context, nextPageToken string) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: NextPageTokenDocProperty, Value: nextPageToken},
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *FirestoreController) RetrieveSeats(ctx context.Context) ([]SeatDoc, error) {
	seats := make([]SeatDoc, 0) // jsonになったときにnullとならないように。
	iter := c.seatsCollection().Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return []SeatDoc{}, err
		}
		var seatDoc SeatDoc
		err = doc.DataTo(&seatDoc)
		if err != nil {
			return []SeatDoc{}, err
		}
		seats = append(seats, seatDoc)
	}
	return seats, nil
}

func (c *FirestoreController) RetrieveSeat(ctx context.Context, tx *firestore.Transaction, seatId int) (SeatDoc, error) {
	ref := c.seatsCollection().Doc(strconv.Itoa(seatId))
	doc, err := c.get(ctx, tx, ref)
	if err != nil {
		return SeatDoc{}, err // NotFoundの場合もerrに含まれる
	}
	var seatDoc SeatDoc
	err = doc.DataTo(&seatDoc)
	if err != nil {
		return SeatDoc{}, err
	}
	return seatDoc, nil
}

func (c *FirestoreController) RetrieveSeatWithUserId(ctx context.Context, userId string) (SeatDoc, error) {
	docs, err := c.seatsCollection().Where(UserIdDocProperty, "==", userId).Documents(ctx).GetAll()
	if err != nil {
		return SeatDoc{}, err
	}
	if len(docs) >= 2 {
		return SeatDoc{}, errors.New("There are more than two seats with the user id = " + userId + " !!")
	}
	if len(docs) == 1 {
		var seatDoc SeatDoc
		err := docs[0].DataTo(&seatDoc)
		if err != nil {
			return SeatDoc{}, err
		}
		return seatDoc, nil
	}
	return SeatDoc{}, status.Errorf(codes.NotFound, "%s not found", "the document with user id = "+userId)
}

func (c *FirestoreController) SetLastEnteredDate(tx *firestore.Transaction, userId string, enteredDate time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: LastEnteredDocProperty, Value: enteredDate},
	})
}

func (c *FirestoreController) SetLastExitedDate(tx *firestore.Transaction, userId string, exitedDate time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: LastExitedDocProperty, Value: exitedDate},
	})
}

func (c *FirestoreController) SetMyRankVisible(tx *firestore.Transaction, userId string,
	rankVisible bool) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: RankVisibleDocProperty, Value: rankVisible},
	})
}

func (c *FirestoreController) SetMyDefaultStudyMin(tx *firestore.Transaction, userId string, defaultStudyMin int) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: DefaultStudyMinDocProperty, Value: defaultStudyMin},
	})
}

func (c *FirestoreController) SetMyFavoriteColor(tx *firestore.Transaction, userId string, colorCode string) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: FavoriteColorDocProperty, Value: colorCode},
	})
}

func (c *FirestoreController) RetrieveUser(ctx context.Context, tx *firestore.Transaction, userId string) (UserDoc, error) {
	ref := c.usersCollection().Doc(userId)
	doc, err := c.get(ctx, tx, ref)
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

func (c *FirestoreController) UpdateTotalTime(
	tx *firestore.Transaction,
	userId string,
	newTotalTimeSec int,
	newDailyTotalTimeSec int,
) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: newDailyTotalTimeSec},
		{Path: TotalStudySecDocProperty, Value: newTotalTimeSec},
	})
}

func (c *FirestoreController) UpdateUserRankPoint(tx *firestore.Transaction, userId string, rp int) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: RankPointDocProperty, Value: rp},
	})
}

func (c *FirestoreController) UpdateUserLastRPProcessed(tx *firestore.Transaction, userId string, date time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: LastRPProcessedDocProperty, Value: date},
	})
}

func (c *FirestoreController) SaveLiveChatId(ctx context.Context, tx *firestore.Transaction, liveChatId string) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: LiveChatIdDocProperty, Value: liveChatId},
	})
}

func (c *FirestoreController) InitializeUser(tx *firestore.Transaction, userId string, userData UserDoc) error {
	ref := c.usersCollection().Doc(userId)
	return c.set(nil, tx, ref, userData)
}

func (c *FirestoreController) RetrieveAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
	return c.usersCollection().DocumentRefs(ctx).GetAll()
}

func (c *FirestoreController) RetrieveAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator {
	return c.usersCollection().Where(DailyTotalStudySecDocProperty, "!=", 0).Documents(ctx)
}

func (c *FirestoreController) ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error {
	_, err := userRef.Update(ctx, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: 0},
	})
	return err
}

func (c *FirestoreController) SetLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastResetDailyTotalStudySecDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreController) SetLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastLongTimeSittingCheckedDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreController) SetLastTransferCollectionHistoryBigquery(ctx context.Context,
	timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastTransferCollectionHistoryBigqueryDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreController) SetDesiredMaxSeats(ctx context.Context, tx *firestore.Transaction,
	desiredMaxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: DesiredMaxSeatsDocProperty, Value: desiredMaxSeats},
	})
}

func (c *FirestoreController) SetMaxSeats(ctx context.Context, tx *firestore.Transaction, maxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: MaxSeatsDocProperty, Value: maxSeats},
	})
}

func (c *FirestoreController) SetAccessTokenOfChannelCredential(tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(nil, tx, ref, []firestore.Update{
		{Path: YoutubeChannelAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeChannelExpirationDate, Value: expireDate},
	})
}

func (c *FirestoreController) SetAccessTokenOfBotCredential(ctx context.Context, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, nil, ref, []firestore.Update{
		{Path: YoutubeBotAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeBotExpirationDateDocProperty, Value: expireDate},
	})
}

func (c *FirestoreController) AddSeat(tx *firestore.Transaction, seat SeatDoc) error {
	ref := c.seatsCollection().Doc(strconv.Itoa(seat.SeatId))
	return tx.Create(ref, seat)
}

func (c *FirestoreController) UpdateSeat(tx *firestore.Transaction, seat SeatDoc) error {
	ref := c.seatsCollection().Doc(strconv.Itoa(seat.SeatId))
	return c.set(nil, tx, ref, seat)
}

func (c *FirestoreController) RemoveSeat(tx *firestore.Transaction, seatId int) error {
	ref := c.seatsCollection().Doc(strconv.Itoa(seatId))
	return c.delete(nil, tx, ref)
}

func (c *FirestoreController) AddLiveChatHistoryDoc(ctx context.Context, tx *firestore.Transaction,
	liveChatHistoryDoc LiveChatHistoryDoc) error {
	ref := c.liveChatHistoryCollection().NewDoc()
	return c.set(ctx, tx, ref, liveChatHistoryDoc)
}

func (c *FirestoreController) Retrieve500LiveChatHistoryDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return c.liveChatHistoryCollection().Where(PublishedAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreController) AddUserActivityDoc(tx *firestore.Transaction, activity UserActivityDoc) error {
	ref := c.userActivitiesCollection().NewDoc()
	return c.set(nil, tx, ref, activity)
}

func (c *FirestoreController) Retrieve500UserActivityDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreController) RetrieveAllUserActivityDocIdsAfterDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreController) RetrieveAllUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, ">=",
		date).Where(UserIdDocProperty, "==", userId).Where(SeatIdDocProperty, "==", seatId).OrderBy(TakenAtDocProperty,
		firestore.Asc).Documents(ctx)
}

// RetrieveUsersActiveAfterDate date以後に入室したことのあるuserを全て取得
func (c *FirestoreController) RetrieveUsersActiveAfterDate(ctx context.Context, date time.Time) *firestore.DocumentIterator {
	return c.usersCollection().Where(LastEnteredDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreController) UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(
	tx *firestore.Transaction, userId string, isContinuousActive bool, currentActivityStateStarted time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(nil, tx, ref, []firestore.Update{
		{Path: IsContinuousActiveDocProperty, Value: isContinuousActive},
		{Path: CurrentActivityStateStartedDocProperty, Value: currentActivityStateStarted},
	})
}

func (c *FirestoreController) UpdateUserLastPenaltyImposedDays(tx *firestore.Transaction, userId string, lastPenaltyImposedDays int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(nil, tx, ref, []firestore.Update{
		{Path: LastPenaltyImposedDaysDocProperty, Value: lastPenaltyImposedDays},
	})
}

func (c *FirestoreController) UpdateUserRPAndLastPenaltyImposedDays(tx *firestore.Transaction, userId string,
	newRP int, newLastPenaltyImposedDays int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(nil, tx, ref, []firestore.Update{
		{Path: RankPointDocProperty, Value: newRP},
		{Path: LastPenaltyImposedDaysDocProperty, Value: newLastPenaltyImposedDays},
	})
}
