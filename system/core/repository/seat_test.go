package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const testTimeLayout = "2006-01-02 15:04:05"

// テスト用のヘルパー関数
func mustParseTime(layout, value string) time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}
	return t
}

func mustSeat(mutate func(doc *SeatDoc)) SeatDoc {
	seat := SeatDoc{
		State:                   WorkState,
		WorkName:                "作業",
		CurrentStateStartedAt:   mustParseTime(testTimeLayout, "2026-02-01 10:00:00"),
		CurrentSegmentStartedAt: mustParseTime(testTimeLayout, "2026-02-01 10:00:00"),
		CurrentStateUntil:       mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
		Until:                   mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
	}
	if mutate != nil {
		mutate(&seat)
	}
	return seat
}

func TestSeatDoc_StartBreak(t *testing.T) {
	t.Run("通常の休憩開始", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.CumulativeWorkSec = 3600 // 既に1時間分累積
			s.DailyCumulativeWorkSec = 3600
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 11:00:00") // 1時間作業
		seat.StartBreak(now, "休憩中", 15)

		assert.Equal(t, BreakState, seat.State)
		assert.Equal(t, now, seat.CurrentStateStartedAt)
		assert.Equal(t, now, seat.CurrentSegmentStartedAt)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 11:15:00"), seat.CurrentStateUntil)
		assert.Equal(t, 3600+3600, seat.CumulativeWorkSec) // 1時間+1時間
		assert.Equal(t, 3600+3600, seat.DailyCumulativeWorkSec)
		assert.Equal(t, "休憩中", seat.BreakWorkName)
	})

	t.Run("日付跨ぎなし_当日中の作業後に休憩", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.CurrentStateStartedAt = mustParseTime(testTimeLayout, "2026-02-01 09:00:00")
			s.CurrentSegmentStartedAt = mustParseTime(testTimeLayout, "2026-02-01 09:00:00")
			s.CumulativeWorkSec = 0
			s.DailyCumulativeWorkSec = 0
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 12:00:00") // 3時間作業
		seat.StartBreak(now, "ランチ", 60)

		assert.Equal(t, 3*3600, seat.CumulativeWorkSec)
		assert.Equal(t, 3*3600, seat.DailyCumulativeWorkSec)
	})

	t.Run("日付跨ぎあり_作業時間が当日の秒数を超える", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.CurrentStateStartedAt = mustParseTime(testTimeLayout, "2026-02-01 00:00:00")
			s.CumulativeWorkSec = 0
			s.DailyCumulativeWorkSec = 0
		})

		// 翌日の午前1時に休憩開始（25時間作業）
		now := mustParseTime(testTimeLayout, "2026-02-02 01:00:00")
		seat.StartBreak(now, "深夜休憩", 10)

		assert.Equal(t, 25*3600, seat.CumulativeWorkSec)     // CumulativeWorkSecは実際の作業時間
		assert.Equal(t, 1*3600, seat.DailyCumulativeWorkSec) // DailyCumulativeWorkSecは当日の秒数（1時間分 = 3600秒）
	})

	t.Run("0分作業後の休憩", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.CurrentStateStartedAt = mustParseTime(testTimeLayout, "2026-02-01 10:00:00")
			s.CumulativeWorkSec = 1800
			s.DailyCumulativeWorkSec = 1800
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 10:00:00") // 同じ時刻
		seat.StartBreak(now, "即休憩", 5)

		assert.Equal(t, 1800, seat.CumulativeWorkSec) // 変化なし
		assert.Equal(t, 1800, seat.DailyCumulativeWorkSec)
	})

	t.Run("BreakWorkNameの空文字列", func(t *testing.T) {
		seat := SeatDoc{
			State:                  WorkState,
			CurrentStateStartedAt:  mustParseTime(testTimeLayout, "2026-02-01 10:00:00"),
			CumulativeWorkSec:      0,
			DailyCumulativeWorkSec: 0,
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 11:00:00")
		seat.StartBreak(now, "", 15)

		assert.Equal(t, "", seat.BreakWorkName)
	})
}

func TestSeatDoc_ResumeWork(t *testing.T) {
	t.Run("通常の作業再開_作業名変更", func(t *testing.T) {
		seat := SeatDoc{
			State:                  BreakState,
			CurrentStateStartedAt:  mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			Until:                  mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
			WorkName:               "既存の作業",
			CumulativeWorkSec:      7200, // 2時間分
			DailyCumulativeWorkSec: 7200, // 2時間分
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 13:00:00") // 1時間休憩
		seat.ResumeWork(now, "新しい作業")

		assert.Equal(t, WorkState, seat.State)
		assert.Equal(t, now, seat.CurrentStateStartedAt)
		assert.Equal(t, now, seat.CurrentSegmentStartedAt)
		assert.Equal(t, seat.Until, seat.CurrentStateUntil)
		assert.Equal(t, "新しい作業", seat.WorkName)
		assert.Equal(t, 7200, seat.CumulativeWorkSec)      // 変化なし
		assert.Equal(t, 7200, seat.DailyCumulativeWorkSec) // 変化なし
	})

	t.Run("WorkName引継ぎ_呼び出し側で既存の作業名を渡す", func(t *testing.T) {
		seat := SeatDoc{
			State:                  BreakState,
			CurrentStateStartedAt:  mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			Until:                  mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
			WorkName:               "既存の作業名",
			DailyCumulativeWorkSec: 3600,
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:30:00")
		// 呼び出し側で既存の作業名を渡す
		seat.ResumeWork(now, seat.WorkName)

		assert.Equal(t, "既存の作業名", seat.WorkName)
	})

	t.Run("WorkNameクリア_空文字列を明示的に設定", func(t *testing.T) {
		seat := SeatDoc{
			State:                  BreakState,
			CurrentStateStartedAt:  mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			Until:                  mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
			WorkName:               "クリアする作業名",
			DailyCumulativeWorkSec: 3600,
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:30:00")
		seat.ResumeWork(now, "") // 空文字列で明示的にクリア

		assert.Equal(t, "", seat.WorkName) // クリアされる
	})

	t.Run("日付跨ぎなし_当日中の休憩後に再開", func(t *testing.T) {
		seat := SeatDoc{
			State:                  BreakState,
			CurrentStateStartedAt:  mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			Until:                  mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
			DailyCumulativeWorkSec: 10800, // 3時間分
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 13:00:00") // 1時間休憩
		seat.ResumeWork(now, "再開")

		assert.Equal(t, 10800, seat.DailyCumulativeWorkSec) // 変化なし
	})

	t.Run("日付跨ぎあり_休憩時間が当日の秒数を超える", func(t *testing.T) {
		seat := SeatDoc{
			State:                   BreakState,
			CurrentStateStartedAt:   mustParseTime(testTimeLayout, "2026-02-01 22:00:00"),
			CurrentSegmentStartedAt: mustParseTime(testTimeLayout, "2026-02-01 22:00:00"),
			Until:                   mustParseTime(testTimeLayout, "2026-02-02 18:00:00"),
			CumulativeWorkSec:       3600, // 1時間分
			DailyCumulativeWorkSec:  3600, // 1時間分
		}

		// 翌日の午前2時に再開（4時間休憩）
		now := mustParseTime(testTimeLayout, "2026-02-02 02:00:00")
		seat.ResumeWork(now, "翌日再開")

		assert.Equal(t, 3600, seat.CumulativeWorkSec)   // リセットされない
		assert.Equal(t, 0, seat.DailyCumulativeWorkSec) // リセットされる
	})

	t.Run("Untilの引継ぎ確認", func(t *testing.T) {
		until := mustParseTime(testTimeLayout, "2026-02-01 20:00:00")
		seat := SeatDoc{
			State:                  BreakState,
			CurrentStateStartedAt:  mustParseTime(testTimeLayout, "2026-02-01 15:00:00"),
			Until:                  until,
			DailyCumulativeWorkSec: 0,
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 15:30:00")
		seat.ResumeWork(now, "作業")

		assert.Equal(t, until, seat.CurrentStateUntil)
	})

	t.Run("0分休憩後の再開", func(t *testing.T) {
		seat := SeatDoc{
			State:                  BreakState,
			CurrentStateStartedAt:  mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			Until:                  mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
			DailyCumulativeWorkSec: 3600,
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:00:00") // 同じ時刻
		seat.ResumeWork(now, "即再開")

		assert.Equal(t, 3600, seat.DailyCumulativeWorkSec) // 変化なし
	})
}

func TestSeatDoc_SetWorkDuration(t *testing.T) {
	t.Run("通常の作業時間変更", func(t *testing.T) {
		seat := mustSeat(nil)

		newUntil := mustParseTime(testTimeLayout, "2026-02-01 20:00:00")
		seat.SetWorkDuration(newUntil)

		assert.Equal(t, newUntil, seat.Until)
		assert.Equal(t, newUntil, seat.CurrentStateUntil)
	})

	t.Run("延長ケース", func(t *testing.T) {
		original := mustParseTime(testTimeLayout, "2026-02-01 17:00:00")
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = original
			s.CurrentStateUntil = original
		})

		extended := mustParseTime(testTimeLayout, "2026-02-01 19:00:00")
		seat.SetWorkDuration(extended)

		assert.Equal(t, extended, seat.Until)
		assert.Equal(t, extended, seat.CurrentStateUntil)
	})

	t.Run("短縮ケース", func(t *testing.T) {
		original := mustParseTime(testTimeLayout, "2026-02-01 19:00:00")
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = original
			s.CurrentStateUntil = original
		})

		shortened := mustParseTime(testTimeLayout, "2026-02-01 17:00:00")
		seat.SetWorkDuration(shortened)

		assert.Equal(t, shortened, seat.Until)
		assert.Equal(t, shortened, seat.CurrentStateUntil)
	})

	t.Run("UntilとCurrentStateUntilの同期確認", func(t *testing.T) {
		seat := mustSeat(nil)

		newUntil := mustParseTime(testTimeLayout, "2026-02-01 21:00:00")
		seat.SetWorkDuration(newUntil)

		assert.Equal(t, seat.Until, seat.CurrentStateUntil)
	})
}

func TestSeatDoc_ExtendWorkDuration(t *testing.T) {
	until1700 := mustParseTime(testTimeLayout, "2026-02-01 17:00:00")

	t.Run("通常の延長", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = until1700
			s.CurrentStateUntil = until1700
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 16:00:00")
		actualAdded, newRemaining := seat.ExtendWorkDuration(now, 60, 180) // 60分延長、最大180分

		assert.Equal(t, 60, actualAdded)
		assert.Equal(t, 120, newRemaining) // 元々60分残り + 60分延長 = 120分
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:00:00"), seat.Until)
		assert.Equal(t, seat.Until, seat.CurrentStateUntil)
	})

	t.Run("最大値超過_maxWorkTimeMinで制限", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = until1700
			s.CurrentStateUntil = until1700
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 16:00:00")
		actualAdded, newRemaining := seat.ExtendWorkDuration(now, 200, 120) // 200分延長希望、最大120分

		// 最大120分までしか延長できない（現在から120分後 = 18:00）
		// 元のUntilは17:00だったので、実際の延長は60分
		assert.Equal(t, 60, actualAdded)
		assert.Equal(t, 120, newRemaining)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:00:00"), seat.Until)
	})

	t.Run("最大値ギリギリ", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = until1700
			s.CurrentStateUntil = until1700
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 16:00:00")
		actualAdded, newRemaining := seat.ExtendWorkDuration(now, 120, 120) // 120分延長、最大120分

		assert.Equal(t, 60, actualAdded) // 17:00まで60分残っているので、追加は60分
		assert.Equal(t, 120, newRemaining)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:00:00"), seat.Until)
	})

	t.Run("延長なし_0分", func(t *testing.T) {
		original := until1700
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = original
			s.CurrentStateUntil = original
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 16:00:00")
		actualAdded, newRemaining := seat.ExtendWorkDuration(now, 0, 180)

		assert.Equal(t, 0, actualAdded)
		assert.Equal(t, 60, newRemaining) // 元々60分残り
		assert.Equal(t, original, seat.Until)
	})

	t.Run("現在時刻がUntilを過ぎている場合", func(t *testing.T) {
		until1600 := mustParseTime(testTimeLayout, "2026-02-01 16:00:00")
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = until1600
			s.CurrentStateUntil = until1600
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 17:00:00") // 既に過ぎている
		actualAdded, newRemaining := seat.ExtendWorkDuration(now, 60, 180)

		// 60分延長されるので17:00になる
		assert.Equal(t, 60, actualAdded) // 16:00 → 17:00なので60分
		assert.Equal(t, 0, newRemaining) // 17:00 → 17:00なので0分
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 17:00:00"), seat.Until)
	})

	t.Run("UntilとCurrentStateUntilの同期確認", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = until1700
			s.CurrentStateUntil = until1700
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 16:00:00")
		seat.ExtendWorkDuration(now, 30, 180)

		assert.Equal(t, seat.Until, seat.CurrentStateUntil)
	})
}

func TestSeatDoc_RemainingWorkMin(t *testing.T) {
	t.Run("正の残り時間", func(t *testing.T) {
		seat := mustSeat(nil)

		now := mustParseTime(testTimeLayout, "2026-02-01 17:00:00")
		remaining := seat.RemainingWorkMin(now)

		assert.Equal(t, 60, remaining)
	})

	t.Run("負の残り時間は0", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = mustParseTime(testTimeLayout, "2026-02-01 17:00:00")
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 18:00:00") // 既に過ぎている
		remaining := seat.RemainingWorkMin(now)

		assert.Equal(t, 0, remaining)
	})

	t.Run("0分残り", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = mustParseTime(testTimeLayout, "2026-02-01 17:00:00")
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 17:00:00")
		remaining := seat.RemainingWorkMin(now)

		assert.Equal(t, 0, remaining)
	})

	t.Run("数秒残りは切り捨て", func(t *testing.T) {
		seat := mustSeat(func(s *SeatDoc) {
			s.Until = mustParseTime(testTimeLayout, "2026-02-01 17:00:30")
		})

		now := mustParseTime(testTimeLayout, "2026-02-01 17:00:00")
		remaining := seat.RemainingWorkMin(now)

		assert.Equal(t, 0, remaining) // 30秒は0分
	})
}

func TestSeatDoc_RemainingBreakMin(t *testing.T) {
	t.Run("正の残り時間", func(t *testing.T) {
		seat := SeatDoc{
			CurrentStateUntil: mustParseTime(testTimeLayout, "2026-02-01 13:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:30:00")
		remaining := seat.RemainingBreakMin(now)

		assert.Equal(t, 30, remaining)
	})

	t.Run("負の残り時間は0", func(t *testing.T) {
		seat := SeatDoc{
			CurrentStateUntil: mustParseTime(testTimeLayout, "2026-02-01 13:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 14:00:00") // 既に過ぎている
		remaining := seat.RemainingBreakMin(now)

		assert.Equal(t, 0, remaining)
	})

	t.Run("0分残り", func(t *testing.T) {
		seat := SeatDoc{
			CurrentStateUntil: mustParseTime(testTimeLayout, "2026-02-01 13:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 13:00:00")
		remaining := seat.RemainingBreakMin(now)

		assert.Equal(t, 0, remaining)
	})
}

func TestSeatDoc_ExtendBreakDuration(t *testing.T) {
	t.Run("通常の延長_Untilは延長なし", func(t *testing.T) {
		seat := SeatDoc{
			State:                 BreakState,
			CurrentStateStartedAt: mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			CurrentStateUntil:     mustParseTime(testTimeLayout, "2026-02-01 13:00:00"),
			Until:                 mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:30:00")
		actualAdded, remainingBreak, remainingExit := seat.ExtendBreakDuration(now, 30, 120)

		assert.Equal(t, 30, actualAdded)
		assert.Equal(t, 60, remainingBreak) // 12:30 → 13:30 = 60分
		assert.Equal(t, 330, remainingExit) // 12:30 → 18:00 = 330分
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 13:30:00"), seat.CurrentStateUntil)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:00:00"), seat.Until) // 変化なし
	})

	t.Run("Untilも延長_休憩終了時刻がUntilを超える", func(t *testing.T) {
		seat := SeatDoc{
			State:                 BreakState,
			CurrentStateStartedAt: mustParseTime(testTimeLayout, "2026-02-01 17:00:00"),
			CurrentStateUntil:     mustParseTime(testTimeLayout, "2026-02-01 17:30:00"),
			Until:                 mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 17:15:00")
		actualAdded, remainingBreak, remainingExit := seat.ExtendBreakDuration(now, 60, 120)

		// 休憩が18:30まで延長される（Untilの18:00を超える）
		assert.Equal(t, 60, actualAdded)
		assert.Equal(t, 75, remainingBreak) // 17:15 → 18:30 = 75分
		assert.Equal(t, 75, remainingExit)  // Untilも18:30に延長される
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:30:00"), seat.CurrentStateUntil)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:30:00"), seat.Until)
	})

	t.Run("最大値超過_maxBreakDurationMinで制限", func(t *testing.T) {
		seat := SeatDoc{
			State:                 BreakState,
			CurrentStateStartedAt: mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			CurrentStateUntil:     mustParseTime(testTimeLayout, "2026-02-01 12:30:00"),
			Until:                 mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:15:00")
		actualAdded, remainingBreak, remainingExit := seat.ExtendBreakDuration(now, 200, 60) // 200分延長希望、最大60分

		// 休憩開始から最大60分（13:00まで）
		assert.Equal(t, 30, actualAdded)    // 12:30 → 13:00 = 30分
		assert.Equal(t, 45, remainingBreak) // 12:15 → 13:00 = 45分
		assert.Equal(t, 345, remainingExit) // 12:15 → 18:00 = 345分
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 13:00:00"), seat.CurrentStateUntil)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:00:00"), seat.Until)
	})

	t.Run("延長なし_0分", func(t *testing.T) {
		originalBreakUntil := mustParseTime(testTimeLayout, "2026-02-01 13:00:00")
		originalUntil := mustParseTime(testTimeLayout, "2026-02-01 18:00:00")
		seat := SeatDoc{
			State:                 BreakState,
			CurrentStateStartedAt: mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			CurrentStateUntil:     originalBreakUntil,
			Until:                 originalUntil,
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:30:00")
		actualAdded, remainingBreak, remainingExit := seat.ExtendBreakDuration(now, 0, 120)

		assert.Equal(t, 0, actualAdded)
		assert.Equal(t, 30, remainingBreak) // 12:30 → 13:00 = 30分
		assert.Equal(t, 330, remainingExit) // 12:30 → 18:00 = 330分
		assert.Equal(t, originalBreakUntil, seat.CurrentStateUntil)
		assert.Equal(t, originalUntil, seat.Until)
	})

	t.Run("最大休憩時間ギリギリ", func(t *testing.T) {
		seat := SeatDoc{
			State:                 BreakState,
			CurrentStateStartedAt: mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			CurrentStateUntil:     mustParseTime(testTimeLayout, "2026-02-01 12:30:00"),
			Until:                 mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:15:00")
		actualAdded, _, _ := seat.ExtendBreakDuration(now, 30, 60) // 開始から60分ちょうど

		assert.Equal(t, 30, actualAdded)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 13:00:00"), seat.CurrentStateUntil)
	})

	t.Run("Untilとの関係_休憩がUntilギリギリまで", func(t *testing.T) {
		seat := SeatDoc{
			State:                 BreakState,
			CurrentStateStartedAt: mustParseTime(testTimeLayout, "2026-02-01 17:00:00"),
			CurrentStateUntil:     mustParseTime(testTimeLayout, "2026-02-01 17:30:00"),
			Until:                 mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 17:15:00")
		_, remainingBreak, remainingExit := seat.ExtendBreakDuration(now, 30, 120)

		// 休憩は18:00まで（Untilと同じ）
		assert.Equal(t, 45, remainingBreak) // 17:15 → 18:00
		assert.Equal(t, 45, remainingExit)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:00:00"), seat.CurrentStateUntil)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:00:00"), seat.Until)
	})

	t.Run("複雑なケース_最大値制限とUntil延長の両方", func(t *testing.T) {
		seat := SeatDoc{
			State:                 BreakState,
			CurrentStateStartedAt: mustParseTime(testTimeLayout, "2026-02-01 17:30:00"),
			CurrentStateUntil:     mustParseTime(testTimeLayout, "2026-02-01 17:45:00"),
			Until:                 mustParseTime(testTimeLayout, "2026-02-01 18:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 17:40:00")
		actualAdded, remainingBreak, remainingExit := seat.ExtendBreakDuration(now, 100, 60) // 100分希望、最大60分

		// 開始から60分 = 18:30
		assert.Equal(t, 45, actualAdded)    // 17:45 → 18:30 = 45分
		assert.Equal(t, 50, remainingBreak) // 17:40 → 18:30 = 50分
		assert.Equal(t, 50, remainingExit)  // Untilも18:30
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:30:00"), seat.CurrentStateUntil)
		assert.Equal(t, mustParseTime(testTimeLayout, "2026-02-01 18:30:00"), seat.Until)
	})
}

func TestSeatDoc_GenerateWorkSegment(t *testing.T) {
	t.Run("通常の作業セグメントを生成できること", func(t *testing.T) {
		seat := SeatDoc{
			UserID:                  "user-1",
			SeatID:                  3,
			SessionID:               "session-1",
			State:                   WorkState,
			WorkName:                "数学",
			CurrentSegmentStartedAt: mustParseTime(testTimeLayout, "2026-02-01 09:15:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 10:45:30")
		workSegment, err := seat.GenerateWorkSegment(now, true)

		assert.NoError(t, err)
		assert.Equal(t, WorkSegmentDoc{
			UserID:       "user-1",
			SeatID:       3,
			IsMemberSeat: true,
			SessionID:    "session-1",
			WorkName:     "数学",
			SegmentType:  WorkState,
			StartedAt:    mustParseTime(testTimeLayout, "2026-02-01 09:15:00"),
			EndedAt:      now,
			DurationSec:  5430,
		}, workSegment)
	})

	t.Run("休憩セグメントではBreakWorkNameを返すこと", func(t *testing.T) {
		seat := SeatDoc{
			UserID:                  "user-2",
			SeatID:                  8,
			SessionID:               "session-2",
			State:                   BreakState,
			WorkName:                "英語",
			BreakWorkName:           "昼休み",
			CurrentSegmentStartedAt: mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:10:00")
		workSegment, err := seat.GenerateWorkSegment(now, false)

		assert.NoError(t, err)
		assert.Equal(t, WorkSegmentDoc{
			UserID:       "user-2",
			SeatID:       8,
			IsMemberSeat: false,
			SessionID:    "session-2",
			WorkName:     "昼休み",
			SegmentType:  BreakState,
			StartedAt:    mustParseTime(testTimeLayout, "2026-02-01 12:00:00"),
			EndedAt:      now,
			DurationSec:  600,
		}, workSegment)
	})

	t.Run("CurrentSegmentStartedAtがゼロ値だった場合にエラーを返すこと", func(t *testing.T) {
		seat := SeatDoc{
			State: WorkState,
		}

		now := mustParseTime(testTimeLayout, "2026-02-01 12:00:00")
		workSegment, err := seat.GenerateWorkSegment(now, false)

		assert.Error(t, err)
		assert.Zero(t, workSegment)
	})
}
