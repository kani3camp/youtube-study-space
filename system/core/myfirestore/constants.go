package myfirestore

const (
	CONFIG          = "config"
	ROOMS           = "rooms"
	USERS           = "users"
	HISTORY         = "history"
	LiveChatHistory = "live-chat-history"
	UserActivities  = "user-activities"
	
	CredentialsConfigDocName     = "credentials"
	SystemConstantsConfigDocName = "constants"
	DefaultRoomDocName           = "default"
	PublishedAtDocProperty       = "published-at"
	TakenAtDocProperty           = "taken-at"
	UserIdDocProperty            = "user-id"
	SeatIdDocProperty            = "seat-id"
	
	DesiredMaxSeatsDocProperty                       = "desired-max-seats"
	MaxSeatsDocProperty                              = "max-seats"
	MinVacancyRateDocProperty                        = "min-vacancy-rate"
	LastResetDailyTotalStudySecDocProperty           = "last-reset-daily-total-study-sec"
	LastTransferCollectionHistoryBigqueryDocProperty = "last-transfer-collection-history-bigquery"
	LastLongTimeSittingCheckedDocProperty            = "last-long-time-sitting-checked"
	
	NextPageTokenDocProperty             = "youtube-live-chat-next-page-token"
	SeatsDocProperty                     = "seats"
	LiveChatIdDocProperty                = "youtube-live-chat-id"
	YoutubeBotAccessTokenDocProperty     = "youtube-bot-access-token"
	YoutubeChannelAccessTokenDocProperty = "youtube-channel-access-token"
	YoutubeBotExpirationDateDocProperty  = "youtube-bot-expiration-date"
	YoutubeChannelExpirationDate         = "youtube-channel-expiration-date"
	
	LastEnteredDocProperty        = "last-entered"
	LastExitedDocProperty         = "last-exited"
	DailyTotalStudySecDocProperty = "daily-total-study-sec"
	TotalStudySecDocProperty      = "total-study-sec"
	RankVisibleDocProperty        = "rank-visible"
	DefaultStudyMinDocProperty    = "default-study-min"
	FavoriteColorDocProperty      = "favorite-color"
	RankPointDocProperty          = "rank-point"
	
	FirestoreWritesLimitPerRequest = 500 // Firestoreの仕様として決まっている
)

func NewRoomDoc() RoomDoc {
	return RoomDoc{
		Seats: []Seat{}, // 席情報の配列
	}
}
