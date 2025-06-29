package constants

import "time"

// Command execution constants
const (
	// Retry configuration for main bot loop
	MaxRetryIntervalSeconds        = 300
	RetryIntervalCalculationBase   = 1.2
	MinimumTryTimesToNotify        = 2
	
	// Sleep intervals and timeouts
	DefaultSleepIntervalSeconds    = 3
	PageTokenRetrySleepSeconds     = 2
	ChatHistorySaveRetrySleepSeconds = 2
	
	// Work time limits (minutes)
	DefaultWorkTimeMinutes         = 60
	MinWorkTimeMinutes            = 10
	MaxWorkTimeMinutes            = 480 // 8 hours
	
	// Break time limits (minutes)
	DefaultBreakDurationMinutes   = 15
	MinBreakDurationMinutes       = 5
	MaxBreakDurationMinutes       = 60
	MinBreakIntervalMinutes       = 30
	
	// Seat management
	MaxSeatsPerRoom               = 100
	MinVacancyRatePercent         = 10
	DefaultSeatNumber             = 0 // Special value for "any available seat"
	
	// User activity limits
	MaxDailyOrderCount            = 3
	MaxContinuousInactivityDays   = 7
	LongTimeSittingCheckMinutes   = 60
	LongTimeSittingPenaltyMinutes = 30
	
	// Database and batch processing
	CollectionHistoryRetentionDays = 30
	BatchProcessBatchSize          = 100
	ParallelProcessingMaxWorkers   = 5
	
	// Text and display limits
	MaxWorkNameLength             = 50
	MaxReportMessageLength        = 200
	MaxStatusMessageLength        = 100
	
	// RP (Ranking Point) system
	BaseRPPerMinute               = 1
	BonusRPMultiplier             = 2
	InactivityRPPenalty           = 10
	ContinuousActivityBonus       = 5
)

// Time duration constants
const (
	OneMinute                     = 1 * time.Minute
	OneHour                       = 1 * time.Hour
	OneDay                        = 24 * time.Hour
	OneWeek                       = 7 * OneDay
	
	// Polling and check intervals
	DefaultPollingInterval        = 1 * time.Minute
	DesiredMaxSeatsCheckInterval  = 5 * time.Minute
	LongTimeSittingCheckInterval  = 1 * time.Hour
	DatabaseCleanupInterval       = 24 * time.Hour
)

// Error message templates
const (
	GenericErrorTemplate          = "command:error"
	MemberSeatForbiddenTemplate   = "member-seat-forbidden"
	MembershipDisabledTemplate    = "membership-disabled"
	SeatOccupiedTemplate          = "seat-occupied"
	UserNotInRoomTemplate         = "user-not-in-room"
	InvalidWorkTimeTemplate       = "invalid-work-time"
	InvalidBreakTimeTemplate      = "invalid-break-time"
)

// Command prefixes and formats
const (
	GeneralCommandPrefix          = "!"
	MemberCommandPrefix           = "/"
	EmojiCommandPrefix            = "🪑"
	EmojiSuffix                   = "💺"
	
	// Seat ID display formats
	RegularSeatFormat             = "%d"
	MemberSeatI18nKey            = "common:vip-seat-id"
)

// System configuration defaults
const (
	DefaultGCPRegion              = "asia-northeast1"
	DefaultBigQueryDataset        = "youtube_study_space"
	DefaultFirestoreCollection    = "seats"
	DefaultMemberCollection       = "member-seats"
	
	// Bot configuration
	BotConfigSpreadsheetRange     = "01:02"
	WordsFilterCacheMinutes       = 60
	
	// Discord notification limits
	MaxDiscordMessageLength       = 2000
	DiscordEmbedColorSuccess      = 0x00FF00
	DiscordEmbedColorError        = 0xFF0000
	DiscordEmbedColorWarning      = 0xFFAA00
)

// Validation limits
const (
	MinSeatID                     = 1
	MaxSeatID                     = MaxSeatsPerRoom
	MinUserID                     = 1
	
	// String validation
	MinDisplayNameLength          = 1
	MaxDisplayNameLength          = 50
	MinChannelIDLength            = 20
	MaxChannelIDLength            = 30
)

// Feature flags and toggles (usually loaded from config, but defaults here)
const (
	DefaultYoutubeMembershipEnabled = true
	DefaultFixedMaxSeatsEnabled     = false
	DefaultRankingSystemEnabled     = true
	DefaultAutoExitEnabled          = true
	DefaultBreakSystemEnabled       = true
)