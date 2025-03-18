package utils

import (
	"testing"
	"time"

	"app.modules/core/repository"

	"github.com/stretchr/testify/assert"
)

func TestSecondsOfDay(t *testing.T) {
	tests := []struct {
		name            string
		inputTime       time.Time
		expectedSeconds int
	}{
		{
			name:      "Time with hours, minutes, and seconds",
			inputTime: time.Date(2021, 0, 1, 3, 40, 4, 0, JapanLocation()),
			expectedSeconds: int((time.Duration(3)*time.Hour +
				time.Duration(40)*time.Minute +
				time.Duration(4)*time.Second).Seconds()),
		},
		{
			name:            "Midnight (0 seconds)",
			inputTime:       time.Date(2021, 10, 1, 0, 0, 0, 0, JapanLocation()),
			expectedSeconds: 0,
		},
		{
			name:            "Noon",
			inputTime:       time.Date(2021, 10, 1, 12, 0, 0, 0, JapanLocation()),
			expectedSeconds: int((time.Duration(12) * time.Hour).Seconds()),
		},
		{
			name:            "End of day",
			inputTime:       time.Date(2021, 10, 1, 23, 59, 59, 0, JapanLocation()),
			expectedSeconds: int((time.Duration(23)*time.Hour + time.Duration(59)*time.Minute + time.Duration(59)*time.Second).Seconds()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seconds := SecondsOfDay(tt.inputTime)
			assert.Equal(t, tt.expectedSeconds, seconds)
		})
	}
}

func TestDivideStringEqually(t *testing.T) {
	tests := []struct {
		name        string
		batchSize   int
		inputValues []string
		expected    [][]string
	}{
		{
			name:      "Divide 5 strings into 2 batches",
			batchSize: 2,
			inputValues: []string{
				"1", "2", "3", "4", "5",
			},
			expected: [][]string{
				{
					"1", "3", "5",
				},
				{
					"2", "4",
				},
			},
		},
		{
			name:      "Divide 6 strings into 3 batches",
			batchSize: 3,
			inputValues: []string{
				"a", "b", "c", "d", "e", "f",
			},
			expected: [][]string{
				{"a", "d"},
				{"b", "e"},
				{"c", "f"},
			},
		},
		{
			name:      "Batch size equals array size",
			batchSize: 3,
			inputValues: []string{
				"x", "y", "z",
			},
			expected: [][]string{
				{"x"},
				{"y"},
				{"z"},
			},
		},
		{
			name:      "Batch size larger than array size",
			batchSize: 5,
			inputValues: []string{
				"1", "2", "3",
			},
			expected: [][]string{
				{"1"},
				{"2"},
				{"3"},
				nil,
				nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DivideStringEqually(tt.batchSize, tt.inputValues)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchEmojiCommandString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Regular text",
			input:    "hello",
			expected: false,
		},
		{
			name:     "Generic emoji command",
			input:    ":_command:",
			expected: true,
		},
		{
			name:     "Specific emoji command - 360 minutes",
			input:    TestEmoji360Min0,
			expected: true,
		},
		{
			name:     "Specific emoji command - info details",
			input:    TestEmojiInfoD0,
			expected: true,
		},
		{
			name:     "Text with emoji command",
			input:    "dev" + TestEmojiIn0,
			expected: true,
		},
		{
			name:     "Similar but not emoji command",
			input:    ":not_command",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchEmojiCommandString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDurationToString(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "Zero duration",
			input:    time.Duration(0),
			expected: "0分",
		},
		{
			name:     "Minutes only",
			input:    1 * time.Minute,
			expected: "1分",
		},
		{
			name:     "Hours with zero minutes",
			input:    1 * time.Hour,
			expected: "1時間0分",
		},
		{
			name:     "Hours and minutes with seconds (seconds ignored)",
			input:    1*time.Hour + 1*time.Minute + 1*time.Second,
			expected: "1時間1分",
		},
		{
			name:     "Hours and minutes with milliseconds (milliseconds ignored)",
			input:    1*time.Hour + 1*time.Minute + 1*time.Second + 1*time.Millisecond,
			expected: "1時間1分",
		},
		{
			name:     "Multiple hours with minutes",
			input:    24*time.Hour + 1*time.Minute,
			expected: "24時間1分",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DurationToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRealTimeDailyTotalStudyDurationOfSeat(t *testing.T) {
	tests := []struct {
		name             string
		seat             repository.SeatDoc
		now              time.Time
		expectedDuration time.Duration
		expectError      bool
	}{
		{
			name: "Same time (no duration)",
			seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			now:              time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			expectedDuration: 0,
			expectError:      false,
		},
		{
			name: "Work state with time elapsed",
			seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			now:              time.Date(2021, 1, 1, 1, 1, 1, 0, JapanLocation()),
			expectedDuration: 1*time.Hour + 1*time.Minute + 1*time.Second,
			expectError:      false,
		},
		{
			name: "Work state with short duration",
			seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			now:              time.Date(2021, 1, 1, 0, 1, 0, 0, JapanLocation()),
			expectedDuration: 1 * time.Minute,
			expectError:      false,
		},
		{
			name: "Work state with previous accumulated time",
			seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 12, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: int((time.Hour).Seconds()),
			},
			now:              time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			expectedDuration: 1*time.Hour + 30*time.Minute,
			expectError:      false,
		},
		{
			name: "Break state (no additional time)",
			seat: repository.SeatDoc{
				State:                  repository.BreakState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 12, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: int((2 * time.Hour).Seconds()),
			},
			now:              time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			expectedDuration: 2 * time.Hour,
			expectError:      false,
		},
		{
			name: "Invalid state",
			seat: repository.SeatDoc{
				State:                  "invalid_state",
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 12, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: int((2 * time.Hour).Seconds()),
			},
			now:              time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			expectedDuration: 0,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := RealTimeDailyTotalStudyDurationOfSeat(tt.seat, tt.now)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedDuration, duration)
		})
	}
}

func TestNoNegativeDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected time.Duration
	}{
		{
			name:     "Positive duration",
			input:    10 * time.Minute,
			expected: 10 * time.Minute,
		},
		{
			name:     "Zero duration",
			input:    0,
			expected: 0,
		},
		{
			name:     "Negative duration",
			input:    -5 * time.Minute,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NoNegativeDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{
			name:     "Element exists in slice",
			slice:    []string{"apple", "banana", "cherry"},
			element:  "banana",
			expected: true,
		},
		{
			name:     "Element does not exist in slice",
			slice:    []string{"apple", "banana", "cherry"},
			element:  "orange",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			element:  "apple",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.slice, tt.element)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDateEqualJST(t *testing.T) {
	tests := []struct {
		name     string
		date1    time.Time
		date2    time.Time
		expected bool
	}{
		{
			name:     "Same date",
			date1:    time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			date2:    time.Date(2021, 1, 1, 18, 45, 0, 0, JapanLocation()),
			expected: true,
		},
		{
			name:     "Different date",
			date1:    time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			date2:    time.Date(2021, 1, 2, 12, 30, 0, 0, JapanLocation()),
			expected: false,
		},
		{
			name:     "Different month",
			date1:    time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			date2:    time.Date(2021, 2, 1, 12, 30, 0, 0, JapanLocation()),
			expected: false,
		},
		{
			name:     "Different year",
			date1:    time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			date2:    time.Date(2022, 1, 1, 12, 30, 0, 0, JapanLocation()),
			expected: false,
		},
		{
			name:     "Zero time values",
			date1:    time.Time{},
			date2:    time.Time{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DateEqualJST(tt.date1, tt.date2)
			assert.Equal(t, tt.expected, result)
		})
	}
}
