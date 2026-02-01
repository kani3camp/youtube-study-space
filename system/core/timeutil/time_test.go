package timeutil

import (
	"testing"
	"time"

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
			date1:    time.Date(2021, 1, 1, 23, 59, 59, 0, JapanLocation()),
			date2:    time.Date(2021, 1, 2, 0, 0, 0, 0, JapanLocation()),
			expected: false,
		},
		{
			name:     "Different month",
			date1:    time.Date(2021, 1, 1, 12, 0, 0, 0, JapanLocation()),
			date2:    time.Date(2021, 2, 1, 12, 0, 0, 0, JapanLocation()),
			expected: false,
		},
		{
			name:     "Different year",
			date1:    time.Date(2021, 1, 1, 12, 0, 0, 0, JapanLocation()),
			date2:    time.Date(2022, 1, 1, 12, 0, 0, 0, JapanLocation()),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DateEqualJST(tt.date1, tt.date2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJapanLocation(t *testing.T) {
	loc := JapanLocation()
	assert.NotNil(t, loc)
	assert.Equal(t, "Asia/Tokyo", loc.String())
}

func TestJstNow(t *testing.T) {
	now := JstNow()
	assert.NotNil(t, now)
	// Verify it's in JST timezone
	assert.Equal(t, "Asia/Tokyo", now.Location().String())
}

func TestSecondsToHours(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected int
	}{
		{
			name:     "Zero seconds",
			seconds:  0,
			expected: 0,
		},
		{
			name:     "One hour",
			seconds:  3600,
			expected: 1,
		},
		{
			name:     "Two hours",
			seconds:  7200,
			expected: 2,
		},
		{
			name:     "Truncation test",
			seconds:  3599,
			expected: 0,
		},
		{
			name:     "Multiple hours with remainder",
			seconds:  10800 + 1800, // 3.5 hours
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecondsToHours(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}
