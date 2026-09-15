package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"app.modules/core/repository"
)

func TestGetSeatAppearance(t *testing.T) {
	tests := []struct {
		name          string
		totalStudySec int
		rankVisible   bool
		rp            int
		favoriteColor string
		expected      repository.SeatAppearance
	}{
		{
			name:          "rank hidden without favorite color",
			totalStudySec: 3600 * 3,
			rankVisible:   false,
			rp:            0,
			expected: repository.SeatAppearance{
				SchemaVersion: SeatAppearanceSchemaVersion,
				TopBarColor:   ColorHours0To5,
				Rank:          1,
				RankVisible:   false,
				NumStars:      0,
			},
		},
		{
			name:          "rank visible still uses cumulative study color",
			totalStudySec: 3600 * 3,
			rankVisible:   true,
			rp:            15000,
			expected: repository.SeatAppearance{
				SchemaVersion: SeatAppearanceSchemaVersion,
				TopBarColor:   ColorHours0To5,
				Rank:          2,
				RankVisible:   true,
				NumStars:      0,
			},
		},
		{
			name:          "favorite color wins after 1000 hours",
			totalStudySec: 3600 * 1001,
			rankVisible:   true,
			rp:            45000,
			favoriteColor: "#123456",
			expected: repository.SeatAppearance{
				SchemaVersion: SeatAppearanceSchemaVersion,
				TopBarColor:   "#123456",
				Rank:          5,
				RankVisible:   true,
				NumStars:      1,
			},
		},
		{
			name:          "favorite color unavailable below 1000 hours",
			totalStudySec: 3600 * 999,
			rankVisible:   false,
			rp:            0,
			favoriteColor: "#FF00FF",
			expected: repository.SeatAppearance{
				SchemaVersion: SeatAppearanceSchemaVersion,
				TopBarColor:   ColorHours700To1000,
				Rank:          1,
				RankVisible:   false,
				NumStars:      0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetSeatAppearance(tt.totalStudySec, tt.rankVisible, tt.rp, tt.favoriteColor)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSeatAppearanceRejectsNegativeStudyTime(t *testing.T) {
	_, err := GetSeatAppearance(-3600, false, 0, "")
	assert.Error(t, err)
}

func TestGetSeatAppearanceTopBarDoesNotDependOnRankVisibility(t *testing.T) {
	const totalStudySec = 3600 * 1001
	const favoriteColor = "#123456"

	rankHidden, err := GetSeatAppearance(totalStudySec, false, 45000, favoriteColor)
	assert.NoError(t, err)
	rankVisible, err := GetSeatAppearance(totalStudySec, true, 45000, favoriteColor)
	assert.NoError(t, err)

	assert.Equal(t, favoriteColor, rankHidden.TopBarColor)
	assert.Equal(t, rankHidden.TopBarColor, rankVisible.TopBarColor)
	assert.Equal(t, rankHidden.Rank, rankVisible.Rank)
	assert.Equal(t, rankHidden.NumStars, rankVisible.NumStars)
	assert.False(t, rankHidden.RankVisible)
	assert.True(t, rankVisible.RankVisible)
}

func TestRankByRP(t *testing.T) {
	tests := []struct {
		rp   int
		rank int
	}{
		{rp: 0, rank: 1},
		{rp: 9999, rank: 1},
		{rp: 10000, rank: 2},
		{rp: 19999, rank: 2},
		{rp: 20000, rank: 3},
		{rp: 29999, rank: 3},
		{rp: 30000, rank: 4},
		{rp: 39999, rank: 4},
		{rp: 40000, rank: 5},
		{rp: 49999, rank: 5},
		{rp: 50000, rank: 6},
		{rp: 59999, rank: 6},
		{rp: 60000, rank: 7},
		{rp: 69999, rank: 7},
		{rp: 70000, rank: 8},
		{rp: 79999, rank: 8},
		{rp: 80000, rank: 9},
		{rp: 89999, rank: 9},
		{rp: 90000, rank: 10},
		{rp: 99999, rank: 10},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.rp), func(t *testing.T) {
			assert.Equal(t, tt.rank, RankByRP(tt.rp))
		})
	}
}

func TestCanUseFavoriteColor(t *testing.T) {
	tests := []struct {
		hours    int
		expected bool
	}{
		{hours: 0, expected: false},
		{hours: 999, expected: false},
		{hours: 1000, expected: true},
		{hours: 1001, expected: true},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.hours), func(t *testing.T) {
			assert.Equal(t, tt.expected, CanUseFavoriteColor(3600*tt.hours))
		})
	}
}

func TestTotalStudySecToNumStars(t *testing.T) {
	tests := []struct {
		hours int
		stars int
	}{
		{hours: 0, stars: 0},
		{hours: 999, stars: 0},
		{hours: 1000, stars: 1},
		{hours: 1999, stars: 1},
		{hours: 2000, stars: 2},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.hours), func(t *testing.T) {
			assert.Equal(t, tt.stars, TotalStudySecToNumStars(3600*tt.hours))
		})
	}
}

func TestTotalStudyHoursToColorCodeBoundaries(t *testing.T) {
	tests := []struct {
		hours    int
		expected string
	}{
		{0, ColorHours0To5},
		{4, ColorHours0To5},
		{5, ColorHours5To10},
		{10, ColorHours10To20},
		{20, ColorHours20To30},
		{30, ColorHours30To50},
		{50, ColorHours50To70},
		{70, ColorHours70To100},
		{100, ColorHours100To150},
		{150, ColorHours150To200},
		{200, ColorHours200To300},
		{300, ColorHours300To400},
		{400, ColorHours400To500},
		{500, ColorHours500To700},
		{700, ColorHours700To1000},
		{1000, ColorHoursFrom1000},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.hours), func(t *testing.T) {
			actual, err := TotalStudyHoursToColorCode(tt.hours)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}

	_, err := TotalStudyHoursToColorCode(-1)
	assert.Error(t, err)
}

func TestTotalStudySecToColorCode(t *testing.T) {
	actual, err := TotalStudySecToColorCode(3600 * 1001)
	assert.NoError(t, err)
	assert.Equal(t, ColorHoursFrom1000, actual)

	_, err = TotalStudySecToColorCode(-3600)
	assert.Error(t, err)
}

func TestIsIncludedInColorNames(t *testing.T) {
	assert.True(t, IsIncludedInColorNames(ColorName0To5))
	assert.True(t, IsIncludedInColorNames(ColorNameFrom1000))
	assert.False(t, IsIncludedInColorNames(""))
	assert.False(t, IsIncludedInColorNames("RandomColor"))
}

func TestColorNameToColorCode(t *testing.T) {
	assert.Equal(t, ColorHours0To5, ColorNameToColorCode(ColorName0To5))
	assert.Equal(t, ColorHours5To10, ColorNameToColorCode(ColorName5To10))
	assert.Equal(t, ColorHoursFrom1000, ColorNameToColorCode(ColorNameFrom1000))
	assert.Equal(t, "", ColorNameToColorCode("InvalidColor"))
}

func TestColorCodeToColorName(t *testing.T) {
	assert.Equal(t, ColorName0To5, ColorCodeToColorName(ColorHours0To5))
	assert.Equal(t, ColorNameFrom1000, ColorCodeToColorName(ColorHoursFrom1000))
	assert.Equal(t, "不明", ColorCodeToColorName("#123456"))
}
