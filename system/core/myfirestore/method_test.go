package myfirestore

import (
	"reflect"
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
			Name: "dateに収まっている",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 1, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 1, 11, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC),
			Output: 10 * time.Hour,
		},
		{
			Name: "startedAtがdateより前",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 3, 31, 23, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 1, 5, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 1, 0, time.UTC),
			Output: 5 * time.Hour,
		},
		{
			Name: "endedAtがdateより後",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 22, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 2, 3, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC),
			Output: 2 * time.Hour,
		},
		{
			Name: "startedAtがdateより前、endedAtがdateより後",
			WorkHistory: WorkHistoryDoc{
				StartedAt: time.Date(2020, 3, 31, 23, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 2, 3, 0, 0, 0, time.UTC),
			},
			Date:   time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC),
			Output: 24 * time.Hour,
		},
		{
			Name: "dateに全く収まっていない",
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
			Name: "WorkSecが0",
			Input: DailyWorkHistoryDoc{
				WorkSec: 0,
			},
			Output: 0,
		},
		{
			Name: "WorkSecが1",
			Input: DailyWorkHistoryDoc{
				WorkSec: 1,
			},
			Output: 1 * time.Second,
		},
		{
			Name: "WorkSecが3600",
			Input: DailyWorkHistoryDoc{
				WorkSec: 3600,
			},
			Output: 1 * time.Hour,
		},
		{
			Name: "WorkSecが3601",
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

func TestWorkHistoryDoc_DivideToDailyWorkSecList(t *testing.T) {
	type TestCase struct {
		Name   string
		Input  WorkHistoryDoc
		Output []DailyWorkSec
	}
	testCases := [...]TestCase{
		{
			Name: "startedAtとendedAtが同じ日",
			Input: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 1, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 1, 11, 0, 0, 0, time.UTC),
			},
			Output: []DailyWorkSec{
				{
					Date:    "2020-04-01",
					WorkSec: 10 * 60 * 60,
				},
			},
		},
		{
			Name: "startedAtとendedAtが隔日",
			Input: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 22, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 2, 3, 0, 0, 0, time.UTC),
			},
			Output: []DailyWorkSec{
				{
					Date:    "2020-04-01",
					WorkSec: 2 * 60 * 60,
				},
				{
					Date:    "2020-04-02",
					WorkSec: 3 * 60 * 60,
				},
			},
		},
		{
			Name: "startedAtとendedAtが3日以上離れている",
			Input: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 22, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 4, 3, 0, 0, 0, time.UTC),
			},
			Output: []DailyWorkSec{
				{
					Date:    "2020-04-01",
					WorkSec: 2 * 60 * 60,
				},
				{
					Date:    "2020-04-02",
					WorkSec: 24 * 60 * 60,
				},
				{
					Date:    "2020-04-03",
					WorkSec: 24 * 60 * 60,
				},
				{
					Date:    "2020-04-04",
					WorkSec: 3 * 60 * 60,
				},
			},
		},
	}
	for _, testCase := range testCases {
		out := testCase.Input.DivideToDailyWorkSecList()
		if !reflect.DeepEqual(out, testCase.Output) {
			t.Errorf("testcase: %s, expected: %+v, actual: %+v", testCase.Name, testCase.Output, out)
		}
	}
}
