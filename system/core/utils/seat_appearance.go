package utils

import (
	"app.modules/core/myfirestore"
	"github.com/pkg/errors"
	"reflect"
	"strconv"
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
	ColorName5To10     = "うすももいろ"
	ColorName10To20    = "ライトサーモン"
	ColorName20To30    = "オレンジ"
	ColorName30To50    = "黄色"
	ColorName50To70    = "黄緑"
	ColorName70To100   = "ペールグリーン"
	ColorName100To150  = "ミントグリーン"
	ColorName150To200  = "アクアマリン"
	ColorName200To300  = "水色"
	ColorName300To400  = "スカイブルー"
	ColorName400To500  = "ロイヤルブルー"
	ColorName500To700  = "青紫"
	ColorName700To1000 = "紫"
	ColorNameFrom1000  = "ピンク"

	ColorRank1         = "#D8D8D8"
	ColorRank2         = "#93FF66"
	ColorRank3         = "#FFFF66"
	ColorRank4         = "#FFC666"
	ColorRank5         = "#FF6666"
	ColorRank6         = "#00FFFF"
	ColorRank7         = "#95ABED"
	ColorRank8         = "#BDB7E5"
	ColorRank9         = "#BF80DF"
	ColorRank10        = "#FF66FF"
	ColorRank10andMore = "#FF1F1F"
)

func GetSeatAppearance(totalStudySec int, rankVisible bool, rp int, favoriteColor string) (myfirestore.SeatAppearance, error) {
	var colorCode1 string
	var colorCode2 string
	if rankVisible {
		colorCode1, colorCode2 = RankPointToColorCodePair(rp)
	} else {
		if CanUseFavoriteColor(totalStudySec) && !reflect.ValueOf(favoriteColor).IsZero() {
			colorCode1 = favoriteColor
		} else {
			var err error
			colorCode1, err = TotalStudySecToColorCode(totalStudySec)
			if err != nil {
				return myfirestore.SeatAppearance{}, err
			}
		}
	}

	return myfirestore.SeatAppearance{
		ColorCode1:           colorCode1,
		ColorCode2:           colorCode2,
		NumStars:             TotalStudySecToNumStars(totalStudySec),
		ColorGradientEnabled: rankVisible,
	}, nil
}

func CanUseFavoriteColor(totalStudySec int) bool {
	hours := SecondsToHours(totalStudySec)
	return hours >= FavoriteColorAvailableThresholdHours
}

func TotalStudySecToNumStars(totalStudySec int) int {
	hours := SecondsToHours(totalStudySec)
	return hours / 1e3
}

func TotalStudySecToColorCode(totalStudySec int) (string, error) {
	totalHours := SecondsToHours(totalStudySec)
	return TotalStudyHoursToColorCode(totalHours)
}

func TotalStudyHoursToColorCode(totalHours int) (string, error) {
	if totalHours < 0 {
		return "", errors.New("invalid total study hours: " + strconv.Itoa(totalHours))
	} else if totalHours < 5 {
		return ColorHours0To5, nil
	} else if totalHours < 10 {
		return ColorHours5To10, nil
	} else if totalHours < 20 {
		return ColorHours10To20, nil
	} else if totalHours < 30 {
		return ColorHours20To30, nil
	} else if totalHours < 50 {
		return ColorHours30To50, nil
	} else if totalHours < 70 {
		return ColorHours50To70, nil
	} else if totalHours < 100 {
		return ColorHours70To100, nil
	} else if totalHours < 150 {
		return ColorHours100To150, nil
	} else if totalHours < 200 {
		return ColorHours150To200, nil
	} else if totalHours < 300 {
		return ColorHours200To300, nil
	} else if totalHours < 400 {
		return ColorHours300To400, nil
	} else if totalHours < 500 {
		return ColorHours400To500, nil
	} else if totalHours < 700 {
		return ColorHours500To700, nil
	} else if totalHours < 1000 {
		return ColorHours700To1000, nil
	} else {
		return ColorHoursFrom1000, nil
	}
}

func IsIncludedInColorNames(value string) bool {
	return value == ColorName0To5 || value == ColorName5To10 || value == ColorName10To20 || value == ColorName20To30 ||
		value == ColorName30To50 || value == ColorName50To70 || value == ColorName70To100 || value == ColorName100To150 ||
		value == ColorName150To200 || value == ColorName200To300 || value == ColorName300To400 || value == ColorName400To500 ||
		value == ColorName500To700 || value == ColorName700To1000 || value == ColorNameFrom1000
}

func ColorNameToColorCode(colorName string) string {
	switch colorName {
	case ColorName0To5:
		return ColorHours0To5
	case ColorName5To10:
		return ColorHours5To10
	case ColorName10To20:
		return ColorHours10To20
	case ColorName20To30:
		return ColorHours20To30
	case ColorName30To50:
		return ColorHours30To50
	case ColorName50To70:
		return ColorHours50To70
	case ColorName70To100:
		return ColorHours70To100
	case ColorName100To150:
		return ColorHours100To150
	case ColorName150To200:
		return ColorHours150To200
	case ColorName200To300:
		return ColorHours200To300
	case ColorName300To400:
		return ColorHours300To400
	case ColorName400To500:
		return ColorHours400To500
	case ColorName500To700:
		return ColorHours500To700
	case ColorName700To1000:
		return ColorHours700To1000
	case ColorNameFrom1000:
		return ColorHoursFrom1000
	default:
		return ""
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

func RankPointToColorCodePair(rp int) (string, string) {
	if rp < 1e4 {
		return ColorRank1, ColorRank2
	} else if rp < 2e4 {
		return ColorRank2, ColorRank3
	} else if rp < 3e4 {
		return ColorRank3, ColorRank4
	} else if rp < 4e4 {
		return ColorRank4, ColorRank5
	} else if rp < 5e4 {
		return ColorRank5, ColorRank6
	} else if rp < 6e4 {
		return ColorRank6, ColorRank7
	} else if rp < 7e4 {
		return ColorRank7, ColorRank8
	} else if rp < 8e4 {
		return ColorRank8, ColorRank9
	} else if rp < 9e4 {
		return ColorRank9, ColorRank10
	} else {
		return ColorRank10, ColorRank10andMore
	}
}
