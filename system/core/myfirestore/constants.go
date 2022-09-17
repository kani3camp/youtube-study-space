package myfirestore

const (
	CONFIG          = "config"
	SEATS           = "seats"
	USERS           = "users"
	LiveChatHistory = "live-chat-history"
	UserActivities  = "user-activities"
	SeatLimits      = "seat-limits"
	
	CredentialsConfigDocName     = "credentials"
	SystemConstantsConfigDocName = "constants"
	PublishedAtDocProperty       = "published-at"
	TakenAtDocProperty           = "taken-at"
	UserIdDocProperty            = "user-id"
	SeatIdDocProperty            = "seat-id"
	
	CreatedAtDocProperty = "created-at"
	UntilDocProperty     = "until"
	
	DesiredMaxSeatsDocProperty                       = "desired-max-seats"
	MaxSeatsDocProperty                              = "max-seats"
	MinVacancyRateDocProperty                        = "min-vacancy-rate"
	LastResetDailyTotalStudySecDocProperty           = "last-reset-daily-total-study-sec"
	LastTransferCollectionHistoryBigqueryDocProperty = "last-transfer-collection-history-bigquery"
	LastLongTimeSittingCheckedDocProperty            = "last-long-time-sitting-checked"
	
	NextPageTokenDocProperty             = "youtube-live-chat-next-page-token"
	LiveChatIdDocProperty                = "youtube-live-chat-id"
	YoutubeBotAccessTokenDocProperty     = "youtube-bot-access-token"
	YoutubeChannelAccessTokenDocProperty = "youtube-channel-access-token"
	YoutubeBotExpirationDateDocProperty  = "youtube-bot-expiration-date"
	YoutubeChannelExpirationDate         = "youtube-channel-expiration-date"
	
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
	
	FirestoreWritesLimitPerRequest = 500 // Firestoreの仕様として決まっている
)
