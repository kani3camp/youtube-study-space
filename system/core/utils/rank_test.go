package utils

import (
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

type InputCalcRankPoint struct {
	NetStudyDuration         time.Duration
	IsWorkNameSet            bool
	YesterdayContinuedActive bool
	CurrentStateStarted      time.Time
	LastActiveAt             time.Time
	PreviousRankPoint        int
}

type TestCaseRP struct {
	Name   string
	Input  InputCalcRankPoint
	Output int
}

func TestCalcNewRPExitRoom(t *testing.T) {
	testCases := []TestCaseRP{
		{
			Name: "基本的なケース（作業名なし）",
			Input: InputCalcRankPoint{
				NetStudyDuration:         57 * time.Minute,
				IsWorkNameSet:            false,
				YesterdayContinuedActive: false,
				CurrentStateStarted:      JstNow().Add(-time.Hour),
				LastActiveAt:             JstNow().Add(-time.Hour),
				PreviousRankPoint:        0,
			},
			Output: 57,
		},
		{
			Name: "作業名あり",
			Input: InputCalcRankPoint{
				NetStudyDuration:         57 * time.Minute,
				IsWorkNameSet:            true,
				YesterdayContinuedActive: false,
				CurrentStateStarted:      JstNow().Add(-time.Hour),
				LastActiveAt:             JstNow().Add(-time.Hour),
				PreviousRankPoint:        0,
			},
			Output: 62,
		},
		{
			Name: "連続アクティブ日あり",
			Input: InputCalcRankPoint{
				NetStudyDuration:         57 * time.Minute,
				IsWorkNameSet:            false,
				YesterdayContinuedActive: true,
				CurrentStateStarted:      JstNow().Add(-time.Hour * 24 * 30),
				LastActiveAt:             JstNow().Add(-time.Hour),
				PreviousRankPoint:        0,
			},
			Output: 74,
		},
		{
			Name: "短時間",
			Input: InputCalcRankPoint{
				NetStudyDuration:         40 * time.Minute,
				IsWorkNameSet:            false,
				YesterdayContinuedActive: false,
				CurrentStateStarted:      JstNow().Add(-time.Minute * 40),
				LastActiveAt:             JstNow().Add(-time.Minute * 40),
				PreviousRankPoint:        0,
			},
			Output: 40,
		},
		{
			Name: "高いランクポイント（倍率減少）",
			Input: InputCalcRankPoint{
				NetStudyDuration:         100 * time.Minute,
				IsWorkNameSet:            false,
				YesterdayContinuedActive: false,
				CurrentStateStarted:      JstNow().Add(-time.Hour * 2),
				LastActiveAt:             JstNow().Add(-time.Hour * 2),
				PreviousRankPoint:        50000,
			},
			Output: 50070, // 100 * 0.7 + 50000
		},
		{
			Name: "高いランクポイント（最低倍率）",
			Input: InputCalcRankPoint{
				NetStudyDuration:         100 * time.Minute,
				IsWorkNameSet:            false,
				YesterdayContinuedActive: false,
				CurrentStateStarted:      JstNow().Add(-time.Hour * 2),
				LastActiveAt:             JstNow().Add(-time.Hour * 2),
				PreviousRankPoint:        90000,
			},
			Output: 90030, // 100 * 0.3 + 90000
		},
		{
			Name: "作業名と連続アクティブ（複合倍率）",
			Input: InputCalcRankPoint{
				NetStudyDuration:         100 * time.Minute,
				IsWorkNameSet:            true,
				YesterdayContinuedActive: true,
				CurrentStateStarted:      JstNow().Add(-time.Hour * 24 * 2),
				LastActiveAt:             JstNow().Add(-time.Hour),
				PreviousRankPoint:        0,
			},
			Output: 112, // 100 * 1.1 * (1 + 0.02)
		},
		{
			Name: "上限値を超えない",
			Input: InputCalcRankPoint{
				NetStudyDuration:         57 * time.Minute,
				IsWorkNameSet:            false,
				YesterdayContinuedActive: false,
				CurrentStateStarted:      JstNow().Add(-time.Hour),
				LastActiveAt:             JstNow().Add(-time.Hour),
				PreviousRankPoint:        99990,
			},
			Output: 99999, // 99,990 + 57 * 0.3 = 99,990 + 17 = 100,007 > 99,999
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			in := testCase.Input
			rp, err := CalcNewRPExitRoom(in.NetStudyDuration, in.IsWorkNameSet, in.YesterdayContinuedActive, in.CurrentStateStarted, in.LastActiveAt, in.PreviousRankPoint)
			assert.NoError(t, err)
			assert.Equal(t, testCase.Output, rp, "input: %# v", pretty.Formatter(in))
		})
	}
}

type InputDailyUpdateRankPoint struct {
	LastPenaltyImposedDays      int
	IsContinuousActive          bool
	CurrentActivityStateStarted time.Time
	RankPoint                   int
	LastEntered                 time.Time
	LastExited                  time.Time
	JstNow                      time.Time
}

type OutputDailyUpdateRankPoint struct {
	LastPenaltyImposedDays      int
	IsContinuousActive          bool
	CurrentActivityStateStarted time.Time
	RankPoint                   int
}

type TestCaseDailyUpdateRankPoint struct {
	Name   string
	Input  InputDailyUpdateRankPoint
	Output OutputDailyUpdateRankPoint
}

func TestDailyUpdateRankPoint(t *testing.T) {
	jstNow := JstNow()
	testCases := []TestCaseDailyUpdateRankPoint{
		{
			Name: "だいぶ前に登録だけした人",
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      30,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: time.Time{},
				RankPoint:                   0,
				LastEntered:                 time.Time{},
				LastExited:                  time.Time{},
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      30,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: time.Time{},
				RankPoint:                   0,
			},
		},
		{
			Name: "前日に入室した人",
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          true,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -1),
				RankPoint:                   100,
				LastEntered:                 jstNow.AddDate(0, 0, -1),
				LastExited:                  jstNow.AddDate(0, 0, -1).Add(time.Minute * 120),
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          true,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -1),
				RankPoint:                   100,
			},
		},
		{
			Name: "一昨日から入室してる人",
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          true,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -3),
				RankPoint:                   100,
				LastEntered:                 jstNow.AddDate(0, 0, -1),
				LastExited:                  jstNow.AddDate(0, 0, -1).Add(time.Minute * 30),
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          true,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -3),
				RankPoint:                   100,
			},
		},
		{
			Name: "昨日から入室しなくなった人",
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          true,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -3),
				RankPoint:                   100,
				LastEntered:                 jstNow.AddDate(0, 0, -2),
				LastExited:                  jstNow.AddDate(0, 0, -2),
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -2),
				RankPoint:                   100,
			},
		},
		{
			Name: "一昨日から入室しなくなった人",
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -2),
				RankPoint:                   100,
				LastEntered:                 jstNow.AddDate(0, 0, -3),
				LastExited:                  jstNow.AddDate(0, 0, -3).Add(time.Minute * 30),
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -2),
				RankPoint:                   100,
			},
		},
		{
			Name: "今日入室した人",
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: time.Time{},
				RankPoint:                   100,
				LastEntered:                 jstNow,
				LastExited:                  time.Time{},
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          true,
				CurrentActivityStateStarted: time.Time{},
				RankPoint:                   100,
			},
		},
		{
			Name: "3日間入室しなかった人（ペナルティ適用）",
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -3),
				RankPoint:                   100,
				LastEntered:                 jstNow.AddDate(0, 0, -3),
				LastExited:                  jstNow.AddDate(0, 0, -3),
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      3,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -3),
				RankPoint:                   80, // 100 * 0.8 (3日間のペナルティ)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			in := testCase.Input
			lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint, err := DailyUpdateRankPoint(in.LastPenaltyImposedDays, in.IsContinuousActive, in.CurrentActivityStateStarted, in.RankPoint, in.LastEntered, in.LastExited, in.JstNow)
			assert.NoError(t, err)
			resultOutput := OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      lastPenaltyImposedDays,
				IsContinuousActive:          isContinuousActive,
				CurrentActivityStateStarted: currentActivityStateStarted,
				RankPoint:                   rankPoint,
			}
			assert.Equal(t, testCase.Output, resultOutput)
		})
	}
}

func TestLastActiveAt(t *testing.T) {
	TIME1 := time.Date(2020, 1, 1, 0, 0, 0, 0, JapanLocation())
	TIME2 := time.Date(2020, 1, 2, 0, 0, 0, 0, JapanLocation())
	TIME3 := time.Date(2020, 1, 3, 0, 0, 0, 0, JapanLocation())

	type args struct {
		lastEntered time.Time
		lastExited  time.Time
		now         time.Time
	}
	tests := []struct {
		name string
		args args
		want time.Time
	}{
		{
			name: "退室している",
			args: args{
				lastEntered: TIME1,
				lastExited:  TIME2,
				now:         TIME3,
			},
			want: TIME2,
		},
		{
			name: "現在入室中",
			args: args{
				lastEntered: TIME2,
				lastExited:  TIME1,
				now:         TIME3,
			},
			want: TIME3,
		},
		{
			name: "アクティビティ記録なし",
			args: args{
				lastEntered: time.Time{},
				lastExited:  time.Time{},
				now:         TIME3,
			},
			want: time.Time{},
		},
		{
			name: "（基本ないけど）入室と退室が同じ時間",
			args: args{
				lastEntered: TIME1,
				lastExited:  TIME1,
				now:         TIME3,
			},
			want: TIME1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LastActiveAt(tt.args.lastEntered, tt.args.lastExited, tt.args.now)
			assert.Equal(t, tt.want, result, "LastActiveAt(%v, %v, %v)", tt.args.lastEntered, tt.args.lastExited, tt.args.now)
		})
	}
}

func TestCalcContinuousInactiveDays(t *testing.T) {
	tests := []struct {
		name          string
		lastActiveAt  time.Time
		expectedDays  int
		expectedError bool
	}{
		{
			name:          "1日間非アクティブ",
			lastActiveAt:  JstNow().AddDate(0, 0, -1),
			expectedDays:  1,
			expectedError: false,
		},
		{
			name:          "3日間非アクティブ",
			lastActiveAt:  JstNow().AddDate(0, 0, -3),
			expectedDays:  3,
			expectedError: false,
		},
		{
			name:          "7日間非アクティブ",
			lastActiveAt:  JstNow().AddDate(0, 0, -7),
			expectedDays:  7,
			expectedError: false,
		},
		{
			name:          "30日間非アクティブ",
			lastActiveAt:  JstNow().AddDate(0, 0, -30),
			expectedDays:  30,
			expectedError: false,
		},
		{
			name:          "未来の日付（エラーケース）",
			lastActiveAt:  JstNow().AddDate(0, 0, 1),
			expectedDays:  0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, err := CalcContinuousInactiveDays(tt.lastActiveAt)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedDays, days)
		})
	}
}

func TestCalcContinuousActiveDays(t *testing.T) {
	jstNow := JstNow()

	tests := []struct {
		name                     string
		yesterdayContinuedActive bool
		currentStateStarted      time.Time
		lastActiveAt             time.Time
		expectedDays             int
		expectedError            bool
	}{
		{
			name:                     "昨日まで2日連続アクティブ->昨日までの連続日数を返す",
			yesterdayContinuedActive: true,
			currentStateStarted:      jstNow.AddDate(0, 0, -2),
			lastActiveAt:             jstNow.AddDate(0, 0, -1),
			expectedDays:             1,
			expectedError:            false,
		},
		{
			name:                     "2日連続アクティブ",
			yesterdayContinuedActive: true,
			currentStateStarted:      jstNow.AddDate(0, 0, -1),
			lastActiveAt:             jstNow,
			expectedDays:             1,
			expectedError:            false,
		},
		{
			name:                     "4日連続アクティブ",
			yesterdayContinuedActive: true,
			currentStateStarted:      jstNow.AddDate(0, 0, -3),
			lastActiveAt:             jstNow,
			expectedDays:             3,
			expectedError:            false,
		},
		{
			name:                     "未来の開始日（エラーケース）",
			yesterdayContinuedActive: true,
			currentStateStarted:      jstNow.AddDate(0, 0, 1),
			lastActiveAt:             jstNow,
			expectedDays:             0,
			expectedError:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, err := CalcContinuousActiveDays(tt.yesterdayContinuedActive, tt.currentStateStarted, tt.lastActiveAt)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedDays, days)
		})
	}
}

func TestMagnificationByRP(t *testing.T) {
	tests := []struct {
		name     string
		rp       int
		expected float64
	}{
		{
			name:     "RP 0",
			rp:       0,
			expected: 1.0,
		},
		{
			name:     "RP 9999",
			rp:       9999,
			expected: 1.0,
		},
		{
			name:     "RP 10000",
			rp:       10000,
			expected: 1.0,
		},
		{
			name:     "RP 19999",
			rp:       19999,
			expected: 1.0,
		},
		{
			name:     "RP 20000",
			rp:       20000,
			expected: 0.95,
		},
		{
			name:     "RP 29999",
			rp:       29999,
			expected: 0.95,
		},
		{
			name:     "RP 30000",
			rp:       30000,
			expected: 0.9,
		},
		{
			name:     "RP 39999",
			rp:       39999,
			expected: 0.9,
		},
		{
			name:     "RP 40000",
			rp:       40000,
			expected: 0.8,
		},
		{
			name:     "RP 49999",
			rp:       49999,
			expected: 0.8,
		},
		{
			name:     "RP 50000",
			rp:       50000,
			expected: 0.7,
		},
		{
			name:     "RP 59999",
			rp:       59999,
			expected: 0.7,
		},
		{
			name:     "RP 60000",
			rp:       60000,
			expected: 0.6,
		},
		{
			name:     "RP 69999",
			rp:       69999,
			expected: 0.6,
		},
		{
			name:     "RP 70000",
			rp:       70000,
			expected: 0.5,
		},
		{
			name:     "RP 79999",
			rp:       79999,
			expected: 0.5,
		},
		{
			name:     "RP 80000",
			rp:       80000,
			expected: 0.4,
		},
		{
			name:     "RP 89999",
			rp:       89999,
			expected: 0.4,
		},
		{
			name:     "RP 90000",
			rp:       90000,
			expected: 0.3,
		},
		{
			name:     "RP 99999",
			rp:       99999,
			expected: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MagnificationByRP(tt.rp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPenaltyMagnificationByInactiveDays(t *testing.T) {
	tests := []struct {
		name                  string
		inactiveDays          int
		expectedMagnification float64
		expectedDays          int
	}{
		{
			name:                  "0日間非アクティブ",
			inactiveDays:          0,
			expectedMagnification: 1.0,
			expectedDays:          0,
		},
		{
			name:                  "1日間非アクティブ",
			inactiveDays:          1,
			expectedMagnification: 1.0,
			expectedDays:          0,
		},
		{
			name:                  "2日間非アクティブ",
			inactiveDays:          2,
			expectedMagnification: 1.0,
			expectedDays:          0,
		},
		{
			name:                  "3日間非アクティブ",
			inactiveDays:          3,
			expectedMagnification: 0.8,
			expectedDays:          3,
		},
		{
			name:                  "6日間非アクティブ",
			inactiveDays:          6,
			expectedMagnification: 0.8,
			expectedDays:          3,
		},
		{
			name:                  "7日間非アクティブ",
			inactiveDays:          7,
			expectedMagnification: 0.5,
			expectedDays:          7,
		},
		{
			name:                  "29日間非アクティブ",
			inactiveDays:          29,
			expectedMagnification: 0.5,
			expectedDays:          7,
		},
		{
			name:                  "30日間非アクティブ",
			inactiveDays:          30,
			expectedMagnification: 0.0,
			expectedDays:          30,
		},
		{
			name:                  "31日間非アクティブ",
			inactiveDays:          31,
			expectedMagnification: 0.0,
			expectedDays:          30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			magnification, days := PenaltyMagnificationByInactiveDays(tt.inactiveDays)
			assert.Equal(t, tt.expectedMagnification, magnification)
			assert.Equal(t, tt.expectedDays, days)
		})
	}
}

func TestApplyRPRange(t *testing.T) {
	tests := []struct {
		name     string
		rp       int
		expected int
	}{
		{
			name:     "マイナスのRP",
			rp:       -100,
			expected: 0,
		},
		{
			name:     "ゼロRP",
			rp:       0,
			expected: 0,
		},
		{
			name:     "通常のRP",
			rp:       50000,
			expected: 50000,
		},
		{
			name:     "最大RP",
			rp:       99999,
			expected: 99999,
		},
		{
			name:     "最大RPを超える",
			rp:       100000,
			expected: 99999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyRPRange(tt.rp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalcNewRPContinuousInactivity(t *testing.T) {
	tests := []struct {
		name                   string
		previousRP             int
		lastActiveAt           time.Time
		lastPenaltyImposedDays int
		expectedRP             int
		expectedDays           int
		expectedError          bool
	}{
		{
			name:                   "ペナルティ不要 - 同日",
			previousRP:             100,
			lastActiveAt:           JstNow(),
			lastPenaltyImposedDays: 0,
			expectedRP:             100,
			expectedDays:           0,
			expectedError:          false,
		},
		{
			name:                   "ペナルティ不要 - 既に適用済み",
			previousRP:             100,
			lastActiveAt:           JstNow().AddDate(0, 0, -3),
			lastPenaltyImposedDays: 3,
			expectedRP:             100,
			expectedDays:           3,
			expectedError:          false,
		},
		{
			name:                   "3日間ペナルティ",
			previousRP:             100,
			lastActiveAt:           JstNow().AddDate(0, 0, -3),
			lastPenaltyImposedDays: 0,
			expectedRP:             80, // 100 * 0.8
			expectedDays:           3,
			expectedError:          false,
		},
		{
			name:                   "7日間ペナルティ",
			previousRP:             100,
			lastActiveAt:           JstNow().AddDate(0, 0, -7),
			lastPenaltyImposedDays: 0,
			expectedRP:             50, // 100 * 0.5
			expectedDays:           7,
			expectedError:          false,
		},
		{
			name:                   "30日間ペナルティ",
			previousRP:             100,
			lastActiveAt:           JstNow().AddDate(0, 0, -30),
			lastPenaltyImposedDays: 0,
			expectedRP:             0, // 100 * 0.0
			expectedDays:           30,
			expectedError:          false,
		},
		{
			name:                   "エラーケース - lastPenaltyImposedDays > inactiveDays",
			previousRP:             100,
			lastActiveAt:           JstNow().AddDate(0, 0, -3),
			lastPenaltyImposedDays: 5,
			expectedRP:             0,
			expectedDays:           0,
			expectedError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rp, days, err := CalcNewRPContinuousInactivity(tt.previousRP, tt.lastActiveAt, tt.lastPenaltyImposedDays)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedRP, rp)
			assert.Equal(t, tt.expectedDays, days)
		})
	}
}
