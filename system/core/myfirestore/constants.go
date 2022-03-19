package myfirestore

const (
	CONFIG          = "config"
	ROOMS           = "rooms"
	USERS           = "users"
	HISTORY         = "history"
	LiveChatHistory = "live-chat-history"
	
	CredentialsConfigDocName     = "credentials"
	SystemConstantsConfigDocName = "constants"
	DefaultRoomDocName           = "default"
	PublishedAtDocName           = "published-at"
	
	DesiredMaxSeatsFirestore                 = "desired-max-seats"
	MaxSeatsFirestore                        = "max-seats"
	CheckDesiredMaxSeatsIntervalSecFirestore = "check-desired-max-seats-interval-sec"
	MinVacancyRateFirestore                  = "min-vacancy-rate"
	LastResetDailyTotalStudySecFirestore     = "last-reset-daily-total-study-sec"
	LastTransferLiveChatHistoryBigquery      = "last-transfer-live-chat-history-bigquery"
	
	NextPageTokenFirestore             = "youtube-live-chat-next-page-token"
	SeatsFirestore                     = "seats"
	LiveChatIdFirestore                = "youtube-live-chat-id"
	YoutubeBotAccessTokenFirestore     = "youtube-bot-access-token"
	YoutubeChannelAccessTokenFirestore = "youtube-channel-access-token"
	YoutubeBotExpirationDateFirestore  = "youtube-bot-expiration-date"
	YoutubeChannelExpirationDate       = "youtube-channel-expiration-date"
	
	LastEnteredFirestore        = "last-entered"
	LastExitedFirestore         = "last-exited"
	DailyTotalStudySecFirestore = "daily-total-study-sec"
	TotalStudySecFirestore      = "total-study-sec"
	RankVisibleFirestore        = "rank-visible"
	DefaultStudyMinFirestore    = "default-study-min"
)
