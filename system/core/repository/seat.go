package repository

import (
	"time"

	"app.modules/core/timeutil"
)

func (s *SeatDoc) RealtimeEntryDurationMin(now time.Time) time.Duration {
	return now.Sub(s.EnteredAt)
}

func (s *SeatDoc) RemainingWorkDuration(now time.Time) time.Duration {
	return s.Until.Sub(now)
}

// StartBreak は作業状態から休憩状態に遷移する。
// 現在の作業時間を累積し、日付跨ぎを考慮して当日の累積時間を計算する。
//
// 引数:
//   - now: 休憩開始時刻（JSTを想定）
//   - breakWorkName: 休憩中の作業名
//   - breakDurationMin: 休憩時間（分）
//
// 前提条件: s.State == WorkState
func (s *SeatDoc) StartBreak(now time.Time, breakWorkName string, breakDurationMin int) {
	breakUntil := now.Add(time.Duration(breakDurationMin) * time.Minute)
	workedSec := int(timeutil.NoNegativeDuration(now.Sub(s.CurrentStateStartedAt)).Seconds())
	cumulativeWorkSec := s.CumulativeWorkSec + workedSec

	// 日付跨ぎを考慮して当日の累積時間を計算
	var dailyCumulativeWorkSec int
	if workedSec > timeutil.SecondsOfDay(now) {
		dailyCumulativeWorkSec = timeutil.SecondsOfDay(now)
	} else {
		dailyCumulativeWorkSec = s.DailyCumulativeWorkSec + workedSec
	}

	s.State = BreakState
	s.CurrentStateStartedAt = now
	s.CurrentStateUntil = breakUntil
	s.CumulativeWorkSec = cumulativeWorkSec
	s.DailyCumulativeWorkSec = dailyCumulativeWorkSec
	s.BreakWorkName = breakWorkName
}

// ResumeWork は休憩状態から作業状態に復帰する。
// 日付跨ぎを考慮して当日の累積時間をリセットまたは維持する。
//
// 引数:
//   - now: 作業再開時刻（JSTを想定）
//   - workName: 作業名（空文字列の場合は既存のWorkNameを保持）
//
// 前提条件: s.State == BreakState
func (s *SeatDoc) ResumeWork(now time.Time, workName string) {
	breakSec := int(timeutil.NoNegativeDuration(now.Sub(s.CurrentStateStartedAt)).Seconds())

	// 日付跨ぎを考慮して当日の累積時間を調整
	dailyCumulativeWorkSec := s.DailyCumulativeWorkSec
	if breakSec > timeutil.SecondsOfDay(now) {
		dailyCumulativeWorkSec = 0
	}

	s.State = WorkState
	s.CurrentStateStartedAt = now
	s.CurrentStateUntil = s.Until
	s.DailyCumulativeWorkSec = dailyCumulativeWorkSec

	// 作業名が指定されていれば更新
	if workName != "" {
		s.WorkName = workName
	}
}

// SetWorkDuration は作業時間（入室から退室まで）を変更する。
// Until と CurrentStateUntil を同時に更新する。
//
// 引数:
//   - newUntil: 新しい自動退室予定時刻
//
// 前提条件: s.State == WorkState
func (s *SeatDoc) SetWorkDuration(newUntil time.Time) {
	s.Until = newUntil
	s.CurrentStateUntil = newUntil
}

// ExtendWorkDuration は作業時間を延長する。
// 最大作業時間を超えないように調整し、実際の延長時間を返す。
//
// 引数:
//   - now: 現在時刻（JSTを想定）
//   - requestedAddMin: 延長希望時間（分）
//   - maxWorkTimeMin: 最大作業時間（分、現在時刻からの上限）
//
// 戻り値:
//   - actualAddedMin: 実際に延長された時間（分）
//   - newRemainingMin: 延長後の残り時間（分）
//
// 前提条件: s.State == WorkState
func (s *SeatDoc) ExtendWorkDuration(now time.Time, requestedAddMin int, maxWorkTimeMin int) (actualAddedMin int, newRemainingMin int) {
	newUntil := s.Until.Add(time.Duration(requestedAddMin) * time.Minute)
	remainingMin := int(timeutil.NoNegativeDuration(newUntil.Sub(now)).Minutes())

	// 最大作業時間を超えないように調整
	if remainingMin > maxWorkTimeMin {
		newUntil = now.Add(time.Duration(maxWorkTimeMin) * time.Minute)
	}

	actualAddedMin = int(timeutil.NoNegativeDuration(newUntil.Sub(s.Until)).Minutes())
	s.Until = newUntil
	s.CurrentStateUntil = newUntil
	newRemainingMin = int(timeutil.NoNegativeDuration(newUntil.Sub(now)).Minutes())

	return actualAddedMin, newRemainingMin
}

// ExtendBreakDuration は休憩時間を延長する。
// 最大休憩時間を超えないように調整し、必要に応じてUntilも延長する。
//
// 引数:
//   - now: 現在時刻（JSTを想定）
//   - requestedAddMin: 延長希望時間（分）
//   - maxBreakDurationMin: 最大休憩時間（分、休憩開始からの上限）
//
// 戻り値:
//   - actualAddedMin: 実際に延長された時間（分）
//   - newRemainingBreakMin: 延長後の休憩残り時間（分）
//   - newRemainingUntilExitMin: 延長後の自動退室までの残り時間（分）
//
// 前提条件: s.State == BreakState
func (s *SeatDoc) ExtendBreakDuration(now time.Time, requestedAddMin int, maxBreakDurationMin int) (actualAddedMin int, newRemainingBreakMin int, newRemainingUntilExitMin int) {
	newBreakUntil := s.CurrentStateUntil.Add(time.Duration(requestedAddMin) * time.Minute)
	newBreakDuration := timeutil.NoNegativeDuration(newBreakUntil.Sub(s.CurrentStateStartedAt))

	// 最大休憩時間を超えないように調整
	if int(newBreakDuration.Minutes()) > maxBreakDurationMin {
		newBreakUntil = s.CurrentStateStartedAt.Add(time.Duration(maxBreakDurationMin) * time.Minute)
	}

	actualAddedMin = int(timeutil.NoNegativeDuration(newBreakUntil.Sub(s.CurrentStateUntil)).Minutes())
	s.CurrentStateUntil = newBreakUntil

	// 休憩終了時刻がUntilを超える場合はUntilも延長
	if newBreakUntil.After(s.Until) {
		s.Until = newBreakUntil
	}

	newRemainingBreakMin = int(timeutil.NoNegativeDuration(s.CurrentStateUntil.Sub(now)).Minutes())
	newRemainingUntilExitMin = int(timeutil.NoNegativeDuration(s.Until.Sub(now)).Minutes())

	return actualAddedMin, newRemainingBreakMin, newRemainingUntilExitMin
}
