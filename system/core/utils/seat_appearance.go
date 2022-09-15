package utils

import (
	"app.modules/core/myfirestore"
	"reflect"
)

const (
	FavoriteColorAvailableThresholdHours = 1000
	
	ColorHours0To5      = "#FFF"
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
	
	ColorName0To5      = "白"
	ColorName5To10     = "うすい赤"
	ColorName10To20    = "ライトサーモン"
	ColorName20To30    = "トパーズ"
	ColorName30To50    = "黄"
	ColorName50To70    = "黄緑"
	ColorName70To100   = "ミントグリーン"
	ColorName100To150  = "ライトグリーン"
	ColorName150To200  = "アクアマリン"
	ColorName200To300  = "エレクトリックブルー"
	ColorName300To400  = "ライトスカイブルー"
	ColorName400To500  = "フレンチスカイブルー"
	ColorName500To700  = "ブルーバイオレット"
	ColorName700To1000 = "ペールバイオレット"
	ColorNameFrom1000  = "ピンク"
	
	ColorRank1  = "#D8D8D8"
	ColorRank2  = "#93FF66"
	ColorRank3  = "#FFFF66"
	ColorRank4  = "#FFC666"
	ColorRank5  = "#FF6666"
	ColorRank6  = "#00FFFF"
	ColorRank7  = "#95ABED"
	ColorRank8  = "#bdb7e5"
	ColorRank9  = "#BF80DF"
	ColorRank10 = "#FF66FF"
)

func GetSeatAppearance(totalStudySec int, rankVisible bool, rp int, favoriteColor string) myfirestore.SeatAppearance {
	var colorCode string
	if rankVisible {
		colorCode = RankPointToColorCode(rp)
	} else {
		if CanUseFavoriteColor(totalStudySec) && !reflect.ValueOf(favoriteColor).IsZero() {
			colorCode = favoriteColor
		} else {
			colorCode = TotalStudySecToColorCode(totalStudySec)
		}
	}
	
	return myfirestore.SeatAppearance{
		ColorCode:     colorCode,
		NumStars:      TotalStudySecToNumStars(totalStudySec),
		GlowAnimation: rankVisible,
	}
}

func CanUseFavoriteColor(totalStudySec int) bool {
	hours := SecondsToHours(totalStudySec)
	return hours >= FavoriteColorAvailableThresholdHours
}

func TotalStudySecToNumStars(totalStudySec int) int {
	hours := SecondsToHours(totalStudySec)
	return hours / 1e3
}

func TotalStudySecToColorCode(totalStudySec int) string {
	totalHours := SecondsToHours(totalStudySec)
	return TotalStudyHoursToColorCode(totalHours)
}

func TotalStudyHoursToColorCode(totalHours int) string {
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

// ColorCodeToColorName 累計時間の分け方で使用されている色のみ対応。
func ColorCodeToColorName(colorCode string) string {
	switch colorCode {
	case ColorHours0To5:
		return ColorName0To5
	case ColorHours5To10:
		return ColorName5To10
	case ColorHours10To20:
		return ColorName10To20
	case ColorHours20To30:
		return ColorName20To30
	case ColorHours30To50:
		return ColorName30To50
	case ColorHours50To70:
		return ColorName50To70
	case ColorHours70To100:
		return ColorName70To100
	case ColorHours100To150:
		return ColorName100To150
	case ColorHours150To200:
		return ColorName150To200
	case ColorHours200To300:
		return ColorName200To300
	case ColorHours300To400:
		return ColorName300To400
	case ColorHours400To500:
		return ColorName400To500
	case ColorHours500To700:
		return ColorName500To700
	case ColorHours700To1000:
		return ColorName700To1000
	case ColorHoursFrom1000:
		return ColorNameFrom1000
	default:
		return "不明"
	}
}

func RankPointToColorCode(rp int) string {
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
