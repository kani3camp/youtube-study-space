package utils

import "time"

func CalcRankPoint(netStudyDuration time.Duration, isWorkNameSet bool, continuousEntryDays int,
	previousRankPoint int) int {
	basePoint := int(netStudyDuration.Minutes())
	var workNameSetMagnification float64         // 作業内容設定倍率
	var continuousEntryDaysMagnification float64 // 連続入室日数倍率
	var rankMagnification float64                // ランクによる倍率
	
	if isWorkNameSet {
		workNameSetMagnification = 1.1
	} else {
		workNameSetMagnification = 1
	}
	
	continuousEntryDaysMagnification = 1 + 0.01*float64(continuousEntryDays)
	if continuousEntryDaysMagnification > 2 {
		continuousEntryDaysMagnification = 2
	}
	
	rankMagnification = MagnificationByRankPoint(previousRankPoint)
	
	return int(float64(basePoint) * workNameSetMagnification * continuousEntryDaysMagnification * rankMagnification)
}

func MagnificationByRankPoint(rp int) float64 {
	if rp < 1e4 {
		return 1
	} else if rp < 2e4 {
		return 1
	} else if rp < 3e4 {
		return 0.95
	} else if rp < 4e4 {
		return 0.9
	} else if rp < 5e4 {
		return 0.8
	} else if rp < 6e4 {
		return 0.7
	} else if rp < 7e4 {
		return 0.6
	} else if rp < 8e4 {
		return 0.5
	} else if rp < 9e4 {
		return 0.4
	} else {
		return 0.3
	}
}
