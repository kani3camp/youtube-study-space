package utils

import (
	"testing"

	"app.modules/core/repository"
	"github.com/stretchr/testify/assert"
)

func TestGetSeatAppearance(t *testing.T) {
	tests := []struct {
		name           string
		totalStudySec  int
		rankVisible    bool
		rp             int
		favoriteColor  string
		expected       repository.SeatAppearance
		expectedErrMsg string
	}{
		{
			name:          "Normal case with rank not visible and no favorite color",
			totalStudySec: 3600 * 3, // 3 hours
			rankVisible:   false,
			rp:            0,
			favoriteColor: "",
			expected: repository.SeatAppearance{
				ColorCode1:           ColorHours0To5,
				ColorCode2:           "",
				NumStars:             0,
				ColorGradientEnabled: false,
			},
		},
		{
			name:          "Normal case with rank visible",
			totalStudySec: 3600 * 3, // 3 hours
			rankVisible:   true,
			rp:            15000,
			favoriteColor: "",
			expected: repository.SeatAppearance{
				ColorCode1:           ColorRank2,
				ColorCode2:           ColorRank3,
				NumStars:             0,
				ColorGradientEnabled: true,
			},
		},
		{
			name:          "With favorite color and enough study time",
			totalStudySec: 3600 * 1001, // Over 1000 hours
			rankVisible:   false,
			rp:            0,
			favoriteColor: "#FF00FF",
			expected: repository.SeatAppearance{
				ColorCode1:           "#FF00FF",
				ColorCode2:           "",
				NumStars:             1,
				ColorGradientEnabled: false,
			},
		},
		{
			name:          "With favorite color but not enough study time",
			totalStudySec: 3600 * 999, // Under 1000 hours
			rankVisible:   false,
			rp:            0,
			favoriteColor: "#FF00FF",
			expected: repository.SeatAppearance{
				ColorCode1:           ColorHours700To1000,
				ColorCode2:           "",
				NumStars:             0,
				ColorGradientEnabled: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetSeatAppearance(tt.totalStudySec, tt.rankVisible, tt.rp, tt.favoriteColor)

			if tt.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCanUseFavoriteColor(t *testing.T) {
	tests := []struct {
		name          string
		totalStudySec int
		expected      bool
	}{
		{
			name:          "Below threshold",
			totalStudySec: 3600 * 999, // 999 hours
			expected:      false,
		},
		{
			name:          "At threshold",
			totalStudySec: 3600 * 1000, // 1000 hours
			expected:      true,
		},
		{
			name:          "Above threshold",
			totalStudySec: 3600 * 1001, // 1001 hours
			expected:      true,
		},
		{
			name:          "Zero study time",
			totalStudySec: 0,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanUseFavoriteColor(tt.totalStudySec)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTotalStudySecToNumStars(t *testing.T) {
	tests := []struct {
		name          string
		totalStudySec int
		expected      int
	}{
		{
			name:          "Zero study time",
			totalStudySec: 0,
			expected:      0,
		},
		{
			name:          "999 hours",
			totalStudySec: 3600 * 999,
			expected:      0,
		},
		{
			name:          "1000 hours",
			totalStudySec: 3600 * 1000,
			expected:      1,
		},
		{
			name:          "1999 hours",
			totalStudySec: 3600 * 1999,
			expected:      1,
		},
		{
			name:          "2000 hours",
			totalStudySec: 3600 * 2000,
			expected:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TotalStudySecToNumStars(tt.totalStudySec)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTotalStudySecToColorCode(t *testing.T) {
	tests := []struct {
		name          string
		totalStudySec int
		expected      string
		expectedErr   bool
	}{
		{
			name:          "0 hours",
			totalStudySec: 0,
			expected:      ColorHours0To5,
			expectedErr:   false,
		},
		{
			name:          "4 hours",
			totalStudySec: 3600 * 4,
			expected:      ColorHours0To5,
			expectedErr:   false,
		},
		{
			name:          "5 hours",
			totalStudySec: 3600 * 5,
			expected:      ColorHours5To10,
			expectedErr:   false,
		},
		{
			name:          "9 hours",
			totalStudySec: 3600 * 9,
			expected:      ColorHours5To10,
			expectedErr:   false,
		},
		{
			name:          "10 hours",
			totalStudySec: 3600 * 10,
			expected:      ColorHours10To20,
			expectedErr:   false,
		},
		{
			name:          "1001 hours",
			totalStudySec: 3600 * 1001,
			expected:      ColorHoursFrom1000,
			expectedErr:   false,
		},
		{
			name:          "Negative hours",
			totalStudySec: -3600,
			expected:      "",
			expectedErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TotalStudySecToColorCode(tt.totalStudySec)

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTotalStudyHoursToColorCode(t *testing.T) {
	tests := []struct {
		name        string
		totalHours  int
		expected    string
		expectedErr bool
	}{
		{
			name:        "0 hours",
			totalHours:  0,
			expected:    ColorHours0To5,
			expectedErr: false,
		},
		{
			name:        "4 hours",
			totalHours:  4,
			expected:    ColorHours0To5,
			expectedErr: false,
		},
		{
			name:        "5 hours",
			totalHours:  5,
			expected:    ColorHours5To10,
			expectedErr: false,
		},
		{
			name:        "9 hours",
			totalHours:  9,
			expected:    ColorHours5To10,
			expectedErr: false,
		},
		{
			name:        "10 hours",
			totalHours:  10,
			expected:    ColorHours10To20,
			expectedErr: false,
		},
		{
			name:        "19 hours",
			totalHours:  19,
			expected:    ColorHours10To20,
			expectedErr: false,
		},
		{
			name:        "20 hours",
			totalHours:  20,
			expected:    ColorHours20To30,
			expectedErr: false,
		},
		{
			name:        "29 hours",
			totalHours:  29,
			expected:    ColorHours20To30,
			expectedErr: false,
		},
		{
			name:        "30 hours",
			totalHours:  30,
			expected:    ColorHours30To50,
			expectedErr: false,
		},
		{
			name:        "49 hours",
			totalHours:  49,
			expected:    ColorHours30To50,
			expectedErr: false,
		},
		{
			name:        "50 hours",
			totalHours:  50,
			expected:    ColorHours50To70,
			expectedErr: false,
		},
		{
			name:        "69 hours",
			totalHours:  69,
			expected:    ColorHours50To70,
			expectedErr: false,
		},
		{
			name:        "70 hours",
			totalHours:  70,
			expected:    ColorHours70To100,
			expectedErr: false,
		},
		{
			name:        "99 hours",
			totalHours:  99,
			expected:    ColorHours70To100,
			expectedErr: false,
		},
		{
			name:        "100 hours",
			totalHours:  100,
			expected:    ColorHours100To150,
			expectedErr: false,
		},
		{
			name:        "149 hours",
			totalHours:  149,
			expected:    ColorHours100To150,
			expectedErr: false,
		},
		{
			name:        "150 hours",
			totalHours:  150,
			expected:    ColorHours150To200,
			expectedErr: false,
		},
		{
			name:        "199 hours",
			totalHours:  199,
			expected:    ColorHours150To200,
			expectedErr: false,
		},
		{
			name:        "200 hours",
			totalHours:  200,
			expected:    ColorHours200To300,
			expectedErr: false,
		},
		{
			name:        "299 hours",
			totalHours:  299,
			expected:    ColorHours200To300,
			expectedErr: false,
		},
		{
			name:        "300 hours",
			totalHours:  300,
			expected:    ColorHours300To400,
			expectedErr: false,
		},
		{
			name:        "399 hours",
			totalHours:  399,
			expected:    ColorHours300To400,
			expectedErr: false,
		},
		{
			name:        "400 hours",
			totalHours:  400,
			expected:    ColorHours400To500,
			expectedErr: false,
		},
		{
			name:        "499 hours",
			totalHours:  499,
			expected:    ColorHours400To500,
			expectedErr: false,
		},
		{
			name:        "500 hours",
			totalHours:  500,
			expected:    ColorHours500To700,
			expectedErr: false,
		},
		{
			name:        "699 hours",
			totalHours:  699,
			expected:    ColorHours500To700,
			expectedErr: false,
		},
		{
			name:        "700 hours",
			totalHours:  700,
			expected:    ColorHours700To1000,
			expectedErr: false,
		},
		{
			name:        "999 hours",
			totalHours:  999,
			expected:    ColorHours700To1000,
			expectedErr: false,
		},
		{
			name:        "1000 hours",
			totalHours:  1000,
			expected:    ColorHoursFrom1000,
			expectedErr: false,
		},
		{
			name:        "1001 hours",
			totalHours:  1001,
			expected:    ColorHoursFrom1000,
			expectedErr: false,
		},
		{
			name:        "Negative hours",
			totalHours:  -1,
			expected:    "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TotalStudyHoursToColorCode(tt.totalHours)

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsIncludedInColorNames(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "Valid color name - 白",
			value:    ColorName0To5,
			expected: true,
		},
		{
			name:     "Valid color name - ピンク",
			value:    ColorNameFrom1000,
			expected: true,
		},
		{
			name:     "Invalid color name - empty string",
			value:    "",
			expected: false,
		},
		{
			name:     "Invalid color name - random string",
			value:    "RandomColor",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIncludedInColorNames(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorNameToColorCode(t *testing.T) {
	tests := []struct {
		name      string
		colorName string
		expected  string
	}{
		{
			name:      "白",
			colorName: ColorName0To5,
			expected:  ColorHours0To5,
		},
		{
			name:      "うすももいろ",
			colorName: ColorName5To10,
			expected:  ColorHours5To10,
		},
		{
			name:      "ピンク",
			colorName: ColorNameFrom1000,
			expected:  ColorHoursFrom1000,
		},
		{
			name:      "Invalid color name",
			colorName: "InvalidColor",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorNameToColorCode(tt.colorName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorCodeToColorName(t *testing.T) {
	tests := []struct {
		name      string
		colorCode string
		expected  string
	}{
		{
			name:      "White color code",
			colorCode: ColorHours0To5,
			expected:  ColorName0To5,
		},
		{
			name:      "Pink color code",
			colorCode: ColorHoursFrom1000,
			expected:  ColorNameFrom1000,
		},
		{
			name:      "Invalid color code",
			colorCode: "#123456",
			expected:  "不明",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ColorCodeToColorName(tt.colorCode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRankPointToColorCodePair(t *testing.T) {
	tests := []struct {
		name          string
		rp            int
		expectedCode1 string
		expectedCode2 string
	}{
		{
			name:          "Rank point 0",
			rp:            0,
			expectedCode1: ColorRank1,
			expectedCode2: ColorRank2,
		},
		{
			name:          "Rank point 9999",
			rp:            9999,
			expectedCode1: ColorRank1,
			expectedCode2: ColorRank2,
		},
		{
			name:          "Rank point 10000",
			rp:            10000,
			expectedCode1: ColorRank2,
			expectedCode2: ColorRank3,
		},
		{
			name:          "Rank point 19999",
			rp:            19999,
			expectedCode1: ColorRank2,
			expectedCode2: ColorRank3,
		},
		{
			name:          "Rank point 20000",
			rp:            20000,
			expectedCode1: ColorRank3,
			expectedCode2: ColorRank4,
		},
		{
			name:          "Rank point 29999",
			rp:            29999,
			expectedCode1: ColorRank3,
			expectedCode2: ColorRank4,
		},
		{
			name:          "Rank point 30000",
			rp:            30000,
			expectedCode1: ColorRank4,
			expectedCode2: ColorRank5,
		},
		{
			name:          "Rank point 39999",
			rp:            39999,
			expectedCode1: ColorRank4,
			expectedCode2: ColorRank5,
		},
		{
			name:          "Rank point 40000",
			rp:            40000,
			expectedCode1: ColorRank5,
			expectedCode2: ColorRank6,
		},
		{
			name:          "Rank point 49999",
			rp:            49999,
			expectedCode1: ColorRank5,
			expectedCode2: ColorRank6,
		},
		{
			name:          "Rank point 50000",
			rp:            50000,
			expectedCode1: ColorRank6,
			expectedCode2: ColorRank7,
		},
		{
			name:          "Rank point 59999",
			rp:            59999,
			expectedCode1: ColorRank6,
			expectedCode2: ColorRank7,
		},
		{
			name:          "Rank point 60000",
			rp:            60000,
			expectedCode1: ColorRank7,
			expectedCode2: ColorRank8,
		},
		{
			name:          "Rank point 69999",
			rp:            69999,
			expectedCode1: ColorRank7,
			expectedCode2: ColorRank8,
		},
		{
			name:          "Rank point 70000",
			rp:            70000,
			expectedCode1: ColorRank8,
			expectedCode2: ColorRank9,
		},
		{
			name:          "Rank point 79999",
			rp:            79999,
			expectedCode1: ColorRank8,
			expectedCode2: ColorRank9,
		},
		{
			name:          "Rank point 80000",
			rp:            80000,
			expectedCode1: ColorRank9,
			expectedCode2: ColorRank10,
		},
		{
			name:          "Rank point 89999",
			rp:            89999,
			expectedCode1: ColorRank9,
			expectedCode2: ColorRank10,
		},
		{
			name:          "Rank point 90000",
			rp:            90000,
			expectedCode1: ColorRank10,
			expectedCode2: ColorRank10andMore,
		},
		{
			name:          "Rank point 100000",
			rp:            100000,
			expectedCode1: ColorRank10,
			expectedCode2: ColorRank10andMore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code1, code2 := RankPointToColorCodePair(tt.rp)
			assert.Equal(t, tt.expectedCode1, code1)
			assert.Equal(t, tt.expectedCode2, code2)
		})
	}
}
