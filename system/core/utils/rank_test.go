package utils

import (
	"github.com/kr/pretty"
	"testing"
	"time"
)

type Input struct {
	NetStudyDuration    time.Duration
	IsWorkNameSet       bool
	ContinuousEntryDays int
	PreviousRankPoint   int
}

type TestCase struct {
	Input  Input
	Output int
}

func TestCalcRankPoint(t *testing.T) {
	testCases := []TestCase{
		{
			Input: Input{
				NetStudyDuration:    time.Duration(5) * time.Minute,
				IsWorkNameSet:       false,
				ContinuousEntryDays: 0,
				PreviousRankPoint:   0,
			},
			Output: 5,
		},
		{
			Input: Input{
				NetStudyDuration:    time.Duration(57) * time.Minute,
				IsWorkNameSet:       true,
				ContinuousEntryDays: 0,
				PreviousRankPoint:   0,
			},
			Output: 62,
		},
		{
			Input: Input{
				NetStudyDuration:    time.Duration(57) * time.Minute,
				IsWorkNameSet:       false,
				ContinuousEntryDays: 30,
				PreviousRankPoint:   0,
			},
			Output: 74,
		},
	}
	
	for _, testCase := range testCases {
		in := testCase.Input
		rp := CalcNewRPExitRoom(in.NetStudyDuration, in.IsWorkNameSet, in.ContinuousEntryDays, in.PreviousRankPoint)
		if rp != testCase.Output {
			t.Errorf("input: %# v\n", pretty.Formatter(in))
			t.Errorf("result: %d\n", rp)
			t.Errorf("expected: %d\n", testCase.Output)
		}
	}
}
