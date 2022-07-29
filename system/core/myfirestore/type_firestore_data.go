package myfirestore

import (
	"time"
)

type ConstantsConfigDoc struct {
	MaxWorkTimeMin     int `firestore:"max-work-time-min"`     // 設定可能な最大入室時間（分）
	MinWorkTimeMin     int `firestore:"min-work-time-min"`     // // 設定可能な最小入室時間（分）
	DefaultWorkTimeMin int `firestore:"default-work-time-min"` // デフォルト入室時間（分）
	SleepIntervalMilli int `firestore:"sleep-interval-milli"`  // Botプログラムにおいて次のライブチャットを読み込むまでの最小インターバル（ミリ秒）
	
	// 前回のデイリー累計作業時間のリセット日時（1日に2回以上リセット処理を走らせてしまっても大丈夫なように）
	LastResetDailyTotalStudySec time.Time `firestore:"last-reset-daily-total-study-sec" json:"last_reset_daily_total_study_sec"`
	
	// 席数（最大席番号）はfirestoreで管理される。各ルームの座席数の情報はfirestoreやbotプログラムでは保持せず、monitorでのみ参照できるため、
	// monitorが定期的に最大席数がmin-vacancy-rateを満たしつつ妥当な値であるかを判断し、最大席数を変更すべきと判断したらfirestoreの
	// desired-max-seatsを更新し、botプログラムが参照できるようにする。
	// botプログラムは定期的にfirestoreのdesired-max-seatsを読み込み、問題ないことを確認してmax-seatsに反映する。
	MaxSeats        int `firestore:"max-seats" json:"max_seats"`                 // 席数（最大席番号）
	DesiredMaxSeats int `firestore:"desired-max-seats" json:"desired_max_seats"` // 希望の席数（最大席番号）
	
	// botプログラムにおいてdesired-max-seatsをチェックする最小インターバル
	CheckDesiredMaxSeatsIntervalSec int `firestore:"check-desired-max-seats-interval-sec"`
	
	// 最小空席率。これを満たすようにmax-seatsが調整される。
	MinVacancyRate float32 `firestore:"min-vacancy-rate" json:"min_vacancy_rate"`
}

type CredentialsConfigDoc struct {
	// ラインBotのアクセス情報
	LineBotChannelSecret     string `firestore:"line-bot-channel-secret"`
	LineBotChannelToken      string `firestore:"line-bot-channel-token"`
	LineBotDestinationLineId string `firestore:"line-bot-destination-line-id"`
	
	// Discord Botのアクセス情報
	DiscordBotToken         string `firestore:"discord-bot-token"`
	DiscordBotTextChannelId string `firestore:"discord-bot-text-channel-id"`
	
	// Bot用youtubeチャンネルのAPIアクセス情報
	YoutubeBotAccessToken    string    `firestore:"youtube-bot-access-token"`
	YoutubeBotClientId       string    `firestore:"youtube-bot-client-id"`
	YoutubeBotClientSecret   string    `firestore:"youtube-bot-client-secret"`
	YoutubeBotExpirationDate time.Time `firestore:"youtube-bot-expiration-date"`
	YoutubeBotRefreshToken   string    `firestore:"youtube-bot-refresh-token"`
	YoutubeBotChannelId      string    `firestore:"youtube-bot-channel-id"`
	
	// ライブ配信用youtubeチャンネルのAPIアクセス情報
	YoutubeChannelAccessToken    string    `firestore:"youtube-channel-access-token"`
	YoutubeChannelClientId       string    `firestore:"youtube-channel-client-id"`
	YoutubeChannelClientSecret   string    `firestore:"youtube-channel-client-secret"`
	YoutubeChannelExpirationDate time.Time `firestore:"youtube-channel-expiration-date"`
	YoutubeChannelRefreshToken   string    `firestore:"youtube-channel-refresh-token"`
	
	// youtubeライブ配信の情報
	YoutubeLiveChatId            string `firestore:"youtube-live-chat-id"`
	YoutubeLiveChatNextPageToken string `firestore:"youtube-live-chat-next-page-token"`
	OAuthRefreshTokenUrl         string `firestore:"o-auth-refresh-token-url"`
}

// RoomDoc ルームの入室状況
type RoomDoc struct {
	Seats []Seat `json:"seats" firestore:"seats"`
}

func NewRoomDoc() RoomDoc {
	return RoomDoc{
		Seats: []Seat{}, // 席情報の配列
	}
}

type Seat struct {
	SeatId          int       `json:"seat_id" firestore:"seat-id"`                     // 席番号
	UserId          string    `json:"user_id" firestore:"user-id"`                     // ユーザーID
	UserDisplayName string    `json:"user_display_name" firestore:"user-display-name"` // 表示ユーザー名
	WorkName        string    `json:"work_name" firestore:"work-name"`                 // 作業名
	EnteredAt       time.Time `json:"entered_at" firestore:"entered-at"`               // 入室日時
	Until           time.Time `json:"until" firestore:"until"`                         // 自動退室予定時刻
	ColorCode       string    `json:"color_code" firestore:"color-code"`               // 席の背景色のカラーコード
}

type UserDoc struct {
	// 当日の累計作業時間
	DailyTotalStudySec int `json:"daily_total_study_sec" firestore:"daily-total-study-sec"`
	
	// 累計作業時間
	TotalStudySec int `json:"total_study_sec" firestore:"total-study-sec"`
	
	// 登録日
	RegistrationDate time.Time `json:"registration_date" firestore:"registration-date"`
	
	// ステータスメッセージ（今は使用されていない）
	StatusMessage string `json:"status_message" firestore:"status-message"`
	
	// 前回の入室日時
	LastEntered time.Time `json:"last_entered" firestore:"last-entered"`
	
	// 前回の退室日時
	LastExited time.Time `json:"last_exited" firestore:"last-exited"`
	
	// ランク表示をするかどうか
	RankVisible bool `json:"rank_visible" firestore:"rank-visible"`
	
	// そのユーザーのデフォルト入室時間（分）（今は使用されていない）
	DefaultStudyMin int `json:"default_study_min" firestore:"default-study-min"`
}

type UserHistoryDoc struct {
	Action  string      `json:"action" firestore:"action"`
	Date    time.Time   `json:"date" firestore:"date"`
	Details interface{} `json:"details" firestore:"details"`
}
