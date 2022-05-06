package myfirestore

import (
	"time"
)

type ConstantsConfigDoc struct {
	MaxWorkTimeMin     int `firestore:"max-work-time-min"`     // 設定可能な最大入室時間（分）
	MinWorkTimeMin     int `firestore:"min-work-time-min"`     // 設定可能な最小入室時間（分）
	DefaultWorkTimeMin int `firestore:"default-work-time-min"` // デフォルト入室時間（分）
	
	MinBreakDurationMin     int `firestore:"min-break-duration-min"`     // 設定可能な最小休憩時間（分）
	MinBreakIntervalMin     int `firestore:"min-break-interval-min"`     // 休憩できる最短間隔（分）
	MaxBreakDurationMin     int `firestore:"max-break-duration-min"`     // 休憩できる最大時間（分）
	DefaultBreakDurationMin int `firestore:"default-break-duration-min"` // デフォルト休憩時間（分）
	
	SleepIntervalMilli int `firestore:"sleep-interval-milli"` // Botプログラムにおいて次のライブチャットを読み込むまでの最小インターバル（ミリ秒）
	
	// 前回のデイリー累計作業時間のリセット日時（1日に2回以上リセット処理を走らせてしまっても大丈夫なように）
	LastResetDailyTotalStudySec time.Time `firestore:"last-reset-daily-total-study-sec" json:"last_reset_daily_total_study_sec"`
	
	// 前回のチャットログや入退室ログをbigqueryに保存した日時
	LastTransferCollectionHistoryBigquery time.
	Time `firestore:"last-transfer-collection-history-bigquery" json:"last_transfer_collection_history_bigquery"`
	
	// 前回の長時間入室チェックをした日時
	LastLongTimeSittingChecked time.Time `firestore:"last-long-time-sitting-checked" json:"last_long_time_sitting_checked"`
	
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
	
	// bigqueryへのデータバックアップ関連。bigqueryのテーブル名などはmybigqueryで定数定義。
	GcpRegion                      string `firestore:"gcp-region"`
	GcsFirestoreExportBucketName   string `firestore:"gcs-firestore-export-bucket-name"`
	CollectionHistoryRetentionDays int    `firestore:"collection-history-retention-days"` // 何日間live chat historyおよびuser activityを保持するか
	
	// 同座席入室制限関連
	RecentRangeMin     int `firestore:"recent-range-min"`     // 過去何分以内に。
	RecentThresholdMin int `firestore:"recent-threshold-min"` // 何分間以上該当座席に座っていたらアウト
	
	// 長時間入室制限関連
	CheckLongTimeSittingIntervalMinutes int `firestore:"check-long-time-sitting-interval-minutes" json:"check_long_time_sitting_interval_minutes"` // 何分おきにチェックを行うか
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

type SeatState string

const (
	WorkState  SeatState = "work"
	BreakState           = "break"
)

type SeatAppearance struct {
	ColorCode     string `json:"color_code" firestore:"color-code"`
	NumStars      int    `json:"num_stars" firestore:"num-stars"`
	GlowAnimation bool   `json:"glow_animation" firestore:"glow-animation"`
}

type Seat struct {
	SeatId                 int            `json:"seat_id" firestore:"seat-id"`                     // 席番号
	UserId                 string         `json:"user_id" firestore:"user-id"`                     // ユーザーID
	UserDisplayName        string         `json:"user_display_name" firestore:"user-display-name"` // 表示ユーザー名
	WorkName               string         `json:"work_name" firestore:"work-name"`                 // 作業名
	BreakWorkName          string         `json:"break_work_name" firestore:"break-work-name"`     // 休憩中の作業名
	EnteredAt              time.Time      `json:"entered_at" firestore:"entered-at"`               // 入室日時
	Until                  time.Time      `json:"until" firestore:"until"`                         // 自動退室予定時刻
	Appearance             SeatAppearance `json:"appearance" firestore:"appearance"`               // 席の見え方
	State                  SeatState      `json:"state" firestore:"state"`
	CurrentStateStartedAt  time.Time      `json:"current_state_started_at" firestore:"current-state-started-at"`
	CurrentStateUntil      time.Time      `json:"current_state_until" firestore:"current-state-until"`
	CumulativeWorkSec      int            `json:"cumulative_work_sec" firestore:"cumulative-work-sec"` // 前回のstateまでの合計作業時間（秒）。休憩時間は含まない。
	DailyCumulativeWorkSec int            `json:"daily_cumulative_work_sec" firestore:"daily-cumulative-work-sec"`
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
	
	// ランクポイント。ランク表示のオンオフに関わらずランクの計算は行われる
	RankPoint int `json:"rank_point" firestore:"rank-point"`
	
	// 前回の連続非アクティブ日数によるRPペナルティ処理が行われたときの、該当非アクティブ連続日数
	LastPenaltyImposedDays int `json:"last_penalty_imposed_days" firestore:"last-penalty-imposed-days"`
	
	// 昨日までで、連続日数でアクティブか
	IsContinuousActive bool `json:"is_continuous_active" firestore:"is-continuous-active"`
	
	// 昨日までの状態（アクティブor非アクティブ）が始まった日付
	CurrentActivityStateStarted time.Time `json:"current_activity_state_started" firestore:"current-activity-state-started"`
	
	// お気に入りの色のカラーコード
	FavoriteColor string `json:"favorite_color" firestore:"favorite-color"`
}

type UserHistoryDoc struct {
	Action  string      `json:"action" firestore:"action"`
	Date    time.Time   `json:"date" firestore:"date"`
	Details interface{} `json:"details" firestore:"details"`
}

type LiveChatHistoryDoc struct {
	AuthorChannelId       string    `json:"author_channel_id" firestore:"author-channel-id"`
	AuthorDisplayName     string    `json:"author_display_name" firestore:"author-display-name"`
	AuthorProfileImageUrl string    `json:"author_profile_image_url" firestore:"author-profile-image-url"`
	AuthorIsChatModerator bool      `json:"author_is_chat_moderator" firestore:"author-is-chat-moderator"`
	Id                    string    `json:"id" firestore:"id"`                     // メッセージのID。APIで取得するliveChatMessages resourceで定義されているid
	LiveChatId            string    `json:"live_chat_id" firestore:"live-chat-id"` // ライブ配信ごとのid。ずっと続く配信だと不変。
	MessageText           string    `json:"message_text" firestore:"message-text"`
	PublishedAt           time.Time `json:"published_at" firestore:"published-at"`
	Type                  string    `json:"type" firestore:"type"`
}

type UserActivityType string

const (
	EnterRoomActivity  UserActivityType = "enter-room"
	ExitRoomActivity                    = "exit-room"
	StartBreakActivity                  = "start-break"
	EndBreakActivity                    = "end-break"
)

type UserActivityDoc struct {
	UserId       string           `json:"user_id" firestore:"user-id"`
	ActivityType UserActivityType `json:"activity_type" firestore:"activity-type"`
	SeatId       int              `json:"seat_id" firestore:"seat-id"`
	TakenAt      time.Time        `json:"taken_at" firestore:"taken-at"`
}
