package utils

import (
	"github.com/kr/pretty"
	"testing"
	"time"
)

type Input struct {
	NetStudyDuration         time.Duration
	IsWorkNameSet            bool
	YesterdayContinuedActive bool
	CurrentStateStarted      time.Time
	LastActiveAt             time.Time
	PreviousRankPoint        int
}

type TestCase struct {
	Input  Input
	Output int
}

func TestCalcRankPoint(t *testing.T) {
	testCases := []TestCase{
		{
			Input: Input{
				NetStudyDuration:         57 * time.Minute,
				IsWorkNameSet:            false,
				YesterdayContinuedActive: false,
				CurrentStateStarted:      JstNow().Add(-time.Hour),
				LastActiveAt:             JstNow().Add(-time.Hour),
				PreviousRankPoint:        0,
			},
			Output: 5,
		},
		{
			Input: Input{
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
			Input: Input{
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
			Input: Input{
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
