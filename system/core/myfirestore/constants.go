package myfirestore

const ( // TODO: ~~CollectionName, DocProperty~~に変更
	CONFIG                    = "config"
	SEATS                     = "seats"
	MemberSeats               = "member-seats"
	USERS                     = "users"
	LiveChatHistory           = "live-chat-history"
	UserActivities            = "user-activities"
	WorkHistory               = "work-history"
	DailyWorkHistory          = "daily-work-history"
	SeatLimitsBlackList       = "seat-limits-black-list"
	SeatLimitsWhiteList       = "seat-limits-white-list"
	MemberSeatLimitsBlackList = "member-seat-limits-black-list"
	MemberSeatLimitsWhiteList = "member-seat-limits-white-list"

	CredentialsConfigDocName     = "credentials"
	SystemConstantsConfigDocName = "constants"
	PublishedAtDocProperty       = "published-at"
	TakenAtDocProperty           = "taken-at"
	UserIdDocProperty            = "user-id"
	SeatIdDocProperty            = "seat-id"

	UntilDocProperty = "until"

	StateDocProperty             = "state"
	CurrentStateUntilDocProperty = "current-state-until"

	ActivityTypeDocProperty = "activity-type"

	DesiredMaxSeatsDocProperty                       = "desired-max-seats"
	DesiredMemberMaxSeatsDocProperty                 = "desired-member-max-seats"
	MaxSeatsDocProperty                              = "max-seats"
	MemberMaxSeatsDocProperty                        = "member-max-seats"
	LastResetDailyTotalStudySecDocProperty           = "last-reset-daily-total-study-sec"
	LastTransferCollectionHistoryBigqueryDocProperty = "last-transfer-collection-history-bigquery"
	LastLongTimeSittingCheckedDocProperty            = "last-long-time-sitting-checked"
	LastDailyWorkHistoryTargetDateDocProperty        = "last-daily-work-history-target-datetime"

	NextPageTokenDocProperty = "youtube-live-chat-next-page-token"
	LiveChatIdDocProperty    = "youtube-live-chat-id"

	LastEnteredDocProperty                 = "last-entered"
	LastExitedDocProperty                  = "last-exited"
	DailyTotalStudySecDocProperty          = "daily-total-study-sec"
	TotalStudySecDocProperty               = "total-study-sec"
	RankVisibleDocProperty                 = "rank-visible"
	DefaultStudyMinDocProperty             = "default-study-min"
	FavoriteColorDocProperty               = "favorite-color"
	RankPointDocProperty                   = "rank-point"
	LastRPProcessedDocProperty             = "last-rp-processed"
	IsContinuousActiveDocProperty          = "is-continuous-active"
	CurrentActivityStateStartedDocProperty = "current-activity-state-started"
	LastPenaltyImposedDaysDocProperty      = "last-penalty-imposed-days"
	IsMemberSeatDocProperty                = "is-member-seat"
	EndedAtDocProperty                     = "ended-at"
	DateDocProperty                        = "date"
	WorkSecDocProperty                     = "work-sec"
	TimezoneOffsetDocProperty              = "timezone-offset"
	UpdatedAtDocProperty                   = "updated-at"

	FirestoreWritesLimitPerRequest = 500 // Firestoreの仕様として決まっている
)
