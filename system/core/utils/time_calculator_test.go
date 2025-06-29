package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTimeCalculator(t *testing.T) {
	tc := NewTimeCalculator()
	assert.NotNil(t, tc)
	
	// Check that the time is recent (within 1 second)
	now := JstNow()
	diff := now.Sub(tc.now)
	if diff < 0 {
		diff = -diff
	}
	assert.True(t, diff < time.Second)
}

func TestNewTimeCalculatorWithTime(t *testing.T) {
	testTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(testTime)
	
	assert.Equal(t, testTime, tc.now)
}

func TestTimeCalculator_RemainingMinutes(t *testing.T) {
	baseTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(baseTime)
	
	tests := []struct {
		name  string
		until time.Time
		want  int
	}{
		{
			name:  "30 minutes remaining",
			until: baseTime.Add(30 * time.Minute),
			want:  30,
		},
		{
			name:  "1 hour remaining",
			until: baseTime.Add(60 * time.Minute),
			want:  60,
		},
		{
			name:  "already expired",
			until: baseTime.Add(-30 * time.Minute),
			want:  0, // NoNegativeDuration should make this 0
		},
		{
			name:  "exactly now",
			until: baseTime,
			want:  0,
		},
		{
			name:  "1.5 minutes remaining",
			until: baseTime.Add(90 * time.Second),
			want:  1, // Should truncate to 1 minute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tc.RemainingMinutes(tt.until))
		})
	}
}

func TestTimeCalculator_ElapsedMinutes(t *testing.T) {
	baseTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(baseTime)
	
	tests := []struct {
		name string
		from time.Time
		want int
	}{
		{
			name: "30 minutes elapsed",
			from: baseTime.Add(-30 * time.Minute),
			want: 30,
		},
		{
			name: "1 hour elapsed",
			from: baseTime.Add(-60 * time.Minute),
			want: 60,
		},
		{
			name: "future time",
			from: baseTime.Add(30 * time.Minute),
			want: -30,
		},
		{
			name: "exactly now",
			from: baseTime,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tc.ElapsedMinutes(tt.from))
		})
	}
}

func TestTimeCalculator_ElapsedSeconds(t *testing.T) {
	baseTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(baseTime)
	
	tests := []struct {
		name string
		from time.Time
		want int
	}{
		{
			name: "30 seconds elapsed",
			from: baseTime.Add(-30 * time.Second),
			want: 30,
		},
		{
			name: "1 minute elapsed",
			from: baseTime.Add(-60 * time.Second),
			want: 60,
		},
		{
			name: "exactly now",
			from: baseTime,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tc.ElapsedSeconds(tt.from))
		})
	}
}

func TestTimeCalculator_IsExpired(t *testing.T) {
	baseTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(baseTime)
	
	tests := []struct {
		name  string
		until time.Time
		want  bool
	}{
		{
			name:  "future time",
			until: baseTime.Add(30 * time.Minute),
			want:  false,
		},
		{
			name:  "past time",
			until: baseTime.Add(-30 * time.Minute),
			want:  true,
		},
		{
			name:  "exactly now",
			until: baseTime,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tc.IsExpired(tt.until))
		})
	}
}

func TestTimeCalculator_IsWithinDuration(t *testing.T) {
	baseTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(baseTime)
	
	tests := []struct {
		name     string
		target   time.Time
		duration time.Duration
		want     bool
	}{
		{
			name:     "within duration - past",
			target:   baseTime.Add(-5 * time.Minute),
			duration: 10 * time.Minute,
			want:     true,
		},
		{
			name:     "within duration - future",
			target:   baseTime.Add(5 * time.Minute),
			duration: 10 * time.Minute,
			want:     true,
		},
		{
			name:     "outside duration - past",
			target:   baseTime.Add(-15 * time.Minute),
			duration: 10 * time.Minute,
			want:     false,
		},
		{
			name:     "outside duration - future",
			target:   baseTime.Add(15 * time.Minute),
			duration: 10 * time.Minute,
			want:     false,
		},
		{
			name:     "exactly at boundary",
			target:   baseTime.Add(10 * time.Minute),
			duration: 10 * time.Minute,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tc.IsWithinDuration(tt.target, tt.duration))
		})
	}
}

func TestTimeCalculator_AddMinutes(t *testing.T) {
	baseTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(baseTime)
	
	tests := []struct {
		name    string
		minutes int
		want    time.Time
	}{
		{
			name:    "add 30 minutes",
			minutes: 30,
			want:    baseTime.Add(30 * time.Minute),
		},
		{
			name:    "add negative minutes",
			minutes: -15,
			want:    baseTime.Add(-15 * time.Minute),
		},
		{
			name:    "add zero minutes",
			minutes: 0,
			want:    baseTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tc.AddMinutes(tt.minutes))
		})
	}
}

func TestTimeCalculator_GetCurrentTime(t *testing.T) {
	baseTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	tc := NewTimeCalculatorWithTime(baseTime)
	
	assert.Equal(t, baseTime, tc.GetCurrentTime())
}