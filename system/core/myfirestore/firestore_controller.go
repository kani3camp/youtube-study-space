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

func (c *FirestoreController) create(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data interface{}, opts ...firestore.SetOption) error {
	if tx != nil {
		return tx.Create(ref, data)
	} else {
		_, err := ref.Create(ctx, data)
		return err
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
func (c *FirestoreController) memberSeatsCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(MemberSeats)
}

func (c *FirestoreController) liveChatHistoryCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(LiveChatHistory)
}

func (c *FirestoreController) userActivitiesCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(UserActivities)
}

func (c *FirestoreController) seatLimitsBLACKListCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(SeatLimitsBlackList)
}

func (c *FirestoreController) seatLimitsWHITEListCollection() *firestore.CollectionRef {
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

func (c *FirestoreController) ReadCredentialsConfig(ctx context.Context, tx *firestore.Transaction) (CredentialsConfigDoc, error) {
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

func (c *FirestoreController) ReadSystemConstantsConfig(ctx context.Context, tx *firestore.Transaction) (ConstantsConfigDoc, error) {
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

func (c *FirestoreController) ReadLiveChatId(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx, tx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatId, nil
}

func (c *FirestoreController) ReadNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx, tx)
	if err != nil {
		return "", err
	}
	return credentialsDoc.YoutubeLiveChatNextPageToken, nil
}

func (c *FirestoreController) UpdateNextPageToken(ctx context.Context, nextPageToken string) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: NextPageTokenDocProperty, Value: nextPageToken},
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *FirestoreController) ReadGeneralSeats(ctx context.Context) ([]SeatDoc, error) {
	iter := c.seatsCollection().Documents(ctx)
	return GetSeatsFromIterator(iter)
}
func (c *FirestoreController) ReadMemberSeats(ctx context.Context) ([]SeatDoc, error) {
	iter := c.memberSeatsCollection().Documents(ctx)
	return GetSeatsFromIterator(iter)
}

func (c *FirestoreController) ReadSeatsExpiredUntil(ctx context.Context, thresholdTime time.Time) ([]SeatDoc, error) {
	iter := c.seatsCollection().Where(UntilDocProperty, "<", thresholdTime).Documents(ctx)
	return GetSeatsFromIterator(iter)
}

func (c *FirestoreController) ReadSeatsExpiredBreakUntil(ctx context.Context, thresholdTime time.Time) ([]SeatDoc, error) {
	iter := c.seatsCollection().Where(StateDocProperty, "==", BreakState).Where(CurrentStateUntilDocProperty, "<", thresholdTime).Documents(ctx)
	return GetSeatsFromIterator(iter)
}

func GetSeatsFromIterator(iter *firestore.DocumentIterator) ([]SeatDoc, error) {
	seats := make([]SeatDoc, 0) // jsonになったときにnullとならないように。
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

func (c *FirestoreController) ReadSeat(ctx context.Context, tx *firestore.Transaction, seatId int) (SeatDoc, error) {
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

func (c *FirestoreController) ReadSeatWithUserId(ctx context.Context, userId string) (SeatDoc, error) {
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

func (c *FirestoreController) UpdateUserLastEnteredDate(tx *firestore.Transaction, userId string, enteredDate time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: LastEnteredDocProperty, Value: enteredDate},
	})
}

func (c *FirestoreController) UpdateUserLastExitedDate(tx *firestore.Transaction, userId string, exitedDate time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: LastExitedDocProperty, Value: exitedDate},
	})
}

func (c *FirestoreController) UpdateUserRankVisible(tx *firestore.Transaction, userId string,
	rankVisible bool) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: RankVisibleDocProperty, Value: rankVisible},
	})
}

func (c *FirestoreController) UpdateUserDefaultStudyMin(tx *firestore.Transaction, userId string, defaultStudyMin int) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: DefaultStudyMinDocProperty, Value: defaultStudyMin},
	})
}

func (c *FirestoreController) UpdateUserFavoriteColor(tx *firestore.Transaction, userId string, colorCode string) error {
	ref := c.usersCollection().Doc(userId)
	return tx.Update(ref, []firestore.Update{
		{Path: FavoriteColorDocProperty, Value: colorCode},
	})
}

func (c *FirestoreController) ReadUser(ctx context.Context, tx *firestore.Transaction, userId string) (UserDoc, error) {
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

func (c *FirestoreController) UpdateUserTotalTime(
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

func (c *FirestoreController) UpdateLiveChatId(ctx context.Context, tx *firestore.Transaction, liveChatId string) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: LiveChatIdDocProperty, Value: liveChatId},
	})
}

func (c *FirestoreController) CreateUser(tx *firestore.Transaction, userId string, userData UserDoc) error {
	ref := c.usersCollection().Doc(userId)
	return c.create(nil, tx, ref, userData)
}

func (c *FirestoreController) GetAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
	return c.usersCollection().DocumentRefs(ctx).GetAll()
}

func (c *FirestoreController) GetAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator {
	return c.usersCollection().Where(DailyTotalStudySecDocProperty, "!=", 0).Documents(ctx)
}

func (c *FirestoreController) ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error {
	_, err := userRef.Update(ctx, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: 0},
	})
	return err
}

func (c *FirestoreController) UpdateLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastResetDailyTotalStudySecDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreController) UpdateLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastLongTimeSittingCheckedDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreController) UpdateLastTransferCollectionHistoryBigquery(ctx context.Context,
	timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastTransferCollectionHistoryBigqueryDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreController) UpdateDesiredMaxSeats(ctx context.Context, tx *firestore.Transaction,
	desiredMaxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: DesiredMaxSeatsDocProperty, Value: desiredMaxSeats},
	})
}
func (c *FirestoreController) UpdateDesiredMemberMaxSeats(ctx context.Context, tx *firestore.Transaction,
	desiredMemberMaxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: DesiredMemberMaxSeatsDocProperty, Value: desiredMemberMaxSeats},
	})
}

func (c *FirestoreController) UpdateMaxSeats(ctx context.Context, tx *firestore.Transaction, maxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: MaxSeatsDocProperty, Value: maxSeats},
	})
}
func (c *FirestoreController) UpdateMemberMaxSeats(ctx context.Context, tx *firestore.Transaction, memberMaxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: MemberMaxSeatsDocProperty, Value: memberMaxSeats},
	})
}

func (c *FirestoreController) UpdateAccessTokenOfChannelCredential(tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(nil, tx, ref, []firestore.Update{
		{Path: YoutubeChannelAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeChannelExpirationDate, Value: expireDate},
	})
}

func (c *FirestoreController) UpdateAccessTokenOfBotCredential(ctx context.Context, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, nil, ref, []firestore.Update{
		{Path: YoutubeBotAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeBotExpirationDateDocProperty, Value: expireDate},
	})
}

func (c *FirestoreController) CreateSeat(tx *firestore.Transaction, seat SeatDoc) error {
	ref := c.seatsCollection().Doc(strconv.Itoa(seat.SeatId))
	return tx.Create(ref, seat)
}

func (c *FirestoreController) UpdateSeat(tx *firestore.Transaction, seat SeatDoc) error {
	ref := c.seatsCollection().Doc(strconv.Itoa(seat.SeatId))
	return c.set(nil, tx, ref, seat)
}

func (c *FirestoreController) DeleteSeat(tx *firestore.Transaction, seatId int) error {
	ref := c.seatsCollection().Doc(strconv.Itoa(seatId))
	return c.delete(nil, tx, ref)
}

func (c *FirestoreController) CreateLiveChatHistoryDoc(ctx context.Context, tx *firestore.Transaction,
	liveChatHistoryDoc LiveChatHistoryDoc) error {
	ref := c.liveChatHistoryCollection().NewDoc()
	return c.create(ctx, tx, ref, liveChatHistoryDoc)
}

func (c *FirestoreController) Get500LiveChatHistoryDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return c.liveChatHistoryCollection().Where(PublishedAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreController) CreateUserActivityDoc(tx *firestore.Transaction, activity UserActivityDoc) error {
	ref := c.userActivitiesCollection().NewDoc()
	return c.create(nil, tx, ref, activity)
}

func (c *FirestoreController) Get500UserActivityDocIdsBeforeDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreController) GetAllUserActivityDocIdsAfterDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreController) GetAllUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=",
		date).Where(UserIdDocProperty, "==", userId).Where(SeatIdDocProperty, "==", seatId).OrderBy(TakenAtDocProperty,
		firestore.Asc).Documents(ctx)
	return getUserActivitiesFromIterator(iter)
}

func (c *FirestoreController) GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIdDocProperty, "==", userId).
		Where(SeatIdDocProperty, "==", seatId).Where(ActivityTypeDocProperty, "==", EnterRoomActivity).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getUserActivitiesFromIterator(iter)
}

func (c *FirestoreController) GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIdDocProperty, "==", userId).
		Where(SeatIdDocProperty, "==", seatId).Where(ActivityTypeDocProperty, "==", ExitRoomActivity).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getUserActivitiesFromIterator(iter)
}

func getUserActivitiesFromIterator(iter *firestore.DocumentIterator) ([]UserActivityDoc, error) {
	var activityList []UserActivityDoc
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return []UserActivityDoc{}, err
		}
		var activity UserActivityDoc
		err = doc.DataTo(&activity)
		if err != nil {
			return []UserActivityDoc{}, err
		}
		activityList = append(activityList, activity)
	}
	return activityList, nil
}

// GetUsersActiveAfterDate date以後に入室したことのあるuserを全て取得
func (c *FirestoreController) GetUsersActiveAfterDate(ctx context.Context, date time.Time) *firestore.DocumentIterator {
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

func (c *FirestoreController) ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string) ([]SeatLimitDoc, error) {
	iter := c.seatLimitsWHITEListCollection().Where(SeatIdDocProperty, "==", seatId).Where(UserIdDocProperty, "==", userId).Documents(ctx)
	return getSeatLimitsDocsFromIterator(iter)
}

func (c *FirestoreController) ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string) ([]SeatLimitDoc, error) {
	iter := c.seatLimitsBLACKListCollection().Where(SeatIdDocProperty, "==", seatId).Where(UserIdDocProperty, "==", userId).Documents(ctx)
	return getSeatLimitsDocsFromIterator(iter)
}

func getSeatLimitsDocsFromIterator(iter *firestore.DocumentIterator) ([]SeatLimitDoc, error) {
	var seatLimits []SeatLimitDoc
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var seatLimitDoc SeatLimitDoc
		err = doc.DataTo(&seatLimitDoc)
		if err != nil {
			return nil, err
		}
		seatLimits = append(seatLimits, seatLimitDoc)
	}
	return seatLimits, nil
}

func (c *FirestoreController) CreateSeatLimitInWhiteList(ctx context.Context, seatId int, userId string, createdAt, until time.Time) error {
	ref := c.seatLimitsWHITEListCollection().NewDoc()
	return c.createSeatLimit(ctx, ref, seatId, userId, createdAt, until)
}

func (c *FirestoreController) CreateSeatLimitInBlackList(ctx context.Context, seatId int, userId string, createdAt, until time.Time) error {
	ref := c.seatLimitsBLACKListCollection().NewDoc()
	return c.createSeatLimit(ctx, ref, seatId, userId, createdAt, until)
}

func (c *FirestoreController) createSeatLimit(ctx context.Context, ref *firestore.DocumentRef, seatId int, userId string, createdAt, until time.Time) error {
	data := SeatLimitDoc{
		SeatId:    seatId,
		UserId:    userId,
		CreatedAt: createdAt,
		Until:     until,
	}
	return c.create(ctx, nil, ref, data)
}

// Get500SeatLimitsAfterUntilInWHITEList returns all seat limit docs whose `until` is after `thresholdTime`.
func (c *FirestoreController) Get500SeatLimitsAfterUntilInWHITEList(ctx context.Context, thresholdTime time.Time) *firestore.DocumentIterator {
	return c.seatLimitsWHITEListCollection().Where(UntilDocProperty, "<", thresholdTime).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

// Get500SeatLimitsAfterUntilInBLACKList returns all seat limit docs whose `until` is after `thresholdTime`.
func (c *FirestoreController) Get500SeatLimitsAfterUntilInBLACKList(ctx context.Context, thresholdTime time.Time) *firestore.DocumentIterator {
	return c.seatLimitsBLACKListCollection().Where(UntilDocProperty, "<", thresholdTime).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreController) DeleteSeatLimitInWHITEList(ctx context.Context, docId string) error {
	ref := c.seatLimitsWHITEListCollection().Doc(docId)
	return c.delete(ctx, nil, ref)
}

func (c *FirestoreController) DeleteSeatLimitInBLACKList(ctx context.Context, docId string) error {
	ref := c.seatLimitsBLACKListCollection().Doc(docId)
	return c.delete(ctx, nil, ref)
}
