package constants

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant time.Duration
		expected time.Duration
	}{
		{"OneMinute", OneMinute, 1 * time.Minute},
		{"OneHour", OneHour, 1 * time.Hour},
		{"OneDay", OneDay, 24 * time.Hour},
		{"OneWeek", OneWeek, 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestWorkTimeLimits(t *testing.T) {
	assert.True(t, MinWorkTimeMinutes > 0, "Min work time should be positive")
	assert.True(t, MaxWorkTimeMinutes > MinWorkTimeMinutes, "Max work time should be greater than min")
	assert.True(t, DefaultWorkTimeMinutes >= MinWorkTimeMinutes, "Default work time should be at least min")
	assert.True(t, DefaultWorkTimeMinutes <= MaxWorkTimeMinutes, "Default work time should not exceed max")
}

func TestBreakTimeLimits(t *testing.T) {
	assert.True(t, MinBreakDurationMinutes > 0, "Min break duration should be positive")
	assert.True(t, MaxBreakDurationMinutes > MinBreakDurationMinutes, "Max break duration should be greater than min")
	assert.True(t, DefaultBreakDurationMinutes >= MinBreakDurationMinutes, "Default break duration should be at least min")
	assert.True(t, DefaultBreakDurationMinutes <= MaxBreakDurationMinutes, "Default break duration should not exceed max")
	assert.True(t, MinBreakIntervalMinutes > 0, "Min break interval should be positive")
}

func TestSeatManagementConstants(t *testing.T) {
	assert.True(t, MaxSeatsPerRoom > 0, "Max seats per room should be positive")
	assert.True(t, MinVacancyRatePercent >= 0, "Min vacancy rate should be non-negative")
	assert.True(t, MinVacancyRatePercent <= 100, "Min vacancy rate should not exceed 100%")
	assert.Equal(t, 0, DefaultSeatNumber, "Default seat number should be 0 (any available)")
}

func TestRetryConstants(t *testing.T) {
	assert.True(t, MaxRetryIntervalSeconds > 0, "Max retry interval should be positive")
	assert.True(t, RetryIntervalCalculationBase > 1.0, "Retry interval base should be greater than 1")
	assert.True(t, MinimumTryTimesToNotify > 0, "Minimum tries to notify should be positive")
}

func TestStringLengthLimits(t *testing.T) {
	assert.True(t, MaxWorkNameLength > 0, "Max work name length should be positive")
	assert.True(t, MaxReportMessageLength > 0, "Max report message length should be positive")
	assert.True(t, MaxStatusMessageLength > 0, "Max status message length should be positive")
	
	assert.True(t, MinDisplayNameLength > 0, "Min display name length should be positive")
	assert.True(t, MaxDisplayNameLength > MinDisplayNameLength, "Max display name length should be greater than min")
	
	assert.True(t, MinChannelIDLength > 0, "Min channel ID length should be positive")
	assert.True(t, MaxChannelIDLength > MinChannelIDLength, "Max channel ID length should be greater than min")
}

func TestSeatIDLimits(t *testing.T) {
	assert.True(t, MinSeatID > 0, "Min seat ID should be positive")
	assert.True(t, MaxSeatID > MinSeatID, "Max seat ID should be greater than min")
	assert.True(t, MaxSeatID <= MaxSeatsPerRoom, "Max seat ID should not exceed max seats per room")
}

func TestRPSystemConstants(t *testing.T) {
	assert.True(t, BaseRPPerMinute > 0, "Base RP per minute should be positive")
	assert.True(t, BonusRPMultiplier > 1, "Bonus RP multiplier should be greater than 1")
	assert.True(t, InactivityRPPenalty > 0, "Inactivity RP penalty should be positive")
	assert.True(t, ContinuousActivityBonus > 0, "Continuous activity bonus should be positive")
}

func TestCommandPrefixes(t *testing.T) {
	assert.NotEmpty(t, GeneralCommandPrefix, "General command prefix should not be empty")
	assert.NotEmpty(t, MemberCommandPrefix, "Member command prefix should not be empty")
	assert.NotEmpty(t, EmojiCommandPrefix, "Emoji command prefix should not be empty")
	assert.NotEmpty(t, EmojiSuffix, "Emoji suffix should not be empty")
	
	assert.NotEqual(t, GeneralCommandPrefix, MemberCommandPrefix, "Command prefixes should be different")
}

func TestErrorTemplates(t *testing.T) {
	templates := []string{
		GenericErrorTemplate,
		MemberSeatForbiddenTemplate,
		MembershipDisabledTemplate,
		SeatOccupiedTemplate,
		UserNotInRoomTemplate,
		InvalidWorkTimeTemplate,
		InvalidBreakTimeTemplate,
	}
	
	for _, template := range templates {
		assert.NotEmpty(t, template, "Error template should not be empty")
	}
}

func TestDiscordConstants(t *testing.T) {
	assert.True(t, MaxDiscordMessageLength > 0, "Max Discord message length should be positive")
	assert.True(t, MaxDiscordMessageLength <= 2000, "Max Discord message length should not exceed Discord's limit")
	
	// Test color constants are valid hex colors (non-negative)
	assert.True(t, DiscordEmbedColorSuccess >= 0, "Discord success color should be non-negative")
	assert.True(t, DiscordEmbedColorError >= 0, "Discord error color should be non-negative")
	assert.True(t, DiscordEmbedColorWarning >= 0, "Discord warning color should be non-negative")
}

func TestFeatureFlags(t *testing.T) {
	// Test that feature flags are boolean constants
	// These tests mainly ensure the constants are defined and have reasonable defaults
	flags := []bool{
		DefaultYoutubeMembershipEnabled,
		DefaultFixedMaxSeatsEnabled,
		DefaultRankingSystemEnabled,
		DefaultAutoExitEnabled,
		DefaultBreakSystemEnabled,
	}
	
	for i, flag := range flags {
		assert.IsType(t, true, flag, "Feature flag %d should be boolean", i)
	}
}

func TestBatchProcessingConstants(t *testing.T) {
	assert.True(t, CollectionHistoryRetentionDays > 0, "Collection history retention days should be positive")
	assert.True(t, BatchProcessBatchSize > 0, "Batch process batch size should be positive")
	assert.True(t, ParallelProcessingMaxWorkers > 0, "Parallel processing max workers should be positive")
	assert.True(t, ParallelProcessingMaxWorkers <= 10, "Parallel processing max workers should be reasonable")
}

func TestSystemConfigurationDefaults(t *testing.T) {
	assert.NotEmpty(t, DefaultGCPRegion, "Default GCP region should not be empty")
	assert.NotEmpty(t, DefaultBigQueryDataset, "Default BigQuery dataset should not be empty")
	assert.NotEmpty(t, DefaultFirestoreCollection, "Default Firestore collection should not be empty")
	assert.NotEmpty(t, DefaultMemberCollection, "Default member collection should not be empty")
	
	assert.NotEqual(t, DefaultFirestoreCollection, DefaultMemberCollection, "Collections should be different")
}

func TestIntervalConsistency(t *testing.T) {
	// Test that intervals make sense relative to each other
	assert.True(t, DefaultPollingInterval <= DesiredMaxSeatsCheckInterval, 
		"Polling interval should not be longer than max seats check interval")
	assert.True(t, DesiredMaxSeatsCheckInterval <= LongTimeSittingCheckInterval,
		"Max seats check interval should not be longer than long sitting check interval")
}