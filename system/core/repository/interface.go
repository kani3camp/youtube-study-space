package repository

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
)

// DBClient テストでfirestore.Clientをmockできるように定義
type DBClient interface {
	Collection(path string) *firestore.CollectionRef
	Doc(path string) *firestore.DocumentRef
	RunTransaction(ctx context.Context, f func(context.Context, *firestore.Transaction) error, opts ...firestore.TransactionOption) (err error)
	Close() error
}

type Repository interface {
	FirestoreClient() DBClient

	// Document Operations
	DeleteDocRef(ctx context.Context, tx *firestore.Transaction, ref *firestore.DocumentRef) error

	// Credential Operations
	ReadCredentialsConfig(ctx context.Context, tx *firestore.Transaction) (CredentialsConfigDoc, error)
	ReadSystemConstantsConfig(ctx context.Context, tx *firestore.Transaction) (ConstantsConfigDoc, error)
	ReadLiveChatId(ctx context.Context, tx *firestore.Transaction) (string, error)
	ReadNextPageToken(ctx context.Context, tx *firestore.Transaction) (string, error)
	UpdateNextPageToken(ctx context.Context, nextPageToken string) error

	// Seat Operations
	ReadGeneralSeats(ctx context.Context) ([]SeatDoc, error)
	ReadMemberSeats(ctx context.Context) ([]SeatDoc, error)
	ReadSeatsExpiredUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error)
	ReadSeatsExpiredBreakUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error)
	ReadSeat(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) (SeatDoc, error)
	ReadSeatWithUserId(ctx context.Context, userId string, isMemberSeat bool) (SeatDoc, error)
	ReadActiveWorkNameSeats(ctx context.Context, isMemberSeat bool) ([]SeatDoc, error)
	CreateSeat(tx *firestore.Transaction, seat SeatDoc, isMemberSeat bool) error
	UpdateSeat(ctx context.Context, tx *firestore.Transaction, seat SeatDoc, isMemberSeat bool) error
	DeleteSeat(ctx context.Context, tx *firestore.Transaction, seatId int, isMemberSeat bool) error

	// User Operations
	ReadUser(ctx context.Context, tx *firestore.Transaction, userId string) (UserDoc, error)
	CreateUser(ctx context.Context, tx *firestore.Transaction, userId string, userData UserDoc) error
	UpdateUserLastEnteredDate(tx *firestore.Transaction, userId string, enteredDate time.Time) error
	UpdateUserLastExitedDate(tx *firestore.Transaction, userId string, exitedDate time.Time) error
	UpdateUserRankVisible(tx *firestore.Transaction, userId string, rankVisible bool) error
	UpdateUserDefaultStudyMin(tx *firestore.Transaction, userId string, defaultStudyMin int) error
	UpdateUserFavoriteColor(tx *firestore.Transaction, userId string, colorCode string) error
	UpdateUserTotalTime(tx *firestore.Transaction, userId string, newTotalTimeSec int, newDailyTotalTimeSec int) error
	UpdateUserRankPoint(tx *firestore.Transaction, userId string, rp int) error
	UpdateUserLastRPProcessed(tx *firestore.Transaction, userId string, date time.Time) error
	UpdateUserRPAndLastPenaltyImposedDays(ctx context.Context, tx *firestore.Transaction, userId string, newRP int, newLastPenaltyImposedDays int) error
	UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx context.Context, tx *firestore.Transaction, userId string, isContinuousActive bool, currentActivityStateStarted time.Time) error
	UpdateUserLastPenaltyImposedDays(ctx context.Context, tx *firestore.Transaction, userId string, lastPenaltyImposedDays int) error

	// Live Chat Operations
	UpdateLiveChatId(ctx context.Context, tx *firestore.Transaction, liveChatId string) error
	CreateLiveChatHistoryDoc(ctx context.Context, tx *firestore.Transaction, liveChatHistoryDoc LiveChatHistoryDoc) error
	Get500LiveChatHistoryDocIdsBeforeDate(ctx context.Context, date time.Time) *firestore.DocumentIterator

	// User Activity Operations
	CreateUserActivityDoc(ctx context.Context, tx *firestore.Transaction, activity UserActivityDoc) error
	Get500UserActivityDocIdsBeforeDate(ctx context.Context, date time.Time) *firestore.DocumentIterator
	GetAllUserActivityDocIdsAfterDate(ctx context.Context, date time.Time) *firestore.DocumentIterator
	Get500OrderHistoryDocIdsBeforeDate(ctx context.Context, date time.Time) *firestore.DocumentIterator
	GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context, date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error)
	GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context, date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error)
	GetUsersActiveAfterDate(ctx context.Context, date time.Time) *firestore.DocumentIterator

	// Seat Limit Operations
	ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error)
	ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error)
	CreateSeatLimitInWHITEList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error
	CreateSeatLimitInBLACKList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error
	Get500SeatLimitsAfterUntilInWHITEList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) *firestore.DocumentIterator
	Get500SeatLimitsAfterUntilInBLACKList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) *firestore.DocumentIterator
	DeleteSeatLimitInWHITEList(ctx context.Context, docId string, isMemberSeat bool) error
	DeleteSeatLimitInBLACKList(ctx context.Context, docId string, isMemberSeat bool) error

	// Menu Operations
	ReadAllMenuDocsOrderByCode(ctx context.Context) ([]MenuDoc, error)

	// Order History Operations
	CountUserOrdersOfTheDay(ctx context.Context, userId string, date time.Time) (int64, error)
	CreateOrderHistoryDoc(ctx context.Context, tx *firestore.Transaction, orderHistoryDoc OrderHistoryDoc) error

	// Work Name Trend Operations
	UpdateWorkNameTrend(ctx context.Context, tx *firestore.Transaction, workNameTrend WorkNameTrendDoc) error

	// General Operations
	GetAllUserDocRefs(ctx context.Context) ([]*firestore.DocumentRef, error)
	GetAllNonDailyZeroUserDocs(ctx context.Context) *firestore.DocumentIterator
	ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error
	UpdateLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error
	UpdateLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error
	UpdateLastTransferCollectionHistoryBigquery(ctx context.Context, timestamp time.Time) error
	UpdateDesiredMaxSeats(ctx context.Context, tx *firestore.Transaction, desiredMaxSeats int) error
	UpdateDesiredMemberMaxSeats(ctx context.Context, tx *firestore.Transaction, desiredMemberMaxSeats int) error
	UpdateMaxSeats(ctx context.Context, tx *firestore.Transaction, maxSeats int) error
	UpdateMemberMaxSeats(ctx context.Context, tx *firestore.Transaction, memberMaxSeats int) error
	UpdateAccessTokenOfChannelCredential(ctx context.Context, tx *firestore.Transaction, accessToken string, expireDate time.Time) error
	UpdateAccessTokenOfBotCredential(ctx context.Context, tx *firestore.Transaction, accessToken string, expireDate time.Time) error
}
