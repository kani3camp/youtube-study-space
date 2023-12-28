package utils

import (
	"fmt"
	"github.com/pkg/errors"
	"time"
)

const (
	RankPointLowerLimit = 0
	RankPointUpperLimit = 99_999
)

// CalcNewRPExitRoom calculates the newly added rank points when a user leaves a room.
func CalcNewRPExitRoom(
	netStudyDuration time.Duration,
	isWorkNameSet bool,
	yesterdayContinuedActive bool,
	currentStateStarted time.Time,
	lastActiveAt time.Time,
	previousRankPoint int,
) (int, error) {
	basePoint := int(netStudyDuration.Minutes())

	var workNameSetMagnification float64          // 作業内容設定倍率
	var continuousActiveDaysMagnification float64 // 連続入室日数倍率
	var rankMagnification float64                 // ランクによる倍率

	if isWorkNameSet {
		workNameSetMagnification = 1.1
	} else {
		workNameSetMagnification = 1
	}

	continuousActiveDays, err := CalcContinuousActiveDays(yesterdayContinuedActive, currentStateStarted, lastActiveAt)
	if err != nil {
		return 0, fmt.Errorf("in CalcContinuousActiveDays: %w", err)
	}
	continuousActiveDaysMagnification = 1 + 0.01*float64(continuousActiveDays)
	if continuousActiveDaysMagnification > 2 {
		continuousActiveDaysMagnification = 2
	}

	rankMagnification = MagnificationByRP(previousRankPoint)

	addedRP := int(float64(basePoint) * workNameSetMagnification * continuousActiveDaysMagnification * rankMagnification)

	return ApplyRPRange(previousRankPoint + addedRP), nil
}

// DailyUpdateRankPoint checks the number of consecutive days of use by users and adjusts rank points daily.
func DailyUpdateRankPoint(
	lastPenaltyImposedDays int,
	isContinuousActive bool,
	currentActivityStateStarted time.Time,
	rankPoint int,
	lastEntered, lastExited, jstNow time.Time,
) (int, bool, time.Time, int, error) {
	// アクティブ・非アクティブ状態の更新
	// 前日0時〜24時に入室していたらactive、そうでなければinactive
	yesterdayDate := jstNow.AddDate(0, 0, -1)
	lastActiveAt := LastActiveAt(lastEntered, lastExited, jstNow)
	// 本日か昨日と同じ日付に入室していたらactive、そうでなければinactive
	if DateEqualJST(lastActiveAt, yesterdayDate) {
		isContinuousActive = true
		lastPenaltyImposedDays = 0
	} else if DateEqualJST(lastActiveAt, jstNow) {
		isContinuousActive = true
		lastPenaltyImposedDays = 0
	} else {
		if isContinuousActive {
			currentActivityStateStarted = lastActiveAt
		}
		isContinuousActive = false
	}

	// 最終active日時が一定日数以上前のユーザーはRPペナルティ処理
	if !isContinuousActive {
		var err error
		rankPoint, lastPenaltyImposedDays, err = CalcNewRPContinuousInactivity(rankPoint, lastActiveAt, lastPenaltyImposedDays)
		if err != nil {
			return 0, false, time.Time{}, 0, fmt.Errorf("in CalcNewRPContinuousInactivity: %w", err)
		}
	}

	return lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint, nil
}

// CalcNewRPContinuousInactivity 連続で利用しない日が続くとRP減らす。
func CalcNewRPContinuousInactivity(previousRP int, lastActiveAt time.Time, lastPenaltyImposedDays int) (int, int, error) {
	inactiveDays, err := CalcContinuousInactiveDays(lastActiveAt)
	if err != nil {
		return 0, 0, fmt.Errorf("in CalcContinuousInactiveDays: %w", err)
	}
	if lastPenaltyImposedDays > inactiveDays {
		return 0, 0, errors.New("lastPenaltyImposedDays > inactiveDays")
	} else if lastPenaltyImposedDays == inactiveDays {
		// 今日すでにペナルティ処理が完了しているためRPをそのまま返す
		return previousRP, inactiveDays, nil
	}
	magnification, newPenaltyImposedDays := PenaltyMagnificationByInactiveDays(inactiveDays)
	return ApplyRPRange(int(float64(previousRP) * magnification)), newPenaltyImposedDays, nil
}

// CalcContinuousInactiveDays 連続非アクティブn日のとき、nを返す。
func CalcContinuousInactiveDays(lastActiveAt time.Time) (int, error) {
	jstNow := JstNow()
	if lastActiveAt.After(jstNow) {
		return 0, errors.New("lastActiveAt is after jstNow.")
	}
	inactiveDuration := jstNow.Sub(lastActiveAt)
	inactiveDays := int(inactiveDuration.Hours() / 24)
	return inactiveDays, nil
}

// CalcContinuousActiveDays 連続アクティブn日目のとき、n-1を返す。
func CalcContinuousActiveDays(yesterdayContinuedActive bool, currentStateStarted time.Time, lastActiveAt time.Time) (int, error) {
	jstNow := JstNow()
	if currentStateStarted.After(jstNow) || lastActiveAt.After(jstNow) {
		return 0, errors.New("currentStateStarted.After(jstNow) is true or lastActiveAt.After(jstNow) is true.")
	}
	if yesterdayContinuedActive {
		startDate0AM := time.Date(currentStateStarted.Year(), currentStateStarted.Month(), currentStateStarted.Day(),
			0, 0, 0, 0, JapanLocation())
		if DateEqualJST(lastActiveAt, jstNow) {
			return int(jstNow.Sub(startDate0AM).Hours() / 24), nil
		} else { // 今日はまだ入室してないが、今日非アクティブとは断定できない。昨日までの連続日数を返す。
			yesterday := time.Date(jstNow.Year(), jstNow.Month(), jstNow.Day(), 0, 0, 0, 0, JapanLocation())
			return int(yesterday.Sub(startDate0AM).Hours() / 24), nil
		}
	} else { // 昨日非アクティブだった時点で現在の連続アクティブ日数は0。
		return 0, nil
	}
}

func ApplyRPRange(rp int) int {
	if rp < RankPointLowerLimit {
		return RankPointLowerLimit
	} else if rp > RankPointUpperLimit {
		return RankPointUpperLimit
	}
	return rp
}

// MagnificationByRP RPから倍率を求める。
func MagnificationByRP(rp int) float64 {
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

// PenaltyMagnificationByInactiveDays 連続非アクティブ日数によるペナルティRP調整倍率
func PenaltyMagnificationByInactiveDays(inactiveDays int) (float64, int) {
	if inactiveDays >= 30 {
		return 0, 30
	} else if inactiveDays >= 7 {
		return 0.5, 7
	} else if inactiveDays >= 3 {
		return 0.8, 3
	} else {
		return 1, 0
	}
}

// LastActiveAt 最近activeだった日時。現在を含む。
func LastActiveAt(lastEntered, lastExited, now time.Time) time.Time {
	if lastEntered.Before(lastExited) {
		return lastExited
	} else if lastEntered.Equal(lastExited) {
		return lastExited
	} else {
		return now
	}
}
