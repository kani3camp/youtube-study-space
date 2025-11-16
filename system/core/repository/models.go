package repository

import (
	"time"
)

// ConstantsConfigDoc defines constants for the configuration of the system.
type ConstantsConfigDoc struct {
	MaxWorkTimeMin     int `firestore:"max-work-time-min" bson:"max-work-time-min"`     // 指定可能な最大入室時間（分）
	MinWorkTimeMin     int `firestore:"min-work-time-min" bson:"min-work-time-min"`     // 指定可能な最小入室時間（分）
	DefaultWorkTimeMin int `firestore:"default-work-time-min" bson:"default-work-time-min"` // デフォルト入室時間（分）

	MinBreakDurationMin     int `firestore:"min-break-duration-min" bson:"min-break-duration-min"`     // 指定可能な最小休憩時間（分）
	MinBreakIntervalMin     int `firestore:"min-break-interval-min" bson:"min-break-interval-min"`     // 休憩できる最短間隔（分）
	MaxBreakDurationMin     int `firestore:"max-break-duration-min" bson:"max-break-duration-min"`     // 休憩できる最大時間（分）
	DefaultBreakDurationMin int `firestore:"default-break-duration-min" bson:"default-break-duration-min"` // デフォルト休憩時間（分）

	MaxDailyOrderCount int `firestore:"max-daily-order-count" bson:"max-daily-order-count"` // 1日の最大注文数（メンバーシップは上限なし）

	SleepIntervalMilli int `firestore:"sleep-interval-milli" bson:"sleep-interval-milli"` // Botプログラムにおいて次のライブチャットを読み込むまでの最小インターバル（ミリ秒）

	// 前回のデイリー累計作業時間のリセット日時（1日に2回以上リセット処理を走らせてしまっても大丈夫なように）
	LastResetDailyTotalStudySec time.Time `firestore:"last-reset-daily-total-study-sec" json:"last_reset_daily_total_study_sec" bson:"last-reset-daily-total-study-sec"`

	// 前回のチャットログや入退室ログをbigqueryに保存した日時
	LastTransferCollectionHistoryBigquery time.Time `firestore:"last-transfer-collection-history-bigquery" json:"last_transfer_collection_history_bigquery" bson:"last-transfer-collection-history-bigquery"`

	// youtube-monitorが定期的にmax-seatsがmin-vacancy-rateを満たしつつ妥当な値であるかを判断し、最大席数を変更すべきと判断したらfirestoreの
	// desired-max-seatsを更新し、botプログラムが参照できるようにする。
	// botプログラムは定期的にfirestoreのdesired-max-seatsを読み込み、問題ないことを確認してmax-seatsに反映する。
	MaxSeats              int `firestore:"max-seats" json:"max_seats" bson:"max-seats"`                 // 席数（最大席番号）
	MemberMaxSeats        int `firestore:"member-max-seats" json:"member_max_seats" bson:"member-max-seats"`   // メンバー専用席の席数（最大席番号）
	DesiredMaxSeats       int `firestore:"desired-max-seats" json:"desired_max_seats" bson:"desired-max-seats"` // 希望の席数（最大席番号）
	DesiredMemberMaxSeats int `firestore:"desired-member-max-seats" json:"desired_member_max_seats" bson:"desired-member-max-seats"`

	// botプログラムにおいてdesired-max-seatsをチェックする最小インターバル
	CheckDesiredMaxSeatsIntervalSec int `firestore:"check-desired-max-seats-interval-sec" bson:"check-desired-max-seats-interval-sec"`

	// 最小空席率。これを満たすようにmax-seatsが調整される。
	MinVacancyRate float32 `firestore:"min-vacancy-rate" json:"min_vacancy_rate" bson:"min-vacancy-rate"`

	// bigqueryへのデータバックアップ関連。bigqueryのテーブル名などはmybigqueryで定数定義。
	GcpRegion                      string `firestore:"gcp-region" bson:"gcp-region"`
	GcsFirestoreExportBucketName   string `firestore:"gcs-firestore-export-bucket-name" bson:"gcs-firestore-export-bucket-name"`
	CollectionHistoryRetentionDays int    `firestore:"collection-history-retention-days" bson:"collection-history-retention-days"` // 何日間live chat historyおよびuser activityを保持するか

	// 同座席入室制限関連
	RecentRangeMin     int `firestore:"recent-range-min" bson:"recent-range-min"`     // 過去何分以内に。
	RecentThresholdMin int `firestore:"recent-threshold-min" bson:"recent-threshold-min"` // 何分間以上該当座席に座っていたらアウト

	// 長時間入室制限関連
	MinimumCheckLongTimeSittingIntervalMinutes int `firestore:"minimum-check-long-time-sitting-interval-minutes" json:"minimum_check_long_time_sitting_interval_minutes" bson:"minimum-check-long-time-sitting-interval-minutes"` // 最低何分おきにチェックを行うか
	LongTimeSittingPenaltyMinutes              int `firestore:"long-time-sitting-penalty-minutes" json:"long_time_sitting_penalty_minutes" bson:"long-time-sitting-penalty-minutes"`                               // チェックに引っかかった時に課される一定のペナルティ時間。この間はそのユーザーはその座席に座れない。ブラックリストの有効期限に使用される。

	// 並行でRP処理を行うLambdaインスタンスの数
	NumberOfParallelLambdaToProcessUserRP int `firestore:"number-of-parallel-lambda-to-process-user-rp" bson:"number-of-parallel-lambda-to-process-user-rp"`

	// Botの設定（ブロック・通知対象の正規表現など）をまとめたスプレッドシートのID
	BotConfigSpreadsheetId string `firestore:"bot-config-spreadsheet-id" json:"bot_config_spreadsheet_id" bson:"bot-config-spreadsheet-id"`

	YoutubeMembershipEnabled bool `firestore:"youtube-membership-enabled" json:"youtube_membership_enabled" bson:"youtube-membership-enabled"`

	FixedMaxSeatsEnabled bool `firestore:"fixed-max-seats-enabled" json:"fixed_max_seats_enabled" bson:"fixed-max-seats-enabled"`
}

// CredentialsConfigDoc defines credentials for various services.
type CredentialsConfigDoc struct {
	DiscordOwnerBotToken         string `firestore:"discord-owner-bot-token" bson:"discord-owner-bot-token"`
	DiscordOwnerBotTextChannelId string `firestore:"discord-owner-bot-text-channel-id" bson:"discord-owner-bot-text-channel-id"`

	DiscordSharedBotToken         string `firestore:"discord-shared-bot-token" bson:"discord-shared-bot-token"`
	DiscordSharedBotTextChannelId string `firestore:"discord-shared-bot-text-channel-id" bson:"discord-shared-bot-text-channel-id"`
	DiscordSharedBotLogChannelId  string `firestore:"discord-shared-bot-log-channel-id" bson:"discord-shared-bot-log-channel-id"`

	YoutubeBotClientId     string `firestore:"youtube-bot-client-id" bson:"youtube-bot-client-id"`
	YoutubeBotClientSecret string `firestore:"youtube-bot-client-secret" bson:"youtube-bot-client-secret"`
	YoutubeBotRefreshToken string `firestore:"youtube-bot-refresh-token" bson:"youtube-bot-refresh-token"`
	YoutubeBotChannelId    string `firestore:"youtube-bot-channel-id" bson:"youtube-bot-channel-id"`

	YoutubeChannelClientId     string `firestore:"youtube-channel-client-id" bson:"youtube-channel-client-id"`
	YoutubeChannelClientSecret string `firestore:"youtube-channel-client-secret" bson:"youtube-channel-client-secret"`
	YoutubeChannelRefreshToken string `firestore:"youtube-channel-refresh-token" bson:"youtube-channel-refresh-token"`

	YoutubeLiveChatId            string `firestore:"youtube-live-chat-id" bson:"youtube-live-chat-id"`
	YoutubeLiveChatNextPageToken string `firestore:"youtube-live-chat-next-page-token" bson:"youtube-live-chat-next-page-token"`
}

type SeatState string

const (
	WorkState  SeatState = "work"
	BreakState SeatState = "break"
)

type SeatAppearance struct {
	ColorCode1           string `json:"color_code1" firestore:"color-code1" bson:"color-code1"`
	ColorCode2           string `json:"color_code2" firestore:"color-code2" bson:"color-code2"`
	NumStars             int    `json:"num_stars" firestore:"num-stars" bson:"num-stars"`
	ColorGradientEnabled bool   `json:"color_gradient_enabled" firestore:"color-gradient-enabled" bson:"color-gradient-enabled"`
}

type SeatDoc struct {
	SeatId                 int            `json:"seat_id" firestore:"seat-id" bson:"seat-id"` // 席番号
	UserId                 string         `json:"user_id" firestore:"user-id" bson:"user-id"`
	UserDisplayName        string         `json:"user_display_name" firestore:"user-display-name" bson:"user-display-name"`
	WorkName               string         `json:"work_name" firestore:"work-name" bson:"work-name"`             // 作業名
	BreakWorkName          string         `json:"break_work_name" firestore:"break-work-name" bson:"break-work-name"` // 休憩中の作業名
	EnteredAt              time.Time      `json:"entered_at" firestore:"entered-at" bson:"entered-at"`           // 入室日時
	Until                  time.Time      `json:"until" firestore:"until" bson:"until"`                     // 自動退室予定時刻
	Appearance             SeatAppearance `json:"appearance" firestore:"appearance" bson:"appearance"`           // 席の見え方
	MenuCode               string         `json:"menu_code" firestore:"menu-code" bson:"menu-code"`             // メニューコード
	State                  SeatState      `json:"state" firestore:"state" bson:"state"`
	CurrentStateStartedAt  time.Time      `json:"current_state_started_at" firestore:"current-state-started-at" bson:"current-state-started-at"`
	CurrentStateUntil      time.Time      `json:"current_state_until" firestore:"current-state-until" bson:"current-state-until"`
	CumulativeWorkSec      int            `json:"cumulative_work_sec" firestore:"cumulative-work-sec" bson:"cumulative-work-sec"` // 前回のstateまでの合計作業時間（秒）。休憩時間は含まない。
	DailyCumulativeWorkSec int            `json:"daily_cumulative_work_sec" firestore:"daily-cumulative-work-sec" bson:"daily-cumulative-work-sec"`
	UserProfileImageUrl    string         `json:"user_profile_image_url" firestore:"user-profile-image-url" bson:"user-profile-image-url"`
}

type StudySession struct {
	ID        string    `firestore:"id" bson:"id"`
	UserID    string    `firestore:"user_id" bson:"user_id"`
	StartTime time.Time `firestore:"start_time" bson:"start_time"`
	EndTime   time.Time `firestore:"end_time" bson:"end_time"`
}

// SeatLimitDoc defines limitations of a seat.
type SeatLimitDoc struct { // used for both collections seat-limits-black-list and seat-limits-white-list
	SeatId    int       `firestore:"seat-id" bson:"seat-id"`
	UserId    string    `firestore:"user-id" bson:"user-id"`
	CreatedAt time.Time `firestore:"created-at" bson:"created-at"`
	Until     time.Time `firestore:"until" bson:"until"`
}

type UserDoc struct {
	YouTubeUserID         string    `firestore:"youtube_user_id" bson:"youtube_user_id"`
	DisplayName           string    `firestore:"display_name" bson:"display_name"`
	ProfileImageURL       string    `firestore:"profile_image_url" bson:"profile_image_url"`
	LastStudyTime         time.Time `firestore:"last_study_time" bson:"last_study_time"`
	TotalStudyTime        int       `firestore:"total_study_time" bson:"total_study_time"`
	TotalStudySessions    int       `firestore:"total_study_sessions" bson:"total_study_sessions"`
	ConsecutiveStudyDays  int       `firestore:"consecutive_study_days" bson:"consecutive_study_days"`
	LastStreakDate        time.Time `firestore:"last_streak_date" bson:"last_streak_date"`
	RankPoint             int       `firestore:"rank_point" bson:"rank_point"`
	CurrentStudySessionID string    `firestore:"current_study_session_id,omitempty" bson:"current_study_session_id,omitempty"`
}

type LiveChatHistoryDoc struct {
	AuthorChannelId       string    `json:"author_channel_id" firestore:"author-channel-id" bson:"author-channel-id"`
	AuthorDisplayName     string    `json:"author_display_name" firestore:"author-display-name" bson:"author-display-name"`
	AuthorProfileImageUrl string    `json:"author_profile_image_url" firestore:"author-profile-image-url" bson:"author-profile-image-url"`
	AuthorIsChatModerator bool      `json:"author_is_chat_moderator" firestore:"author-is-chat-moderator" bson:"author-is-chat-moderator"`
	Id                    string    `json:"id" firestore:"id" bson:"id"`                     // メッセージのID。APIで取得するliveChatMessages resourceで定義されているid
	LiveChatId            string    `json:"live_chat_id" firestore:"live-chat-id" bson:"live-chat-id"` // ライブ配信ごとのid。ずっと続く配信だと不変。
	MessageText           string    `json:"message_text" firestore:"message-text" bson:"message-text"`
	PublishedAt           time.Time `json:"published_at" firestore:"published-at" bson:"published-at"`
	Type                  string    `json:"type" firestore:"type" bson:"type"`
}

type UserActivityType string

const (
	EnterRoomActivity  UserActivityType = "enter-room"
	ExitRoomActivity   UserActivityType = "exit-room"
	StartBreakActivity UserActivityType = "start-break"
	EndBreakActivity   UserActivityType = "end-break"
)

type UserActivityDoc struct {
	UserId       string           `json:"user_id" firestore:"user-id" bson:"user-id"`
	ActivityType UserActivityType `json:"activity_type" firestore:"activity-type" bson:"activity-type"`
	SeatId       int              `json:"seat_id" firestore:"seat-id" bson:"seat-id"`
	IsMemberSeat bool             `json:"is_member_seat" firestore:"is-member-seat" bson:"is-member-seat"`
	TakenAt      time.Time        `json:"taken_at" firestore:"taken-at" bson:"taken-at"`
}

type MenuDoc struct {
	Code string `json:"code" firestore:"code" bson:"code"`
	Name string `json:"name" firestore:"name" bson:"name"`
}

type OrderHistoryDoc struct {
	UserId       string    `json:"user_id" firestore:"user-id" bson:"user-id"`
	MenuCode     string    `json:"menu_code" firestore:"menu-code" bson:"menu-code"`
	SeatId       int       `json:"seat_id" firestore:"seat-id" bson:"seat-id"`
	IsMemberSeat bool      `json:"is_member_seat" firestore:"is-member-seat" bson:"is-member-seat"`
	OrderedAt    time.Time `json:"ordered_at" firestore:"ordered-at" bson:"ordered-at"`
}

type WorkNameTrendDoc struct {
	WorkName string                 `json:"work_name" firestore:"work_name" bson:"work_name"`
	Ranking  []WorkNameTrendRanking `json:"ranking" firestore:"ranking" bson:"ranking"`
	RankedAt time.Time              `json:"ranked_at" firestore:"ranked-at" bson:"ranked-at"`
}

type WorkNameTrendRanking struct {
	Rank     int      `json:"rank" firestore:"rank" bson:"rank"`
	Genre    string   `json:"genre" firestore:"genre" bson:"genre"`
	Count    int      `json:"count" firestore:"count" bson:"count"`
	Examples []string `json:"examples" firestore:"examples" bson:"examples"`
}
