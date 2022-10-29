package utils

import (
	"github.com/kr/pretty"
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
	Input  InputDailyUpdateRankPoint
	Output OutputDailyUpdateRankPoint
}

func TestDailyUpdateRankPoint(t *testing.T) {
	jstNow := JstNow()
	testCases := []TestCaseDailyUpdateRankPoint{
		{ // だいぶ前に登録だけした人
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
		{ // 前日に入室した人
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
		{ // 一昨日から入室してる人
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
		{ // 昨日から入室しなくなった人
			Input: InputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          true,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -3),
				RankPoint:                   100,
				LastEntered:                 jstNow.AddDate(0, 0, -2),
				LastExited:                  jstNow.AddDate(0, 0, -2).Add(time.Minute * 30),
				JstNow:                      jstNow,
			},
			Output: OutputDailyUpdateRankPoint{
				LastPenaltyImposedDays:      0,
				IsContinuousActive:          false,
				CurrentActivityStateStarted: jstNow.AddDate(0, 0, -1),
				RankPoint:                   100,
			},
		},
		{ // 一昨日から入室しなくなった人
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
		//{
		//	Input: InputDailyUpdateRankPoint{
		//		LastPenaltyImposedDays: ,
		//		IsContinuousActive: ,
		//		CurrentActivityStateStarted: ,
		//		RankPoint: ,
		//		LastEntered: ,
		//		LastExited: ,
		//		JstNow: ,
		//	},
		//	Output: OutputDailyUpdateRankPoint{
		//		LastPenaltyImposedDays: ,
		//		IsContinuousActive: ,
		//		CurrentActivityStateStarted: ,
		//		RankPoint: ,
		//	},
		//},
	}
	
	for _, testCase := range testCases {
		in := testCase.Input
		lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint, err := DailyUpdateRankPoint(in.LastPenaltyImposedDays, in.IsContinuousActive, in.CurrentActivityStateStarted, in.RankPoint, in.LastEntered, in.LastExited, in.JstNow)
		if err != nil {
			t.Error(err)
		}
		if lastPenaltyImposedDays != testCase.Output.LastPenaltyImposedDays ||
			isContinuousActive != testCase.Output.IsContinuousActive ||
			currentActivityStateStarted != testCase.Output.CurrentActivityStateStarted ||
			rankPoint != testCase.Output.RankPoint {
			t.Errorf("input: %# v\n", pretty.Formatter(in))
			t.Error("result: ", lastPenaltyImposedDays, isContinuousActive, currentActivityStateStarted, rankPoint)
			t.Errorf("expected: %# v\n", pretty.Formatter(testCase.Output))
		}
	}
}
