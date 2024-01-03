package myfirestore

import (
	"testing"
	"time"
)

func TestWorkHistoryDoc_WorkDurationOfDate(t *testing.T) {
	type TestCase struct {
		Name        string
		WorkHistory WorkHistoryDoc
		Date        time.Time
		Output      time.Duration
	}
	testCases := [...]TestCase{
		{
			Name: "dateに収まっている場合",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 1, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 1, 11, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC),
			Output: 10 * time.Hour,
		},
		{
			Name: "startedAtがdateより前の場合",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 3, 31, 23, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 1, 5, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 1, 0, time.UTC),
			Output: 5 * time.Hour,
		},
		{
			Name: "endedAtがdateより後の場合",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 22, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 2, 3, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC),
			Output: 2 * time.Hour,
		},
		{
			Name: "startedAtがdateより前、endedAtがdateより後の場合",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 3, 31, 23, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 2, 3, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC),
			Output: 24 * time.Hour,
		},
		{
			Name: "dateに全く収まっていない場合",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 22, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 2, 3, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 3, 0, 0, 0, 0, time.UTC),
			Output: 0,
		},
	}
	for _, testCase := range testCases {
		out := testCase.WorkHistory.WorkDurationOfDate(testCase.Date)
		if out != testCase.Output {
			t.Errorf("testcase: %s, expected: %v, actual: %v", testCase.Name, testCase.Output, out)
		}
	}
}

func TestDailyWorkHistoryDoc_WorkDuration(t *testing.T) {
	type TestCase struct {
		Name   string
		Input  DailyWorkHistoryDoc
		Output time.Duration
	}
	testCases := [...]TestCase{
		{
			Name: "WorkSecが0の場合",
			Input: DailyWorkHistoryDoc{
				WorkSec: 0,
			},
			Output: 0,
		},
		{
			Name: "WorkSecが1の場合",
			Input: DailyWorkHistoryDoc{
				WorkSec: 1,
			},
			Output: 1 * time.Second,
		},
		{
			Name: "WorkSecが3600の場合",
			Input: DailyWorkHistoryDoc{
				WorkSec: 3600,
			},
			Output: 1 * time.Hour,
		},
		{
			Name: "WorkSecが3601の場合",
			Input: DailyWorkHistoryDoc{
				WorkSec: 3601,
			},
			Output: 1*time.Hour + 1*time.Second,
		},
	}
	for _, testCase := range testCases {
		out := testCase.Input.WorkDuration()
		if out != testCase.Output {
			t.Errorf("testcase: %s, expected: %v, actual: %v", testCase.Name, testCase.Output, out)
		}
	}
}
