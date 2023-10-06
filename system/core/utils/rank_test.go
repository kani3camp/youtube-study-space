package utils

import (
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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
	Input  InputCalcRankPoint
	Output int
}

func TestCalcRankPoint(t *testing.T) {
	testCases := []TestCaseRP{
		{
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
	}

	for _, testCase := range testCases {
		in := testCase.Input
		rp, err := CalcNewRPExitRoom(in.NetStudyDuration, in.IsWorkNameSet, in.YesterdayContinuedActive, in.CurrentStateStarted, in.LastActiveAt, in.PreviousRankPoint)
		if err != nil {
			t.Error(err)
		}
		if rp != testCase.Output {
			t.Errorf("input: %# v\n", pretty.Formatter(in))
			t.Errorf("result: %d\n", rp)
			t.Errorf("expected: %d\n", testCase.Output)
		}
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			in := testCase.Input
			lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint, err := DailyUpdateRankPoint(in.LastPenaltyImposedDays, in.IsContinuousActive, in.CurrentActivityStateStarted, in.RankPoint, in.LastEntered, in.LastExited, in.JstNow)
			if err != nil {
				t.Error(err)
			}
			resultOutput := OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      lastPenaltyImposedDays,
				IsContinuousActive:          isContinuousActive,
				CurrentActivityStateStarted: currentActivityStateStarted,
				RankPoint:                   rankPoint,
			}
			assert.Equalf(t, testCase.Output, resultOutput, "")
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
			name: "",
			args: args{
				lastEntered: TIME1,
				lastExited:  TIME2,
				now:         TIME3,
			},
			want: TIME2,
		},
		{
			name: "",
			args: args{
				lastEntered: TIME2,
				lastExited:  TIME1,
				now:         TIME3,
			},
			want: TIME3,
		},
		{
			name: "",
			args: args{
				lastEntered: time.Time{},
				lastExited:  time.Time{},
				now:         TIME3,
			},
			want: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, LastActiveAt(tt.args.lastEntered, tt.args.lastExited, tt.args.now), "LastActiveAt(%v, %v, %v)", tt.args.lastEntered, tt.args.lastExited, tt.args.now)
		})
	}
}
