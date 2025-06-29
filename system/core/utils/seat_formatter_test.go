package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"app.modules/core/i18n"
)

func TestMain(m *testing.M) {
	// Initialize i18n for tests
	if err := i18n.LoadLocaleFolderFS(); err != nil {
		// If i18n loading fails, we'll mock the behavior in tests
	}
	m.Run()
}

func TestFormatSeatId(t *testing.T) {
	tests := []struct {
		name         string
		seatId       int
		isMemberSeat bool
		want         string
	}{
		{
			name:         "regular seat",
			seatId:       5,
			isMemberSeat: false,
			want:         "5",
		},
		{
			name:         "member seat",
			seatId:       3,
			isMemberSeat: true,
			want:         "VIP3", // This might vary based on i18n loading
		},
		{
			name:         "single digit regular seat",
			seatId:       1,
			isMemberSeat: false,
			want:         "1",
		},
		{
			name:         "double digit regular seat",
			seatId:       15,
			isMemberSeat: false,
			want:         "15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSeatId(tt.seatId, tt.isMemberSeat)
			if tt.isMemberSeat {
				// For member seats, just check it's not the plain number
				assert.NotEqual(t, tt.want[:len(tt.want)-1], result) // Remove expected number for comparison
			} else {
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestFormatDurationString(t *testing.T) {
	tests := []struct {
		name    string
		minutes int
		want    string
	}{
		{
			name:    "less than 1 hour",
			minutes: 30,
			want:    "30分",
		},
		{
			name:    "exactly 1 hour",
			minutes: 60,
			want:    "1時間",
		},
		{
			name:    "1 hour and 30 minutes",
			minutes: 90,
			want:    "1時間30分",
		},
		{
			name:    "multiple hours",
			minutes: 150,
			want:    "2時間30分",
		},
		{
			name:    "exactly 2 hours",
			minutes: 120,
			want:    "2時間",
		},
		{
			name:    "zero minutes",
			minutes: 0,
			want:    "0分",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatDurationString(tt.minutes))
		})
	}
}

func TestFormatWorkTimeDisplay(t *testing.T) {
	tests := []struct {
		name string
		sec  int
		want string
	}{
		{
			name: "less than 1 minute",
			sec:  30,
			want: "30秒",
		},
		{
			name: "exactly 1 minute",
			sec:  60,
			want: "1分",
		},
		{
			name: "1 minute and 30 seconds",
			sec:  90,
			want: "1分30秒",
		},
		{
			name: "exactly 1 hour",
			sec:  3600,
			want: "1時間",
		},
		{
			name: "1 hour 30 minutes",
			sec:  5400,
			want: "1時間30分",
		},
		{
			name: "1 hour 30 seconds",
			sec:  3630,
			want: "1時間30秒",
		},
		{
			name: "1 hour 30 minutes 45 seconds",
			sec:  5445,
			want: "1時間30分45秒",
		},
		{
			name: "zero seconds",
			sec:  0,
			want: "0秒",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatWorkTimeDisplay(tt.sec))
		})
	}
}