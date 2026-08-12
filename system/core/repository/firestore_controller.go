package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"

	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreControllerImplements struct {
	firestoreClient DBClient
}

func NewFirestoreController(ctx context.Context, clientOption option.ClientOption) (*FirestoreControllerImplements, error) {
	return NewFirestoreControllerForProject(ctx, firestore.DetectProjectID, clientOption)
}

func NewFirestoreControllerForProject(ctx context.Context, projectID string, clientOptions ...option.ClientOption) (*FirestoreControllerImplements, error) {
	client, err := firestore.NewClient(ctx, projectID, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("in firestore.NewClient: %w", err)
	}

	return &FirestoreControllerImplements{
		firestoreClient: client,
	}, nil
}

func (c *FirestoreControllerImplements) FirestoreClient() DBClient {
	return c.firestoreClient
}

func (c *FirestoreControllerImplements) get(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef) (*firestore.DocumentSnapshot, error) {
	if tx != nil {
		doc, err := tx.Get(ref)
		if err != nil {
			return nil, fmt.Errorf("get document in transaction: %w", err)
		}
		return doc, nil
	}
	doc, err := ref.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	return doc, nil
}

func (c *FirestoreControllerImplements) create(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data interface{}) error {
	if tx != nil {
		if err := tx.Create(ref, data); err != nil {
			return fmt.Errorf("create document in transaction: %w", err)
		}
		return nil
	}
	if _, err := ref.Create(ctx, data); err != nil {
		return fmt.Errorf("create document: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) set(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data interface{}, opts ...firestore.SetOption) error {
	if tx != nil {
		if err := tx.Set(ref, data, opts...); err != nil {
			return fmt.Errorf("set document in transaction: %w", err)
		}
		return nil
	}
	if _, err := ref.Set(ctx, data, opts...); err != nil {
		return fmt.Errorf("set document: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) update(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, data []firestore.Update, opts ...firestore.Precondition) error {
	if tx != nil {
		return updateInTransaction(tx, ref, data, opts...)
	}
	if _, err := ref.Update(ctx, data, opts...); err != nil {
		return fmt.Errorf("update document: %w", err)
	}
	return nil
}

func updateInTransaction(tx *firestore.Transaction, ref *firestore.DocumentRef, data []firestore.Update, opts ...firestore.Precondition) error {
	if err := tx.Update(ref, data, opts...); err != nil {
		return fmt.Errorf("update document in transaction: %w", err)
	}
	return nil
}

// delete deletes the document. If the document doesn't exist, it does nothing and returns no error.
func (c *FirestoreControllerImplements) delete(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef, opts ...firestore.Precondition) error {
	if tx != nil {
		if err := tx.Delete(ref, opts...); err != nil {
			return fmt.Errorf("delete document in transaction: %w", err)
		}
		return nil
	}
	if _, err := ref.Delete(ctx, opts...); err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) configCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(CONFIG)
}

func (c *FirestoreControllerImplements) publicConfigCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(PublicConfig)
}

func (c *FirestoreControllerImplements) usersCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(USERS)
}

func (c *FirestoreControllerImplements) seatsCollection(isMemberSeat bool) *firestore.CollectionRef {
	if isMemberSeat {
		return c.memberSeatsCollection()
	} else {
		return c.generalSeatsCollection()
	}
}

func (c *FirestoreControllerImplements) menuCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(MENU)
}

func (c *FirestoreControllerImplements) orderHistoryCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(OrderHistory)
}

func (c *FirestoreControllerImplements) generalSeatsCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(SEATS)
}

func (c *FirestoreControllerImplements) memberSeatsCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(MemberSeats)
}

func (c *FirestoreControllerImplements) liveChatHistoryCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(LiveChatHistory)
}

func (c *FirestoreControllerImplements) userActivitiesCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(UserActivities)
}

func (c *FirestoreControllerImplements) workSegmentsCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(WorkSegments)
}

func (c *FirestoreControllerImplements) generalSeatLimitsBLACKListCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(SeatLimitsBlackList)
}

func (c *FirestoreControllerImplements) generalSeatLimitsWHITEListCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(SeatLimitsWhiteList)
}

func (c *FirestoreControllerImplements) memberSeatLimitsBLACKListCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(MemberSeatLimitsBlackList)
}

func (c *FirestoreControllerImplements) memberSeatLimitsWHITEListCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(MemberSeatLimitsWhiteList)
}

func (c *FirestoreControllerImplements) workNameTrendCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(WorkNameTrend)
}

func (c *FirestoreControllerImplements) DeleteDocRef(ctx context.Context, tx *firestore.Transaction,
	ref *firestore.DocumentRef,
) error {
	return c.delete(ctx, tx, ref)
}

func (c *FirestoreControllerImplements) ReadCredentialsConfig(ctx context.Context, tx *firestore.Transaction) (CredentialsConfigDoc, error) {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	doc, err := c.get(ctx, tx, ref)
	if err != nil {
		return CredentialsConfigDoc{}, err
	}
	var credentialsData CredentialsConfigDoc
	if err := doc.DataTo(&credentialsData); err != nil {
		return CredentialsConfigDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return credentialsData, nil
}

func (c *FirestoreControllerImplements) ReadSystemConstantsConfig(ctx context.Context, tx *firestore.Transaction) (ConstantsConfigDoc, error) {
	constantsConfig, err := c.readInternalSystemConstantsConfig(ctx, tx)
	if err != nil {
		return ConstantsConfigDoc{}, err
	}

	monitorConfig, err := c.readMonitorPublicConfig(ctx, tx)
	if err != nil {
		return ConstantsConfigDoc{}, fmt.Errorf("read monitor public config: %w", err)
	}
	monitorConfig.applyTo(&constantsConfig)
	return constantsConfig, nil
}

func (c *FirestoreControllerImplements) readInternalSystemConstantsConfig(ctx context.Context, tx *firestore.Transaction) (ConstantsConfigDoc, error) {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	doc, err := c.get(ctx, tx, ref)
	if err != nil {
		return ConstantsConfigDoc{}, err
	}
	var constantsConfig ConstantsConfigDoc
	if err := doc.DataTo(&constantsConfig); err != nil {
		return ConstantsConfigDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return constantsConfig, nil
}

func (c *FirestoreControllerImplements) readMonitorPublicConfig(ctx context.Context, tx *firestore.Transaction) (MonitorPublicConfigDoc, error) {
	ref := c.publicConfigCollection().Doc(MonitorPublicConfigDocName)
	doc, err := c.get(ctx, tx, ref)
	if err != nil {
		return MonitorPublicConfigDoc{}, err
	}
	var monitorConfig MonitorPublicConfigDoc
	if err := doc.DataTo(&monitorConfig); err != nil {
		return MonitorPublicConfigDoc{}, fmt.Errorf("decode monitor public config: %w", err)
	}
	return monitorConfig, nil
}

// PrepareMonitorPublicConfigBootstrap reads the five legacy values from config/constants.
// It intentionally does not write any data; CreateMonitorPublicConfig performs the guarded create.
func (c *FirestoreControllerImplements) PrepareMonitorPublicConfigBootstrap(ctx context.Context) (MonitorPublicConfigDoc, error) {
	constants, err := c.readInternalSystemConstantsConfig(ctx, nil)
	if err != nil {
		return MonitorPublicConfigDoc{}, fmt.Errorf("read legacy system constants: %w", err)
	}
	return monitorPublicConfigFromConstants(constants), nil
}

// CreateMonitorPublicConfig creates public-config/monitor without overwriting an existing document.
func (c *FirestoreControllerImplements) CreateMonitorPublicConfig(ctx context.Context, monitorConfig MonitorPublicConfigDoc) error {
	ref := c.publicConfigCollection().Doc(MonitorPublicConfigDocName)
	if err := c.create(ctx, nil, ref, monitorConfig); err != nil {
		return fmt.Errorf("create monitor public config: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) ReadLiveChatID(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("in ReadCredentialsConfig: %w", err)
	}
	return credentialsDoc.YoutubeLiveChatID, nil
}

func (c *FirestoreControllerImplements) ReadNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error) {
	credentialsDoc, err := c.ReadCredentialsConfig(ctx, tx)
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
		return fmt.Errorf("update next page token: %w", err)
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

func (c *FirestoreControllerImplements) ReadSeat(ctx context.Context, tx *firestore.Transaction, seatID int, isMemberSeat bool) (SeatDoc, error) {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seatID))
	doc, err := c.get(ctx, tx, ref)
	if err != nil {
		return SeatDoc{}, err // NotFoundの場合もerrに含まれる
	}
	var seatDoc SeatDoc
	if err := doc.DataTo(&seatDoc); err != nil {
		return SeatDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
	}
	return seatDoc, nil
}

func (c *FirestoreControllerImplements) ReadSeatWithUserID(ctx context.Context, userID string, isMemberSeat bool) (SeatDoc, error) {
	docs, err := c.seatsCollection(isMemberSeat).Where(UserIDDocProperty, "==", userID).Documents(ctx).GetAll()
	if err != nil {
		return SeatDoc{}, fmt.Errorf("query seat by user ID: %w", err)
	}
	if len(docs) >= 2 {
		return SeatDoc{}, errors.New("There are more than two seats with the user id = " + userID + " !!")
	}
	if len(docs) == 1 {
		var seatDoc SeatDoc
		if err := docs[0].DataTo(&seatDoc); err != nil {
			return SeatDoc{}, fmt.Errorf("in doc.DataTo: %w", err)
		}
		return seatDoc, nil
	}
	return SeatDoc{}, status.Errorf(codes.NotFound, "%s not found", "the document with user id = "+userID)
}

func (c *FirestoreControllerImplements) ReadActiveWorkNameSeats(ctx context.Context, isMemberSeat bool) ([]SeatDoc, error) {
	iter := c.seatsCollection(isMemberSeat).Where(WorkNameDocProperty, "!=", "").Documents(ctx)
	return getDocDataFromIterator[SeatDoc](iter)
}

func (c *FirestoreControllerImplements) UpdateUserLastEnteredDate(tx *firestore.Transaction, userID string, enteredDate time.Time) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: LastEnteredDocProperty, Value: enteredDate},
	})
}

func (c *FirestoreControllerImplements) UpdateUserLastExitedDate(tx *firestore.Transaction, userID string, exitedDate time.Time) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: LastExitedDocProperty, Value: exitedDate},
	})
}

func (c *FirestoreControllerImplements) UpdateUserRankVisible(tx *firestore.Transaction, userID string,
	rankVisible bool,
) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: RankVisibleDocProperty, Value: rankVisible},
	})
}

func (c *FirestoreControllerImplements) UpdateUserDefaultStudyMin(tx *firestore.Transaction, userID string, defaultStudyMin int) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: DefaultStudyMinDocProperty, Value: defaultStudyMin},
	})
}

func (c *FirestoreControllerImplements) UpdateUserFavoriteColor(tx *firestore.Transaction, userID string, colorCode string) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: FavoriteColorDocProperty, Value: colorCode},
	})
}

func (c *FirestoreControllerImplements) ReadUser(ctx context.Context, tx *firestore.Transaction, userID string) (UserDoc, error) {
	ref := c.usersCollection().Doc(userID)
	doc, err := c.get(ctx, tx, ref)
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
	tx *firestore.Transaction,
	userID string,
	newTotalTimeSec int,
	newDailyTotalTimeSec int,
) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: newDailyTotalTimeSec},
		{Path: TotalStudySecDocProperty, Value: newTotalTimeSec},
	})
}

func (c *FirestoreControllerImplements) UpdateUserRankPoint(tx *firestore.Transaction, userID string, rp int) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: RankPointDocProperty, Value: rp},
	})
}

func (c *FirestoreControllerImplements) UpdateUserLastRPProcessed(tx *firestore.Transaction, userID string, date time.Time) error {
	ref := c.usersCollection().Doc(userID)
	return updateInTransaction(tx, ref, []firestore.Update{
		{Path: LastRPProcessedDocProperty, Value: date},
	})
}

func (c *FirestoreControllerImplements) UpdateLiveChatID(ctx context.Context, tx *firestore.Transaction, liveChatID string) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: LiveChatIDDocProperty, Value: liveChatID},
	})
}

func (c *FirestoreControllerImplements) CreateUser(ctx context.Context, tx *firestore.Transaction, userID string, userData UserDoc) error {
	ref := c.usersCollection().Doc(userID)
	return c.create(ctx, tx, ref, userData)
}

func (c *FirestoreControllerImplements) UpdateWorkNameTrend(ctx context.Context, tx *firestore.Transaction, workNameTrend WorkNameTrendDoc) error {
	ref := c.workNameTrendCollection().Doc(WorkNameTrendDocName)
	return c.set(ctx, tx, ref, workNameTrend)
}

func (c *FirestoreControllerImplements) GetAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error) {
	refs, err := c.usersCollection().DocumentRefs(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("get all user document references: %w", err)
	}
	return refs, nil
}

func (c *FirestoreControllerImplements) GetAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator {
	return c.usersCollection().Where(DailyTotalStudySecDocProperty, "!=", 0).Documents(ctx)
}

func (c *FirestoreControllerImplements) ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error {
	_, err := userRef.Update(ctx, []firestore.Update{
		{Path: DailyTotalStudySecDocProperty, Value: 0},
	})
	if err != nil {
		return fmt.Errorf("reset daily study time: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) UpdateLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastResetDailyTotalStudySecDocProperty, Value: timestamp},
	})
	if err != nil {
		return fmt.Errorf("update last daily reset time: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) UpdateLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastLongTimeSittingCheckedDocProperty, Value: timestamp},
	})
	if err != nil {
		return fmt.Errorf("update last long-sitting check time: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) UpdateLastTransferCollectionHistoryBigquery(ctx context.Context,
	timestamp time.Time,
) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	_, err := ref.Update(ctx, []firestore.Update{
		{Path: LastTransferCollectionHistoryBigqueryDocProperty, Value: timestamp},
	})
	if err != nil {
		return fmt.Errorf("update last BigQuery transfer time: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) UpdateDesiredMaxSeats(ctx context.Context, tx *firestore.Transaction,
	desiredMaxSeats int,
) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: DesiredMaxSeatsDocProperty, Value: desiredMaxSeats},
	})
}

func (c *FirestoreControllerImplements) UpdateDesiredMemberMaxSeats(ctx context.Context, tx *firestore.Transaction,
	desiredMemberMaxSeats int,
) error {
	ref := c.configCollection().Doc(SystemConstantsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: DesiredMemberMaxSeatsDocProperty, Value: desiredMemberMaxSeats},
	})
}

func (c *FirestoreControllerImplements) UpdateMaxSeats(ctx context.Context, tx *firestore.Transaction, maxSeats int) error {
	ref := c.publicConfigCollection().Doc(MonitorPublicConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: MaxSeatsDocProperty, Value: maxSeats},
	})
}

func (c *FirestoreControllerImplements) UpdateMemberMaxSeats(ctx context.Context, tx *firestore.Transaction, memberMaxSeats int) error {
	ref := c.publicConfigCollection().Doc(MonitorPublicConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: MemberMaxSeatsDocProperty, Value: memberMaxSeats},
	})
}

func (c *FirestoreControllerImplements) UpdateAccessTokenOfChannelCredential(ctx context.Context, tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: YoutubeChannelAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeChannelExpirationDate, Value: expireDate},
	})
}

func (c *FirestoreControllerImplements) UpdateAccessTokenOfBotCredential(ctx context.Context, tx *firestore.Transaction, accessToken string, expireDate time.Time) error {
	ref := c.configCollection().Doc(CredentialsConfigDocName)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: YoutubeBotAccessTokenDocProperty, Value: accessToken},
		{Path: YoutubeBotExpirationDateDocProperty, Value: expireDate},
	})
}

func (c *FirestoreControllerImplements) CreateSeat(tx *firestore.Transaction, seat SeatDoc, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seat.SeatID))
	if err := tx.Create(ref, seat); err != nil {
		return fmt.Errorf("create seat in transaction: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) UpdateSeat(ctx context.Context, tx *firestore.Transaction, seat SeatDoc, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seat.SeatID))
	return c.set(ctx, tx, ref, seat)
}

func (c *FirestoreControllerImplements) DeleteSeat(ctx context.Context, tx *firestore.Transaction, seatID int, isMemberSeat bool) error {
	ref := c.seatsCollection(isMemberSeat).Doc(strconv.Itoa(seatID))
	return c.delete(ctx, tx, ref)
}

func (c *FirestoreControllerImplements) CreateLiveChatHistoryDoc(ctx context.Context, tx *firestore.Transaction,
	liveChatHistoryDoc LiveChatHistoryDoc,
) error {
	ref := c.liveChatHistoryCollection().NewDoc()
	return c.create(ctx, tx, ref, liveChatHistoryDoc)
}

func (c *FirestoreControllerImplements) Get500LiveChatHistoryDocIDsBeforeDate(ctx context.Context,
	date time.Time,
) *firestore.DocumentIterator {
	return c.liveChatHistoryCollection().Where(PublishedAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreControllerImplements) CreateUserActivityDoc(ctx context.Context, tx *firestore.Transaction, activity UserActivityDoc) error {
	ref := c.userActivitiesCollection().NewDoc()
	return c.create(ctx, tx, ref, activity)
}

func (c *FirestoreControllerImplements) Get500UserActivityDocIDsBeforeDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreControllerImplements) Get500OrderHistoryDocIDsBeforeDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.orderHistoryCollection().Where(OrderedAtDocProperty, "<",
		date).Limit(FirestoreWritesLimitPerRequest).Documents(ctx)
}

func (c *FirestoreControllerImplements) GetAllUserActivityDocIDsAfterDate(ctx context.Context, date time.Time,
) *firestore.DocumentIterator {
	return c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreControllerImplements) GetAllUserActivityDocIDsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userID string, seatID int, isMemberSeat bool,
) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=",
		date).Where(UserIDDocProperty, "==", userID).Where(SeatIDDocProperty, "==", seatID).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).OrderBy(TakenAtDocProperty,
		firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[UserActivityDoc](iter)
}

func (c *FirestoreControllerImplements) GetEnterRoomUserActivityDocIDsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userID string, seatID int, isMemberSeat bool,
) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIDDocProperty, "==", userID).
		Where(SeatIDDocProperty, "==", seatID).Where(ActivityTypeDocProperty, "==", EnterRoomActivity).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[UserActivityDoc](iter)
}

func (c *FirestoreControllerImplements) GetExitRoomUserActivityDocIDsAfterDateForUserAndSeat(ctx context.Context,
	date time.Time, userID string, seatID int, isMemberSeat bool,
) ([]UserActivityDoc, error) {
	iter := c.userActivitiesCollection().Where(TakenAtDocProperty, ">=", date).Where(UserIDDocProperty, "==", userID).
		Where(SeatIDDocProperty, "==", seatID).Where(ActivityTypeDocProperty, "==", ExitRoomActivity).
		Where(IsMemberSeatDocProperty, "==", isMemberSeat).
		OrderBy(TakenAtDocProperty, firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[UserActivityDoc](iter)
}

// GetUsersActiveAfterDate date以後に入室したことのあるuserを全て取得
func (c *FirestoreControllerImplements) GetUsersActiveAfterDate(ctx context.Context, date time.Time) *firestore.DocumentIterator {
	return c.usersCollection().Where(LastEnteredDocProperty, ">=", date).Documents(ctx)
}

func (c *FirestoreControllerImplements) CreateWorkSegmentDoc(ctx context.Context, tx *firestore.Transaction, workSegment WorkSegmentDoc) error {
	ref := c.workSegmentsCollection().NewDoc()
	return c.create(ctx, tx, ref, workSegment)
}

// ReadWorkStateSegmentsBySessionID returns work-state segments for the given session ID.
func (c *FirestoreControllerImplements) ReadWorkStateSegmentsBySessionID(ctx context.Context, sessionID string) ([]WorkSegmentDoc, error) {
	iter := c.workSegmentsCollection().
		Where(SessionIDDocProperty, "==", sessionID).
		Where(SegmentTypeDocProperty, "==", WorkState).
		Documents(ctx)
	return getDocDataFromIterator[WorkSegmentDoc](iter)
}

func (c *FirestoreControllerImplements) UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(
	ctx context.Context, tx *firestore.Transaction, userID string, isContinuousActive bool, currentActivityStateStarted time.Time,
) error {
	ref := c.usersCollection().Doc(userID)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: IsContinuousActiveDocProperty, Value: isContinuousActive},
		{Path: CurrentActivityStateStartedDocProperty, Value: currentActivityStateStarted},
	})
}

func (c *FirestoreControllerImplements) UpdateUserLastPenaltyImposedDays(ctx context.Context, tx *firestore.Transaction, userID string, lastPenaltyImposedDays int) error {
	ref := c.usersCollection().Doc(userID)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: LastPenaltyImposedDaysDocProperty, Value: lastPenaltyImposedDays},
	})
}

func (c *FirestoreControllerImplements) UpdateUserRPAndLastPenaltyImposedDays(ctx context.Context, tx *firestore.Transaction, userID string,
	newRP int, newLastPenaltyImposedDays int,
) error {
	ref := c.usersCollection().Doc(userID)
	return c.update(ctx, tx, ref, []firestore.Update{
		{Path: RankPointDocProperty, Value: newRP},
		{Path: LastPenaltyImposedDaysDocProperty, Value: newLastPenaltyImposedDays},
	})
}

func (c *FirestoreControllerImplements) ReadSeatLimitsWHITEListWithSeatIDAndUserID(ctx context.Context, seatID int, userID string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	iter := collection.Where(SeatIDDocProperty, "==", seatID).Where(UserIDDocProperty, "==", userID).Documents(ctx)
	return getDocDataFromIterator[SeatLimitDoc](iter)
}

func (c *FirestoreControllerImplements) ReadSeatLimitsBLACKListWithSeatIDAndUserID(ctx context.Context, seatID int, userID string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	iter := collection.Where(SeatIDDocProperty, "==", seatID).Where(UserIDDocProperty, "==", userID).Documents(ctx)
	return getDocDataFromIterator[SeatLimitDoc](iter)
}

func (c *FirestoreControllerImplements) CreateSeatLimitInWHITEList(ctx context.Context, seatID int, userID string, createdAt, until time.Time, isMemberSeat bool) error {
	var ref *firestore.DocumentRef
	if isMemberSeat {
		ref = c.memberSeatLimitsWHITEListCollection().NewDoc()
	} else {
		ref = c.generalSeatLimitsWHITEListCollection().NewDoc()
	}
	return c.createSeatLimit(ctx, ref, seatID, userID, createdAt, until)
}

func (c *FirestoreControllerImplements) CreateSeatLimitInBLACKList(ctx context.Context, seatID int, userID string, createdAt, until time.Time, isMemberSeat bool) error {
	var ref *firestore.DocumentRef
	if isMemberSeat {
		ref = c.memberSeatLimitsBLACKListCollection().NewDoc()
	} else {
		ref = c.generalSeatLimitsBLACKListCollection().NewDoc()
	}
	return c.createSeatLimit(ctx, ref, seatID, userID, createdAt, until)
}

func (c *FirestoreControllerImplements) createSeatLimit(ctx context.Context, ref *firestore.DocumentRef, seatID int, userID string, createdAt, until time.Time) error {
	data := SeatLimitDoc{
		SeatID:    seatID,
		UserID:    userID,
		CreatedAt: createdAt,
		Until:     until,
	}
	return c.create(ctx, nil, ref, data)
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

func (c *FirestoreControllerImplements) DeleteSeatLimitInWHITEList(ctx context.Context, docID string, isMemberSeat bool) error {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsWHITEListCollection()
	} else {
		collection = c.generalSeatLimitsWHITEListCollection()
	}
	ref := collection.Doc(docID)
	return c.delete(ctx, nil, ref)
}

func (c *FirestoreControllerImplements) DeleteSeatLimitInBLACKList(ctx context.Context, docID string, isMemberSeat bool) error {
	var collection *firestore.CollectionRef
	if isMemberSeat {
		collection = c.memberSeatLimitsBLACKListCollection()
	} else {
		collection = c.generalSeatLimitsBLACKListCollection()
	}
	ref := collection.Doc(docID)
	return c.delete(ctx, nil, ref)
}

func (c *FirestoreControllerImplements) ReadAllMenuDocsOrderByCode(ctx context.Context) ([]MenuDoc, error) {
	iter := c.menuCollection().OrderBy(CodeDocProperty, firestore.Asc).Documents(ctx)
	return getDocDataFromIterator[MenuDoc](iter)
}

func (c *FirestoreControllerImplements) CountUserOrdersOfTheDay(ctx context.Context, userID string, date time.Time) (int64, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1)
	query := c.orderHistoryCollection().
		Where(UserIDDocProperty, "==", userID).
		Where(OrderedAtDocProperty, ">=", start).
		Where(OrderedAtDocProperty, "<", end)
	aggregationQuery := query.NewAggregationQuery().WithCount("all")
	results, err := aggregationQuery.Get(ctx)
	if err != nil {
		return -1, fmt.Errorf("count user orders for day: %w", err)
	}

	count, ok := results["all"]
	if !ok {
		return -1, errors.New("firestore: couldn't get alias for COUNT from results")
	}

	countValue, ok := count.(*firestorepb.Value)
	if !ok {
		return -1, fmt.Errorf("unexpected count type: %T", count)
	}

	return countValue.GetIntegerValue(), nil
}

func (c *FirestoreControllerImplements) CreateOrderHistoryDoc(ctx context.Context, tx *firestore.Transaction, orderHistoryDoc OrderHistoryDoc) error {
	ref := c.orderHistoryCollection().NewDoc()
	return c.create(ctx, tx, ref, orderHistoryDoc)
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
