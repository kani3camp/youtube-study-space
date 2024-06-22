package myfirestore

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreController struct {
	FirestoreClient *firestore.Client
}

func NewFirestoreController(ctx context.Context, clientOption option.ClientOption) (*FirestoreController, error) {
	client, err := firestore.NewClient(ctx, firestore.DetectProjectID, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in firestore.NewClient: %w", err)
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

func (c *FirestoreController) create(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data interface{}) error {
	if tx != nil {
		return tx.Create(ref, data)
	} else {
		_, err := ref.Create(ctx, data)
		return err
	}
}

func (c *FirestoreController) bulkCreate(bulkWriter *firestore.BulkWriter, ref *firestore.DocumentRef, data interface{}) error {
	_, err := bulkWriter.Create(ref, data)
	return err
}

func (c *FirestoreController) bulkUpdate(bulkWriter *firestore.BulkWriter, ref *firestore.DocumentRef, data []firestore.Update) error {
	_, err := bulkWriter.Update(ref, data)
	return err
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

// delete deletes the document. If the document doesn't exist, it does nothing and returns no error.
func (c *FirestoreController) delete(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, opts ...firestore.Precondition) error {
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

func (c *FirestoreController) workHistoryCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(WorkHistory)
}

func (c *FirestoreController) dailyWorkHistoryCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(DailyWorkHistory)
}

func (c *FirestoreController) seatsCollection(isMemberSeat bool) *firestore.CollectionRef {
	if isMemberSeat {
		return c.memberSeatsCollection()
	} else {
		return c.generalSeatsCollection()
	}
}

func (c *FirestoreController) generalSeatsCollection() *firestore.CollectionRef {
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

func (c *FirestoreController) generalSeatLimitsBLACKListCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(SeatLimitsBlackList)
}

func (c *FirestoreController) generalSeatLimitsWHITEListCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(SeatLimitsWhiteList)
}

func (c *FirestoreController) memberSeatLimitsBLACKListCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(MemberSeatLimitsBlackList)
}

func (c *FirestoreController) memberSeatLimitsWHITEListCollection() *firestore.CollectionRef {
	return c.FirestoreClient.Collection(MemberSeatLimitsWhiteList)
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
		return CredentialsConfigDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
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
		return ConstantsConfigDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return constantsConfig, nil
}

func (c *FirestoreController) ReadLiveChatId(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("in ReadCredentialsConfig: %w", err)
	}
	return credentialsDoc.YoutubeLiveChatId, nil
}

func (c *FirestoreController) ReadNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("in ReadCredentialsConfig: %w", err)
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
	iter := c.generalSeatsCollection().Documents(ctx)
	return getDocsFromIterator[SeatDoc](iter)
}
func (c *FirestoreController) ReadMemberSeats(ctx context.Context) ([]SeatDoc, error) {
	iter := c.memberSeatsCollection().Documents(ctx)
	return getDocsFromIterator[SeatDoc](iter)
}

func (c *FirestoreController) ReadSeatsExpiredUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error) {
	iter := c.seatsCollection(isMemberSeat).Where(UntilDocProperty, "<", thresholdTime).Documents(ctx)
	return getDocsFromIterator[SeatDoc](iter)
}

func (c *FirestoreController) ReadSeatsExpiredBreakUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error) {
	iter := c.seatsCollection(isMemberSeat).Where(StateDocProperty, "==", BreakState).Where(CurrentStateUntilDocProperty, "<", thresholdTime).Documents(ctx)
	return getDocsFromIterator[SeatDoc](iter)
}

func (c *FirestoreController) ReadSeat(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) (SeatDoc, error) {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seatId))
	doc, err := c.get(ctx, tx, ref)
	if err != nil {
		return SeatDoc{}, err // NotFoundの場合もerrに含まれる
	}
	var seatDoc SeatDoc
	err = doc.DataTo(&seatDoc)
	if err != nil {
		return SeatDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return seatDoc, nil
}

func (c *FirestoreController) ReadSeatWithUserId(ctx context.Context, userId string, isMemberSeat bool) (SeatDoc, error) {
	docs, err := c.seatsCollection(isMemberSeat).Where(UserIdDocProperty, "==", userId).Documents(ctx).GetAll()
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
			return SeatDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
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

func (c *FirestoreController) CreateWorkHistory(ctx context.Context, tx *firestore.Transaction, userId string, seatId int, isMemberSeat bool, startedAt time.Time, workName string, createdAt time.Time) (string, error) {
	ref := c.workHistoryCollection().NewDoc()
	doc := WorkHistoryDoc{
		UserId:       userId,
		SeatId:       seatId,
		IsMemberSeat: isMemberSeat,
		StartedAt:    startedAt,
		EndedAt:      time.Time{},
		WorkName:     workName,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
	return ref.ID, c.create(ctx, tx, ref, doc)
}

func (c *FirestoreController) UpdateWorkHistoryEndedAt(ctx context.Context, tx *firestore.Transaction, workHistoryId string, endedAt, updatedAt time.Time) error {
	ref := c.workHistoryCollection().Doc(workHistoryId)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: EndedAtDocProperty, Value: endedAt},
		{Path: UpdatedAtDocProperty, Value: updatedAt},
	})
}

// GetWorkHistoriesByEndedAt endedAtFrom〜endedAtToの間に確定した全ての作業履歴を取得する
func (c *FirestoreController) GetWorkHistoriesByEndedAt(ctx context.Context, endedAtFrom, endedAtTo time.Time) ([]WorkHistoryDoc, error) {
	iter := c.workHistoryCollection().Where(EndedAtDocProperty, ">=", endedAtFrom).Where(EndedAtDocProperty, "<=", endedAtTo).Documents(ctx)
	return getDocsFromIterator[WorkHistoryDoc](iter)
}

func (c *FirestoreController) CreateOrUpdateDailyWorkHistory(ctx context.Context, bulkWriter *firestore.BulkWriter, dateString string, userId string, workSec int, timeZoneOffset string, updatedAt time.Time) error {
	ref := c.dailyWorkHistoryCollection().Doc(dateString + "-" + userId)
	snapshot, err := ref.Get(ctx)
	var exists bool
	if err != nil {
		if status.Code(err) == codes.NotFound {
			exists = false
		} else {
			return err
		}
	} else if snapshot.Exists() {
		exists = true
	} else {
		exists = false
	}
	if exists {
		var doc DailyWorkHistoryDoc
		err := snapshot.DataTo(&doc)
		if err != nil {
			return fmt.Errorf("in snapshot.DataTo: %w", err)
		}
		return c.bulkUpdate(bulkWriter, ref, []firestore.Update{
			{Path: WorkSecDocProperty, Value: doc.WorkSec + workSec},
			{Path: TimezoneOffsetDocProperty, Value: timeZoneOffset},
			{Path: UpdatedAtDocProperty, Value: updatedAt},
		})
	} else {
		doc := DailyWorkHistoryDoc{
			UserId:         userId,
			Date:           dateString,
			WorkSec:        workSec,
			TimezoneOffset: timeZoneOffset,
			CreatedAt:      updatedAt,
			UpdatedAt:      updatedAt,
		}
		return c.bulkCreate(bulkWriter, ref, doc)
	}
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
		return UserDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
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

func (c *FirestoreController) CreateUser(ctx context.Context, tx *firestore.Transaction, userId string, userData UserDoc) error {
	ref := c.usersCollection().Doc(userId)
	return c.create(ctx, tx, ref, userData)
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

func (c *FirestoreController) UpdateLastDailyWorkHistoryTargetDateTime(ctx context.Context, tx *firestore.Transaction, datetime time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: LastDailyWorkHistoryTargetDateDocProperty, Value: datetime},
	})
}

func (c *FirestoreController) CreateSeat(tx *firestore.Transaction, seat SeatDoc, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seat.SeatId))
	return tx.Create(ref, seat)
}

func (c *FirestoreController) UpdateSeat(ctx context.Context, tx *firestore.Transaction, seat SeatDoc, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seat.SeatId))
	return c.set(ctx, tx, ref, seat)
}

func (c *FirestoreController) DeleteSeat(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seatId))
	return c.delete(ctx, tx, ref)
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

func (c *FirestoreController) CreateUserActivityDoc(ctx context.Context, tx *firestore.Transaction, activity UserActivityDoc) error {
	ref := c.userActivitiesCollection().NewDoc()
	return c.create(ctx, tx, ref, activity)
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
	date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().
		Where(TakenAtDocProperty, ">=", date).
		Where(UserIdDocProperty, "==", userId).
		Where(SeatIdDocProperty, "==", seatId).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).
		OrderBy(TakenAtDocProperty, firestore.Asc).
		Documents(ctx)
	return getDocsFromIterator[UserActivityDoc](iter)
}

func (c *FirestoreController) GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIdDocProperty, "==", userId).
		Where(SeatIdDocProperty, "==", seatId).Where(ActivityTypeDocProperty, "==", EnterRoomActivity).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getDocsFromIterator[UserActivityDoc](iter)
}

func (c *FirestoreController) GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIdDocProperty, "==", userId).
		Where(SeatIdDocProperty, "==", seatId).Where(ActivityTypeDocProperty, "==", ExitRoomActivity).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getDocsFromIterator[UserActivityDoc](iter)
}

// GetUsersActiveAfterDate date以後に入室したことのあるuserを全て取得
func (c *FirestoreController) GetUsersActiveAfterDate(ctx context.Context, date time.Time) *firestore.DocumentIterator {
	return c.usersCollection().Where(LastEnteredDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreController) UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(
	ctx context.Context, tx *firestore.Transaction, userId string, isContinuousActive bool, currentActivityStateStarted time.Time) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: IsContinuousActiveDocProperty, Value: isContinuousActive},
		{Path: CurrentActivityStateStartedDocProperty, Value: currentActivityStateStarted},
	})
}

func (c *FirestoreController) UpdateUserLastPenaltyImposedDays(ctx context.Context, tx *firestore.Transaction, userId string, lastPenaltyImposedDays int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: LastPenaltyImposedDaysDocProperty, Value: lastPenaltyImposedDays},
	})
}

func (c *FirestoreController) UpdateUserRPAndLastPenaltyImposedDays(ctx context.Context, tx *firestore.Transaction, userId string,
	newRP int, newLastPenaltyImposedDays int) error {
	ref := c.usersCollection().Doc(userId)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: RankPointDocProperty, Value: newRP},
		{Path: LastPenaltyImposedDaysDocProperty, Value: newLastPenaltyImposedDays},
	})
}

func (c *FirestoreController) ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	iter := collection.Where(SeatIdDocProperty, "==", seatId).Where(UserIdDocProperty, "==", userId).Documents(ctx)
	return getDocsFromIterator[SeatLimitDoc](iter)
}

func (c *FirestoreController) ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	iter := collection.Where(SeatIdDocProperty, "==", seatId).Where(UserIdDocProperty, "==", userId).Documents(ctx)
	return getDocsFromIterator[SeatLimitDoc](iter)
}

func getDocsFromIterator[T any](iter *firestore.DocumentIterator) ([]T, error) {
	list := make([]T, 0) // jsonになったときにnullとならないように
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return []T{}, fmt.Errorf("in iter.Next(): %w", err)
		}
		var data T
		err = doc.DataTo(&data)
		if err != nil {
			return []T{}, fmt.Errorf("in doc.DataTo: %w", err)
		}
		list = append(list, data)
	}
	return list, nil
}

func (c *FirestoreController) CreateSeatLimitInWHITEList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error {
	var ref *firestore.DocumentRef
	if isMemberSeat {
		ref = c.memberSeatLimitsWHITEListCollection().NewDoc()
	} else {
		ref = c.generalSeatLimitsWHITEListCollection().NewDoc()
	}
	return c.createSeatLimit(ctx, ref, seatId, userId, createdAt, until)
}

func (c *FirestoreController) CreateSeatLimitInBLACKList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error {
	var ref *firestore.DocumentRef
	if isMemberSeat {
		ref = c.memberSeatLimitsBLACKListCollection().NewDoc()
	} else {
		ref = c.generalSeatLimitsBLACKListCollection().NewDoc()
	}
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
func (c *FirestoreController) Get500SeatLimitsAfterUntilInWHITEList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) *firestore.DocumentIterator {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	return collection.Where(UntilDocProperty, "<", thresholdTime).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

// Get500SeatLimitsAfterUntilInBLACKList returns all seat limit docs whose `until` is after `thresholdTime`.
func (c *FirestoreController) Get500SeatLimitsAfterUntilInBLACKList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) *firestore.DocumentIterator {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	return collection.Where(UntilDocProperty, "<", thresholdTime).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreController) DeleteSeatLimitInWHITEList(ctx context.Context, docId string, isMemberSeat bool) error {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	ref := collection.Doc(docId)
	return c.delete(ctx, nil, ref)
}

func (c *FirestoreController) DeleteSeatLimitInBLACKList(ctx context.Context, docId string, isMemberSeat bool) error {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	ref := collection.Doc(docId)
	return c.delete(ctx, nil, ref)
}

func (c *FirestoreController) ReadDailyWorkHistoryOfDate(ctx context.Context, tx *firestore.Transaction, userId string, date time.Time) (DailyWorkHistoryDoc, error) {
	docId := date.Format("2006-01-02") + "-" + userId
	ref := c.dailyWorkHistoryCollection().Doc(docId)
	doc, err := c.get(ctx, tx, ref)
	if err != nil {
		return DailyWorkHistoryDoc{}, err
	}
	var workHistoryData WorkHistoryDoc
	err = doc.DataTo(&workHistoryData)
	if err != nil {
		return DailyWorkHistoryDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return DailyWorkHistoryDoc{}, nil
}

// ReadDailyWorkHistoryBetweenDates returns all work history docs whose `date` is between `from` and `until`. `from` is inclusive and `until` is exclusive.
func (c *FirestoreController) ReadDailyWorkHistoryBetweenDates(ctx context.Context, userId string, from, until time.Time) ([]DailyWorkHistoryDoc, error) {
	iter := c.dailyWorkHistoryCollection().
		Where(UserIdDocProperty, "==", userId).
		Where(DateDocProperty, ">=", from.Format("2006-01-02")).
		Where(DateDocProperty, "<", until.Format("2006-01-02")).
		Documents(ctx)
	return getDocsFromIterator[DailyWorkHistoryDoc](iter)
}
