package utils

import (
	"testing"
	"time"

	"app.modules/core/repository"

	"github.com/stretchr/testify/assert"
)

func TestTodaySeconds(t *testing.T) {
	type TestCase struct {
		T               time.Time
		ExpectedSeconds int
	}
	testCases := []TestCase{
		{
			T: time.Date(2021, 0, 1, 3, 40, 4, 0, JapanLocation()),
			ExpectedSeconds: int((time.Duration(3)*time.Hour +
				time.Duration(40)*time.Minute +
				time.Duration(4)*time.Second).Seconds()),
		},
		{
			T:               time.Date(2021, 10, 1, 0, 0, 0, 0, JapanLocation()),
			ExpectedSeconds: 0,
		},
	}

	for _, testCase := range testCases {
		seconds := SecondsOfDay(testCase.T)
		assert.Equal(t, testCase.ExpectedSeconds, seconds)
	}
}

func TestDivideStringEqually(t *testing.T) {
	type TestCase struct {
		InSize     int
		InStrings  []string
		OutStrings [][]string
	}
	testCases := []TestCase{
		{
			InSize: 2,
			InStrings: []string{
				"1", "2", "3", "4", "5",
			},
			OutStrings: [][]string{
				{
					"1", "3", "5",
				},
				{
					"2", "4",
				},
			},
		},
	}

	for _, testCase := range testCases {
		strings := DivideStringEqually(testCase.InSize, testCase.InStrings)
		assert.Equal(t, testCase.OutStrings, strings)
	}
}

func TestMatchEmojiCommandIndex(t *testing.T) {
	s := "こんにちは" + TestEmojiIn0 + "jio"
	expected := "jio"
	loc := FindEmojiCommandIndex(s, InString)
	if s[loc[1]:] != expected {
		t.Error()
	}
}

func TestIsEmojiCommandString(t *testing.T) {
	type TestCase struct {
		Input  string
		Output bool
	}
	testCases := [...]TestCase{
		{
			Input:  "hello",
			Output: false,
		},
		{
			Input:  ":_command:",
			Output: true,
		},
		{
			Input:  TestEmoji360Min0,
			Output: true,
		},
		{
			Input:  TestEmojiInfoD0,
			Output: true,
		},
		{
			Input:  "dev" + TestEmojiIn0,
			Output: true,
		},
	}

	for _, testCase := range testCases {
		out := MatchEmojiCommandString(testCase.Input)
		if out != testCase.Output {
			t.Error("input: ", testCase.Input)
			t.Error("result: ", out)
			t.Error("expected: ", testCase.Output)
		}
	}
}

func TestReplaceAnyEmojiCommandStringWithSpace(t *testing.T) {
	type TestCase struct {
		Input  string
		Output string
	}
	testCases := [...]TestCase{
		{
			Input:  TestEmojiIn0,
			Output: HalfWidthSpace,
		},
		{
			Input:  TestEmojiIn0 + "orange" + TestEmojiWork0 + "apple",
			Output: " orange apple",
		},
	}
	for _, testCase := range testCases {
		out := ReplaceAnyEmojiCommandStringWithSpace(testCase.Input)
		if out != testCase.Output {
			t.Error("input: ", testCase.Input)
			t.Error("result: ", out)
			t.Error("expected: ", testCase.Output)
		}
	}
}

func TestDurationToString(t *testing.T) {
	type TestCase struct {
		Input  time.Duration
		Output string
	}
	testCases := [...]TestCase{
		{
			Input:  time.Duration(0),
			Output: "0分",
		},
		{
			Input:  1 * time.Minute,
			Output: "1分",
		},
		{
			Input:  1 * time.Hour,
			Output: "1時間0分",
		},
		{
			Input:  1*time.Hour + 1*time.Minute + 1*time.Second,
			Output: "1時間1分",
		},
		{
			Input:  1*time.Hour + 1*time.Minute + 1*time.Second + 1*time.Millisecond,
			Output: "1時間1分",
		},
		{
			Input:  24*time.Hour + 1*time.Minute,
			Output: "24時間1分",
		},
	}
	for _, testCase := range testCases {
		out := DurationToString(testCase.Input)
		if out != testCase.Output {
			t.Error("input: ", testCase.Input)
			t.Error("result: ", out)
			t.Error("expected: ", testCase.Output)
		}
	}
}

func TestRealTimeDailyTotalStudyDurationOfSeat(t *testing.T) {
	type TestCase struct {
		Seat             repository.SeatDoc
		Now              time.Time
		ExpectedDuration time.Duration
	}
	testCases := [...]TestCase{
		{
			Seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			Now:              time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			ExpectedDuration: 0,
		},
		{
			Seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			Now:              time.Date(2021, 1, 1, 1, 1, 1, 0, JapanLocation()),
			ExpectedDuration: 1*time.Hour + 1*time.Minute + 1*time.Second,
		},
		{
			Seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			Now:              time.Date(2021, 1, 1, 0, 1, 0, 0, JapanLocation()),
			ExpectedDuration: 1 * time.Minute,
		},

		{
			Seat: repository.SeatDoc{
				State:                  repository.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 12, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: int((time.Hour).Seconds()),
			},
			Now:              time.Date(2021, 1, 1, 12, 30, 0, 0, JapanLocation()),
			ExpectedDuration: 1*time.Hour + 30*time.Minute,
		},
	}
	for _, testCase := range testCases {
		duration, err := RealTimeDailyTotalStudyDurationOfSeat(testCase.Seat, testCase.Now)
		if err != nil {
			t.Fatal(err)
		}
		if duration != testCase.ExpectedDuration {
			t.Errorf("input: %#v", testCase)
			t.Error("result: ", duration)
			t.Error("expected: ", testCase.ExpectedDuration)
		}
	}
}
