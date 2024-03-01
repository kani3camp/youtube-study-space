package myfirestore

import (
	"reflect"
	"testing"
	"time"
)

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
		Name          string
		Input         WorkHistoryDoc
		InputLocation *time.Location
		Output        []DailyWorkSec
	}
	testCases := [...]TestCase{
		{
			Name: "startedAtとendedAtが同じ日",
			Input: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 1, 0, 0, 0, time.UTC),
				EndedAt:   time.Date(2020, 4, 1, 11, 0, 0, 0, time.UTC),
			},
			InputLocation: time.UTC,
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
			InputLocation: time.UTC,
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
			InputLocation: time.UTC,
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
		{
			Name: "startedAtとendedAtが同じ日(JST)",
			Input: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 1, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)).UTC(),
				EndedAt:   time.Date(2020, 4, 1, 11, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)).UTC(),
			},
			InputLocation: time.FixedZone("Asia/Tokyo", 9*60*60),
			Output: []DailyWorkSec{
				{
					Date:    "2020-04-01",
					WorkSec: 10 * 60 * 60,
				},
			},
		},
		{
			Name: "startedAtとendedAtが隔日(JST)",
			Input: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 22, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)).UTC(),
				EndedAt:   time.Date(2020, 4, 2, 3, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)).UTC(),
			},
			InputLocation: time.FixedZone("Asia/Tokyo", 9*60*60),
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
			Name: "startedAtとendedAtが3日以上離れている(JST)",
			Input: WorkHistoryDoc{
				StartedAt: time.Date(2020, 4, 1, 22, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)).UTC(),
				EndedAt:   time.Date(2020, 4, 4, 3, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)).UTC(),
			},
			InputLocation: time.FixedZone("Asia/Tokyo", 9*60*60),
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
		out := testCase.Input.DivideToDailyWorkSecList(testCase.InputLocation)
		if !reflect.DeepEqual(out, testCase.Output) {
			t.Errorf("testcase: %s, expected: %+v, actual: %+v", testCase.Name, testCase.Output, out)
		}
	}
}
