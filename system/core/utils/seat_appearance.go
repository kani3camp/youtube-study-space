package utils

import (
	"app.modules/core/myfirestore"
)

const (
	ColorHours0To5      = "#fff"
	ColorHours5To10     = "#FFD4CC"
	ColorHours10To20    = "#FF9580"
	ColorHours20To30    = "#FFC880"
	ColorHours30To50    = "#FFFB7F"
	ColorHours50To70    = "#D0FF80"
	ColorHours70To100   = "#9DFF7F"
	ColorHours100To150  = "#80FF95"
	ColorHours150To200  = "#80FFC8"
	ColorHours200To300  = "#80FFFB"
	ColorHours300To400  = "#80D0FF"
	ColorHours400To500  = "#809EFF"
	ColorHours500To700  = "#947FFF"
	ColorHours700To1000 = "#C880FF"
	ColorHoursFrom1000  = "#FF7FFF"
	
	ColorRank1  = "#C0C0C0"
	ColorRank2  = "#FFFAFA"
	ColorRank3  = "#FFFF00"
	ColorRank4  = "#FFA500"
	ColorRank5  = "#FF0000"
	ColorRank6  = "#00FFFF"
	ColorRank7  = "#4169E1"
	ColorRank8  = "#8470FF"
	ColorRank9  = "#9932CC"
	ColorRank10 = "#FF00FF"
)

func GetSeatAppearance(totalStudySec int, rankVisible bool, rp int) myfirestore.SeatAppearance {
	var colorCode string
	if rankVisible {
		colorCode = rankPointToColorCode(rp)
	} else {
		colorCode = totalStudySecToColorCode(totalStudySec)
	}
	
	return myfirestore.SeatAppearance{
		ColorCode:     colorCode,
		NumStars:      totalStudySecToNumStars(totalStudySec),
		GlowAnimation: rankVisible,
	}
}

func totalStudySecToNumStars(totalStudySec int) int {
	hours := SecondsToHours(totalStudySec)
	return hours / 1e3
}

func totalStudySecToColorCode(totalStudySec int) string {
	// 時間に換算
	totalHours := SecondsToHours(totalStudySec)
	
	if totalHours < 5 {
		return ColorHours0To5
	} else if totalHours < 10 {
		return ColorHours5To10
	} else if totalHours < 20 {
		return ColorHours10To20
	} else if totalHours < 30 {
		return ColorHours20To30
	} else if totalHours < 50 {
		return ColorHours30To50
	} else if totalHours < 70 {
		return ColorHours50To70
	} else if totalHours < 100 {
		return ColorHours70To100
	} else if totalHours < 150 {
		return ColorHours100To150
	} else if totalHours < 200 {
		return ColorHours150To200
	} else if totalHours < 300 {
		return ColorHours200To300
	} else if totalHours < 400 {
		return ColorHours300To400
	} else if totalHours < 500 {
		return ColorHours400To500
	} else if totalHours < 700 {
		return ColorHours500To700
	} else if totalHours < 1000 {
		return ColorHours700To1000
	} else {
		return ColorHoursFrom1000
	}
}

func rankPointToColorCode(rp int) string {
	if rp < 1e4 {
		return ColorRank1
	} else if rp < 2e4 {
		return ColorRank2
	} else if rp < 3e4 {
		return ColorRank3
	} else if rp < 4e4 {
		return ColorRank4
	} else if rp < 5e4 {
		return ColorRank5
	} else if rp < 6e4 {
		return ColorRank6
	} else if rp < 7e4 {
		return ColorRank7
	} else if rp < 8e4 {
		return ColorRank8
	} else if rp < 9e4 {
		return ColorRank9
	} else {
		return ColorRank10
	}
}
