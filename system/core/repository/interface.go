package repository

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
)

// DB is a generic database interface.
type DB interface {
	Close() error
}

type Repository interface {
	RunTransaction(ctx context.Context, f func(ctx context.Context) error) error

	// Document Operations
	DeleteDocRef(ctx context.Context, ref *firestore.DocumentRef) error

	// Credential Operations
	ReadCredentialsConfig(ctx context.Context) (CredentialsConfigDoc, error)
	ReadSystemConstantsConfig(ctx context.Context) (ConstantsConfigDoc, error)
	ReadLiveChatId(ctx context.Context) (string, error)
	ReadNextPageToken(ctx context.Context) (string, error)
	UpdateNextPageToken(ctx context.Context, nextPageToken string) error

	// Seat Operations
	ReadGeneralSeats(ctx context.Context) ([]SeatDoc, error)
	ReadMemberSeats(ctx context.Context) ([]SeatDoc, error)
	ReadSeatsExpiredUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error)
	ReadSeatsExpiredBreakUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error)
	ReadSeat(ctx context.Context, seatId int, isMemberSeat bool) (SeatDoc, error)
	ReadSeatWithUserId(ctx context.Context, userId string, isMemberSeat bool) (SeatDoc, error)
	ReadActiveWorkNameSeats(ctx context.Context, isMemberSeat bool) ([]SeatDoc, error)
	CreateSeat(ctx context.Context, seat SeatDoc, isMemberSeat bool) error
	UpdateSeat(ctx context.Context, seat SeatDoc, isMemberSeat bool) error
	DeleteSeat(ctx context.Context, seatId int, isMemberSeat bool) error

	// User Operations
	ReadUser(ctx context.Context, userId string) (UserDoc, error)
	CreateUser(ctx context.Context, userId string, userData UserDoc) error
	UpdateUserLastEnteredDate(ctx context.Context, userId string, enteredDate time.Time) error
	UpdateUserLastExitedDate(ctx context.Context, userId string, exitedDate time.Time) error
	UpdateUserRankVisible(ctx context.Context, userId string, rankVisible bool) error
	UpdateUserDefaultStudyMin(ctx context.Context, userId string, defaultStudyMin int) error
	UpdateUserFavoriteColor(ctx context.Context, userId string, colorCode string) error
	UpdateUserTotalTime(ctx context.Context, userId string, newTotalTimeSec int, newDailyTotalTimeSec int) error
	UpdateUserRankPoint(ctx context.Context, userId string, rp int) error
	UpdateUserLastRPProcessed(ctx context.Context, userId string, date time.Time) error
	UpdateUserRPAndLastPenaltyImposedDays(ctx context.Context, userId string, newRP int, newLastPenaltyImposedDays int) error
	UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx context.Context, userId string, isContinuousActive bool, currentActivityStateStarted time.Time) error
	UpdateUserLastPenaltyImposedDays(ctx context.Context, userId string, lastPenaltyImposedDays int) error

	// Live Chat Operations
	UpdateLiveChatId(ctx context.Context, liveChatId string) error
	CreateLiveChatHistoryDoc(ctx context.Context, liveChatHistoryDoc LiveChatHistoryDoc) error
	Get500LiveChatHistoryDocIdsBeforeDate(ctx context.Context, date time.Time) ([]LiveChatHistoryDoc, error)

	// User Activity Operations
	CreateUserActivityDoc(ctx context.Context, activity UserActivityDoc) error
	Get500UserActivityDocIdsBeforeDate(ctx context.Context, date time.Time) ([]UserActivityDoc, error)
	GetAllUserActivityDocIdsAfterDate(ctx context.Context, date time.Time) ([]UserActivityDoc, error)
	Get500OrderHistoryDocIdsBeforeDate(ctx context.Context, date time.Time) ([]OrderHistoryDoc, error)
	GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context, date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error)
	GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context, date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error)
	GetUsersActiveAfterDate(ctx context.Context, date time.Time) ([]UserDoc, error)

	// Seat Limit Operations
	ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error)
	ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error)
	CreateSeatLimitInWHITEList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error
	CreateSeatLimitInBLACKList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error
	Get500SeatLimitsAfterUntilInWHITEList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatLimitDoc, error)
	Get500SeatLimitsAfterUntilInBLACKList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatLimitDoc, error)
	DeleteSeatLimitInWHITEList(ctx context.Context, docId string, isMemberSeat bool) error
	DeleteSeatLimitInBLACKList(ctx context.Context, docId string, isMemberSeat bool) error

	// Menu Operations
	ReadAllMenuDocsOrderByCode(ctx context.Context) ([]MenuDoc, error)

	// Order History Operations
	CountUserOrdersOfTheDay(ctx context.Context, userId string, date time.Time) (int64, error)
	CreateOrderHistoryDoc(ctx context.Context, orderHistoryDoc OrderHistoryDoc) error

	// Work Name Trend Operations
	UpdateWorkNameTrend(ctx context.Context, workNameTrend WorkNameTrendDoc) error

	// General Operations
	GetAllUserDocRefs(ctx context.Context) ([]string, error)
	GetAllNonDailyZeroUserDocs(ctx context.Context) ([]UserDoc, error)
	ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error
	UpdateLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error
	UpdateLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error
	UpdateLastTransferCollectionHistoryBigquery(ctx context.Context, timestamp time.Time) error
	UpdateDesiredMaxSeats(ctx context.Context, desiredMaxSeats int) error
	UpdateDesiredMemberMaxSeats(ctx context.Context, desiredMemberMaxSeats int) error
	UpdateMaxSeats(ctx context.Context, maxSeats int) error
	UpdateMemberMaxSeats(ctx context.Context, memberMaxSeats int) error
	UpdateAccessTokenOfChannelCredential(ctx context.Context, accessToken string, expireDate time.Time) error
	UpdateAccessTokenOfBotCredential(ctx context.Context, accessToken string, expireDate time.Time) error
}
