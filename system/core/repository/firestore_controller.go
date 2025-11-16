package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreControllerImplements struct {
	firestoreClient DB
}

func NewFirestoreController(ctx context.Context, clientOption option.ClientOption) (*FirestoreControllerImplements, error) {
	client, err := firestore.NewClient(ctx, firestore.DetectProjectID, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in firestore.NewClient: %w", err)
	}

	return &FirestoreControllerImplements{
		firestoreClient: client,
	}, nil
}

func (c *FirestoreControllerImplements) RunTransaction(ctx context.Context, f func(ctx context.Context) error) error {
	return c.firestoreClient.(*firestore.Client).RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return f(ctx)
	})
}

func (c *FirestoreControllerImplements) get(ctx context.Context, ref *firestore.DocumentRef) (*firestore.DocumentSnapshot, error) {
	return ref.Get(ctx)
}


func (c *FirestoreControllerImplements) create(ctx context.Context, ref *firestore.DocumentRef, data interface{}) error {
	_, err := ref.Create(ctx, data)
	return err
}

func (c *FirestoreControllerImplements) set(ctx context.Context, ref *firestore.DocumentRef, data interface{}, opts ...firestore.SetOption) error {
	_, err := ref.Set(ctx, data, opts...)
	return err
}

func (c *FirestoreControllerImplements) update(ctx context.Context, ref *firestore.DocumentRef, data []firestore.Update, opts ...firestore.Precondition) error {
	_, err := ref.Update(ctx, data, opts...)
	return err
}

// delete deletes the document. If the document doesn't exist, it does nothing and returns no error.
func (c *FirestoreControllerImplements) delete(ctx context.Context, ref *firestore.DocumentRef, opts ...firestore.Precondition) error {
	_, err := ref.Delete(ctx, opts...)
	return err
}

func (c *FirestoreControllerImplements) configCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(CONFIG)
}

func (c *FirestoreControllerImplements) usersCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(USERS)
}

func (c *FirestoreControllerImplements) seatsCollection(isMemberSeat bool) *firestore.CollectionRef {
	if isMemberSeat {
		return c.memberSeatsCollection()
	} else {
		return c.generalSeatsCollection()
	}
}

func (c *FirestoreControllerImplements) menuCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(MENU)
}

func (c *FirestoreControllerImplements) orderHistoryCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(OrderHistory)
}

func (c *FirestoreControllerImplements) generalSeatsCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(SEATS)
}
func (c *FirestoreControllerImplements) memberSeatsCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(MemberSeats)
}

func (c *FirestoreControllerImplements) liveChatHistoryCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(LiveChatHistory)
}

func (c *FirestoreControllerImplements) userActivitiesCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(UserActivities)
}

func (c *FirestoreControllerImplements) generalSeatLimitsBLACKListCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(SeatLimitsBlackList)
}

func (c *FirestoreControllerImplements) generalSeatLimitsWHITEListCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(SeatLimitsWhiteList)
}

func (c *FirestoreControllerImplements) memberSeatLimitsBLACKListCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(MemberSeatLimitsBlackList)
}

func (c *FirestoreControllerImplements) memberSeatLimitsWHITEListCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(MemberSeatLimitsWhiteList)
}

func (c *FirestoreControllerImplements) workNameTrendCollection() *firestore.CollectionRef {
	return c.firestoreClient.(*firestore.Client).Collection(WorkNameTrend)
}

func (c *FirestoreControllerImplements) DeleteDocRef(ctx context.Context,
	ref *firestore.DocumentRef) error {
	_, err := ref.Delete(ctx)
	return err
}

func (c *FirestoreControllerImplements) ReadCredentialsConfig(ctx context.Context) (CredentialsConfigDoc, error) {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	doc, err := c.get(ctx, ref)
	if err != nil {
		return CredentialsConfigDoc{}, err
	}
	var credentialsData CredentialsConfigDoc
	if err := doc.DataTo(&credentialsData); err != nil {
		return CredentialsConfigDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return credentialsData, nil
}

func (c *FirestoreControllerImplements) ReadSystemConstantsConfig(ctx context.Context) (ConstantsConfigDoc, error) {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	doc, err := c.get(ctx, ref)
	if err != nil {
		return ConstantsConfigDoc{}, err
	}
	var constantsConfig ConstantsConfigDoc
	if err := doc.DataTo(&constantsConfig); err != nil {
		return ConstantsConfigDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return constantsConfig, nil
}

func (c *FirestoreControllerImplements) ReadLiveChatId(ctx context.Context) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("in ReadCredentialsConfig: %w", err)
	}
	return credentialsDoc.YoutubeLiveChatId, nil
}

func (c *FirestoreControllerImplements) ReadNextPageToken(ctx context.Context) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("in ReadCredentialsConfig: %w", err)
	}
	return credentialsDoc.YoutubeLiveChatNextPageToken, nil
}

func (c *FirestoreControllerImplements) UpdateNextPageToken(ctx context.Context, nextPageToken string) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: NextPageTokenDocProperty, Value: nextPageToken},
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *FirestoreControllerImplements) ReadGeneralSeats(ctx context.Context) ([]SeatDoc, error) {
	iter := c.generalSeatsCollection().Documents(ctx)
	return getDocDataFromIterator[SeatDoc](iter)
}
func (c *FirestoreControllerImplements) ReadMemberSeats(ctx context.Context) ([]SeatDoc, error) {
	iter := c.memberSeatsCollection().Documents(ctx)
	return getDocDataFromIterator[SeatDoc](iter)
}

func (c *FirestoreControllerImplements) ReadSeatsExpiredUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error) {
	iter := c.seatsCollection(isMemberSeat).Where(UntilDocProperty, "<", thresholdTime).Documents(ctx)
	return getDocDataFromIterator[SeatDoc](iter)
}

func (c *FirestoreControllerImplements) ReadSeatsExpiredBreakUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error) {
	iter := c.seatsCollection(isMemberSeat).Where(StateDocProperty, "==", BreakState).Where(CurrentStateUntilDocProperty, "<", thresholdTime).Documents(ctx)
	return getDocDataFromIterator[SeatDoc](iter)
}

func (c *FirestoreControllerImplements) ReadSeat(ctx context.Context, seatId int, isMemberSeat bool) (SeatDoc, error) {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seatId))
	doc, err := c.get(ctx, ref)
	if err != nil {
		return SeatDoc{}, err // NotFoundの場合もerrに含まれる
	}
	var seatDoc SeatDoc
	if err := doc.DataTo(&seatDoc); err != nil {
		return SeatDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return seatDoc, nil
}

func (c *FirestoreControllerImplements) ReadSeatWithUserId(ctx context.Context, userId string, isMemberSeat bool) (SeatDoc, error) {
	docs, err := c.seatsCollection(isMemberSeat).Where(UserIdDocProperty, "==", userId).Documents(ctx).GetAll()
	if err != nil {
		return SeatDoc{}, err
	}
	if len(docs) >= 2 {
		return SeatDoc{}, errors.New("There are more than two seats with the user id = " + userId + " !!")
	}
	if len(docs) == 1 {
		var seatDoc SeatDoc
		if err := docs[0].DataTo(&seatDoc); err != nil {
			return SeatDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
		}
		return seatDoc, nil
	}
	return SeatDoc{}, status.Errorf(codes.NotFound, "%s not found", "the document with user id = "+userId)
}

func (c *FirestoreControllerImplements) ReadActiveWorkNameSeats(ctx context.Context, isMemberSeat bool) ([]SeatDoc, error) {
	iter := c.seatsCollection(isMemberSeat).Where(WorkNameDocProperty, "!=", "").Documents(ctx)
	return getDocDataFromIterator[SeatDoc](iter)
}

func (c *FirestoreControllerImplements) UpdateUserLastEnteredDate(ctx context.Context, userId string, enteredDate time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: LastEnteredDocProperty, Value: enteredDate},
	})
}

func (c *FirestoreControllerImplements) UpdateUserLastExitedDate(ctx context.Context, userId string, exitedDate time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: LastExitedDocProperty, Value: exitedDate},
	})
}

func (c *FirestoreControllerImplements) UpdateUserRankVisible(ctx context.Context, userId string,
	rankVisible bool) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: RankVisibleDocProperty, Value: rankVisible},
	})
}

func (c *FirestoreControllerImplements) UpdateUserDefaultStudyMin(ctx context.Context, userId string, defaultStudyMin int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: DefaultStudyMinDocProperty, Value: defaultStudyMin},
	})
}

func (c *FirestoreControllerImplements) UpdateUserFavoriteColor(ctx context.Context, userId string, colorCode string) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: FavoriteColorDocProperty, Value: colorCode},
	})
}

func (c *FirestoreControllerImplements) ReadUser(ctx context.Context, userId string) (UserDoc, error) {
	ref := c.usersCollection().Doc(userId)
	doc, err := c.get(ctx, ref)
	if err != nil {
		return UserDoc{}, err
	}
	userData := UserDoc{}
	if err = doc.DataTo(&userData); err != nil {
		return UserDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return userData, nil
}

func (c *FirestoreControllerImplements) UpdateUserTotalTime(
	ctx context.Context,
	userId string,
	newTotalTimeSec int,
	newDailyTotalTimeSec int,
) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: newDailyTotalTimeSec},
		{Path: TotalStudySecDocProperty, Value: newTotalTimeSec},
	})
}

func (c *FirestoreControllerImplements) UpdateUserRankPoint(ctx context.Context, userId string, rp int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: RankPointDocProperty, Value: rp},
	})
}

func (c *FirestoreControllerImplements) UpdateUserLastRPProcessed(ctx context.Context, userId string, date time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: LastRPProcessedDocProperty, Value: date},
	})
}

func (c *FirestoreControllerImplements) UpdateLiveChatId(ctx context.Context, liveChatId string) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, ref, []firestore.Update{
		{Path: LiveChatIdDocProperty, Value: liveChatId},
	})
}

func (c *FirestoreControllerImplements) CreateUser(ctx context.Context, userId string, userData UserDoc) error {
	ref := c.usersCollection().Doc(userId)
	return c.create(ctx, ref, userData)
}

func (c *FirestoreControllerImplements) UpdateWorkNameTrend(ctx context.Context, workNameTrend WorkNameTrendDoc) error {
	ref := c.workNameTrendCollection().Doc(WorkNameTrendDocName)
	return c.set(ctx, ref, workNameTrend)
}

func (c *FirestoreControllerImplements) GetAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
	return c.usersCollection().DocumentRefs(ctx).GetAll()
}

func (c *FirestoreControllerImplements) GetAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator {
	return c.usersCollection().Where(DailyTotalStudySecDocProperty, "!=", 0).Documents(ctx)
}

func (c *FirestoreControllerImplements) ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error {
	_, err := userRef.Update(ctx, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: 0},
	})
	return err
}

func (c *FirestoreControllerImplements) UpdateLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastResetDailyTotalStudySecDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreControllerImplements) UpdateLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastLongTimeSittingCheckedDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreControllerImplements) UpdateLastTransferCollectionHistoryBigquery(ctx context.Context,
	timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastTransferCollectionHistoryBigqueryDocProperty, Value: timestamp},
	})
	return err
}

func (c *FirestoreControllerImplements) UpdateDesiredMaxSeats(ctx context.Context,
	desiredMaxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, ref, []firestore.Update{
		{Path: DesiredMaxSeatsDocProperty, Value: desiredMaxSeats},
	})
}
func (c *FirestoreControllerImplements) UpdateDesiredMemberMaxSeats(ctx context.Context,
	desiredMemberMaxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, ref, []firestore.Update{
		{Path: DesiredMemberMaxSeatsDocProperty, Value: desiredMemberMaxSeats},
	})
}

func (c *FirestoreControllerImplements) UpdateMaxSeats(ctx context.Context, maxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, ref, []firestore.Update{
		{Path: MaxSeatsDocProperty, Value: maxSeats},
	})
}
func (c *FirestoreControllerImplements) UpdateMemberMaxSeats(ctx context.Context, memberMaxSeats int) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, ref, []firestore.Update{
		{Path: MemberMaxSeatsDocProperty, Value: memberMaxSeats},
	})
}

func (c *FirestoreControllerImplements) UpdateAccessTokenOfChannelCredential(ctx context.Context, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, ref, []firestore.Update{
		{Path: YoutubeChannelAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeChannelExpirationDate, Value: expireDate},
	})
}

func (c *FirestoreControllerImplements) UpdateAccessTokenOfBotCredential(ctx context.Context, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, ref, []firestore.Update{
		{Path: YoutubeBotAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeBotExpirationDateDocProperty, Value: expireDate},
	})
}

func (c *FirestoreControllerImplements) CreateSeat(ctx context.Context, seat SeatDoc, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seat.SeatId))
	return c.create(ctx, ref, seat)
}

func (c *FirestoreControllerImplements) UpdateSeat(ctx context.Context, seat SeatDoc, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seat.SeatId))
	return c.set(ctx, ref, seat)
}

func (c *FirestoreControllerImplements) DeleteSeat(ctx context.Context, seatId int, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seatId))
	return c.delete(ctx, ref)
}

func (c *FirestoreControllerImplements) CreateLiveChatHistoryDoc(ctx context.Context,
	liveChatHistoryDoc LiveChatHistoryDoc) error {
	ref := c.liveChatHistoryCollection().NewDoc()
	return c.create(ctx, ref, liveChatHistoryDoc)
}

func (c *FirestoreControllerImplements) Get500LiveChatHistoryDocIdsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return c.liveChatHistoryCollection().Where(PublishedAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreControllerImplements) CreateUserActivityDoc(ctx context.Context, activity UserActivityDoc) error {
	ref := c.userActivitiesCollection().NewDoc()
	return c.create(ctx, ref, activity)
}

func (c *FirestoreControllerImplements) Get500UserActivityDocIdsBeforeDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreControllerImplements) Get500OrderHistoryDocIdsBeforeDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.orderHistoryCollection().Where(OrderedAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreControllerImplements) GetAllUserActivityDocIdsAfterDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreControllerImplements) GetAllUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=",
		date).Where(UserIdDocProperty, "==", userId).Where(SeatIdDocProperty, "==", seatId).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).OrderBy(TakenAtDocProperty,
		firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[UserActivityDoc](iter)
}

func (c *FirestoreControllerImplements) GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIdDocProperty, "==", userId).
		Where(SeatIdDocProperty, "==", seatId).Where(ActivityTypeDocProperty, "==", EnterRoomActivity).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[UserActivityDoc](iter)
}

func (c *FirestoreControllerImplements) GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIdDocProperty, "==", userId).
		Where(SeatIdDocProperty, "==", seatId).Where(ActivityTypeDocProperty, "==", ExitRoomActivity).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[UserActivityDoc](iter)
}

// GetUsersActiveAfterDate date以後に入室したことのあるuserを全て取得
func (c *FirestoreControllerImplements) GetUsersActiveAfterDate(ctx context.Context, date time.Time) *firestore.DocumentIterator {
	return c.usersCollection().Where(LastEnteredDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreControllerImplements) UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(
	ctx context.Context, userId string, isContinuousActive bool, currentActivityStateStarted time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: IsContinuousActiveDocProperty, Value: isContinuousActive},
		{Path: CurrentActivityStateStartedDocProperty, Value: currentActivityStateStarted},
	})
}

func (c *FirestoreControllerImplements) UpdateUserLastPenaltyImposedDays(ctx context.Context, userId string, lastPenaltyImposedDays int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: LastPenaltyImposedDaysDocProperty, Value: lastPenaltyImposedDays},
	})
}

func (c *FirestoreControllerImplements) UpdateUserRPAndLastPenaltyImposedDays(ctx context.Context, userId string,
	newRP int, newLastPenaltyImposedDays int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, ref, []firestore.Update{
		{Path: RankPointDocProperty, Value: newRP},
		{Path: LastPenaltyImposedDaysDocProperty, Value: newLastPenaltyImposedDays},
	})
}

func (c *FirestoreControllerImplements) ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	iter := collection.Where(SeatIdDocProperty, "==", seatId).Where(UserIdDocProperty, "==", userId).Documents(ctx)
	return getDocDataFromIterator[SeatLimitDoc](iter)
}

func (c *FirestoreControllerImplements) ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	iter := collection.Where(SeatIdDocProperty, "==", seatId).Where(UserIdDocProperty, "==", userId).Documents(ctx)
	return getDocDataFromIterator[SeatLimitDoc](iter)
}

func (c *FirestoreControllerImplements) CreateSeatLimitInWHITEList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error {
	var ref *firestore.DocumentRef
	if isMemberSeat {
		ref = c.memberSeatLimitsWHITEListCollection().NewDoc()
	} else {
		ref = c.generalSeatLimitsWHITEListCollection().NewDoc()
	}
	return c.createSeatLimit(ctx, ref, seatId, userId, createdAt, until)
}

func (c *FirestoreControllerImplements) CreateSeatLimitInBLACKList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error {
	var ref *firestore.DocumentRef
	if isMemberSeat {
		ref = c.memberSeatLimitsBLACKListCollection().NewDoc()
	} else {
		ref = c.generalSeatLimitsBLACKListCollection().NewDoc()
	}
	return c.createSeatLimit(ctx, ref, seatId, userId, createdAt, until)
}

func (c *FirestoreControllerImplements) createSeatLimit(ctx context.Context, ref *firestore.DocumentRef, seatId int, userId string, createdAt, until time.Time) error {
	data := SeatLimitDoc{
		SeatId:    seatId,
		UserId:    userId,
		CreatedAt: createdAt,
		Until:     until,
	}
	return c.create(ctx, ref, data)
}

// Get500SeatLimitsAfterUntilInWHITEList returns all seat limit docs whose `until` is after `thresholdTime`.
func (c *FirestoreControllerImplements) Get500SeatLimitsAfterUntilInWHITEList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) *firestore.DocumentIterator {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	return collection.Where(UntilDocProperty, "<", thresholdTime).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

// Get500SeatLimitsAfterUntilInBLACKList returns all seat limit docs whose `until` is after `thresholdTime`.
func (c *FirestoreControllerImplements) Get500SeatLimitsAfterUntilInBLACKList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) *firestore.DocumentIterator {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	return collection.Where(UntilDocProperty, "<", thresholdTime).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreControllerImplements) DeleteSeatLimitInWHITEList(ctx context.Context, docId string, isMemberSeat bool) error {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	ref := collection.Doc(docId)
	return c.delete(ctx, ref)
}

func (c *FirestoreControllerImplements) DeleteSeatLimitInBLACKList(ctx context.Context, docId string, isMemberSeat bool) error {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	ref := collection.Doc(docId)
	return c.delete(ctx, ref)
}

func (c *FirestoreControllerImplements) ReadAllMenuDocsOrderByCode(ctx context.Context) ([]MenuDoc, error) {
	iter := c.menuCollection().OrderBy(CodeDocProperty, firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[MenuDoc](iter)
}

func (c *FirestoreControllerImplements) CountUserOrdersOfTheDay(ctx context.Context, userId string, date time.Time) (int64, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1)
	query := c.orderHistoryCollection().
		Where(UserIdDocProperty, "==", userId).
		Where(OrderedAtDocProperty, ">=", start).
		Where(OrderedAtDocProperty, "<", end)
	aggregationQuery := query.NewAggregationQuery().WithCount("all")
	results, err := aggregationQuery.Get(ctx)
	if err != nil {
		return -1, err
	}

	count, ok := results["all"]
	if !ok {
		return -1, errors.New("firestore: couldn't get alias for COUNT from results")
	}

	countValue := count.(*firestorepb.Value)

	return countValue.GetIntegerValue(), nil
}

func (c *FirestoreControllerImplements) CreateOrderHistoryDoc(ctx context.Context, orderHistoryDoc OrderHistoryDoc) error {
	ref := c.orderHistoryCollection().NewDoc()
	return c.create(ctx, ref, orderHistoryDoc)
}

func getDocDataFromIterator[T any](iter *firestore.DocumentIterator) ([]T, error) {
	docs := make([]T, 0) // jsonになったときにnullとならないように。
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return []T{}, fmt.Errorf("in iter.Next(): %w", err)
		}
		var data T
		if err := doc.DataTo(&data); err != nil {
			return []T{}, fmt.Errorf("in doc.DataTo: %w", err)
		}
		docs = append(docs, data)
	}
	return docs, nil
}
