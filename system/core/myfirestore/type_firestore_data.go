package myfirestore

import (
	"time"
)


type ConstantsConfigDoc struct {
	MaxWorkTimeMin int `firestore:"max-work-time-min"`
	MinWorkTimeMin int `firestore:"min-work-time-min"`
	DefaultWorkTimeMin int `firestore:"default-work-time-min"`
	SleepIntervalMilli int `firestore:"sleep-interval-milli"`
	LastResetDailyTotalStudySec time.Time `firestore:"last-reset-daily-total-study-sec" json:"last_reset_daily_total_study_sec"`
}

type CredentialsConfigDoc struct {
	LineBotChannelSecret string `firestore:"line-bot-channel-secret"`
	LineBotChannelToken string `firestore:"line-bot-channel-token"`
	LineBotDestinationLineId string `firestore:"line-bot-destination-line-id"`
	
	YoutubeBotAccessToken string `firestore:"youtube-bot-access-token"`
	YoutubeBotClientId string `firestore:"youtube-bot-client-id"`
	YoutubeBotClientSecret string `firestore:"youtube-bot-client-secret"`
	YoutubeBotExpirationDate time.Time `firestore:"youtube-bot-expiration-date"`
	YoutubeBotRefreshToken string `firestore:"youtube-bot-refresh-token"`
	
	YoutubeChannelAccessToken string `firestore:"youtube-channel-access-token"`
	YoutubeChannelClientId string `firestore:"youtube-channel-client-id"`
	YoutubeChannelClientSecret string `firestore:"youtube-channel-client-secret"`
	YoutubeChannelExpirationDate time.Time `firestore:"youtube-channel-expiration-date"`
	YoutubeChannelRefreshToken string `firestore:"youtube-channel-refresh-token"`
	
	YoutubeLiveChatId string `firestore:"youtube-live-chat-id"`
	YoutubeLiveChatNextPageToken string `firestore:"youtube-live-chat-next-page-token"`
	OAuthRefreshTokenUrl string `firestore:"o-auth-refresh-token-url"`
}


type RoomDoc struct {
	Seats []Seat `json:"seats" firestore:"seats"`
}
func NewRoomDoc() RoomDoc {
	return RoomDoc{
		Seats: []Seat{},
	}
}

type Seat struct {
	SeatId int `json:"seat_id" firestore:"seat-id"`
	UserId string `json:"user_id" firestore:"user-id"`
	UserDisplayName string `json:"user_display_name" firestore:"user-display-name"`
	WorkName string `json:"work_name" firestore:"work-name"`
	EnteredAt time.Time `json:"entered_at" firestore:"entered-at"`
	Until time.Time `json:"until" firestore:"until"`
	ColorCode string `json:"color_code" firestore:"color-code"`
}


type UserDoc struct {
	DailyTotalStudySec int `json:"daily_total_study_sec" firestore:"daily-total-study-sec"`
	TotalStudySec int `json:"total_study_sec" firestore:"total-study-sec"`
	RegistrationDate time.Time `json:"registration_date" firestore:"registration-date"`
	StatusMessage string `json:"status_message" firestore:"status-message"`
	LastEntered time.Time `json:"last_entered" firestore:"last-entered"`
	LastExited      time.Time `json:"last_exited" firestore:"last-exited"`
	RankVisible     bool      `json:"rank_visible" firestore:"rank-visible"`
	DefaultStudyMin int       `json:"default_study_min" firestore:"default-study-min"`
}


type UserHistoryDoc struct {
	Action string `json:"action" firestore:"action"`
	Date time.Time `json:"date" firestore:"date"`
	Details interface{} `json:"details" firestore:"details"`
}



