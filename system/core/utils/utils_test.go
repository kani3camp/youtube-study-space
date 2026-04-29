package utils

import (
	"regexp"
	"testing"
	"time"

	"app.modules/core/repository"
	"app.modules/core/timeutil"

	"github.com/stretchr/testify/assert"
)

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
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, timeutil.JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			now:              time.Date(2021, 1, 1, 0, 0, 0, 0, timeutil.JapanLocation()),
			expectedDuration: 0,
			expectError:      false,
		},
		{
			name: "Work state with time elapsed",
			seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, timeutil.JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			now:              time.Date(2021, 1, 1, 1, 1, 1, 0, timeutil.JapanLocation()),
			expectedDuration: 1*time.Hour + 1*time.Minute + 1*time.Second,
			expectError:      false,
		},
		{
			name: "Work state with short duration",
			seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, timeutil.JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			now:              time.Date(2021, 1, 1, 0, 1, 0, 0, timeutil.JapanLocation()),
			expectedDuration: 1 * time.Minute,
			expectError:      false,
		},
		{
			name: "Work state with previous accumulated time",
			seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 12, 0, 0, 0, timeutil.JapanLocation()),
				DailyCumulativeWorkSec: int((time.Hour).Seconds()),
			},
			now:              time.Date(2021, 1, 1, 12, 30, 0, 0, timeutil.JapanLocation()),
			expectedDuration: 1*time.Hour + 30*time.Minute,
			expectError:      false,
		},
		{
			name: "Break state (no additional time)",
			seat: repository.SeatDoc{
				State:                  repository.BreakState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 12, 0, 0, 0, timeutil.JapanLocation()),
				DailyCumulativeWorkSec: int((2 * time.Hour).Seconds()),
			},
			now:              time.Date(2021, 1, 1, 12, 30, 0, 0, timeutil.JapanLocation()),
			expectedDuration: 2 * time.Hour,
			expectError:      false,
		},
		{
			name: "Invalid state",
			seat: repository.SeatDoc{
				State:                  "invalid_state",
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 12, 0, 0, 0, timeutil.JapanLocation()),
				DailyCumulativeWorkSec: int((2 * time.Hour).Seconds()),
			},
			now:              time.Date(2021, 1, 1, 12, 30, 0, 0, timeutil.JapanLocation()),
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

func TestGenerateSessionID(t *testing.T) {
	hexRegex := regexp.MustCompile("^[0-9a-f]{32}$")

	t.Run("format", func(t *testing.T) {
		const n = 5
		for i := 0; i < n; i++ {
			result := GenerateSessionID()
			assert.Len(t, result, 32, "session ID must be 32 characters")
			assert.True(t, hexRegex.MatchString(result), "session ID must contain only hex digits (0-9, a-f), no hyphens")
		}
	})
}

func TestTruncateStringUTF8(t *testing.T) {
	t.Run("negative maxBytes should not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			got := TruncateStringUTF8("hello", -1)
			assert.Equal(t, "", got)
		})
	})

	t.Run("zero maxBytes should not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			got := TruncateStringUTF8("hello", 0)
			assert.Equal(t, "", got)
		})
	})

	t.Run("utf8 rune boundary should not split", func(t *testing.T) {
		// Each "あ" is 3 bytes in UTF-8. So 3 bytes should keep only the first rune.
		s := "あいう"
		assert.Equal(t, "あ", TruncateStringUTF8(s, 3))
	})
}

func TestTruncateStringRunes(t *testing.T) {
	t.Run("negative maxRunes should not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			got := TruncateStringRunes("hello", -1)
			assert.Equal(t, "", got)
		})
	})

	t.Run("rune boundary should not split", func(t *testing.T) {
		assert.Equal(t, "あい", TruncateStringRunes("あいう", 2))
		assert.Equal(t, "勉強", TruncateStringRunes("勉強🚀", 2))
	})
}

func TestSplitStringRunes(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		assert.Nil(t, SplitStringRunes("", 10))
	})

	t.Run("split preserves utf8 boundaries", func(t *testing.T) {
		got := SplitStringRunes("勉強🚀最高", 2)
		assert.Equal(t, []string{"勉強", "🚀最", "高"}, got)
	})
}
