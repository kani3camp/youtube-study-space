package utils

import (
	"testing"
	"time"

	"app.modules/core/myfirestore"

	"github.com/stretchr/testify/assert"
)

func TestTimezoneOffsetStringOf(t *testing.T) {
	type TestCase struct {
		Input    time.Time
		Expected string
	}
	testCases := []TestCase{
		{
			Input:    time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			Expected: "+09:00",
		},
		{
			Input:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			Expected: "+00:00",
		},
		{
			Input:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC+9", 9*60*60)),
			Expected: "+09:00",
		},
		{
			Input:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC-9", -9*60*60)),
			Expected: "-09:00",
		},
		{
			Input:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC+9:30", 9*60*60+30*60)),
			Expected: "+09:30",
		},
		{
			Input:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.FixedZone("UTC-9:30", -9*60*60-30*60)),
			Expected: "-09:30",
		},
	}
	for _, testCase := range testCases {
		offset := TimezoneOffsetStringOf(testCase.Input)
		assert.Equal(t, testCase.Expected, offset)
	}
}

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
		Seat             myfirestore.SeatDoc
		Now              time.Time
		ExpectedDuration time.Duration
	}
	testCases := [...]TestCase{
		{
			Seat: myfirestore.SeatDoc{
				State:                  myfirestore.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			Now:              time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			ExpectedDuration: 0,
		},
		{
			Seat: myfirestore.SeatDoc{
				State:                  myfirestore.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			Now:              time.Date(2021, 1, 1, 1, 1, 1, 0, JapanLocation()),
			ExpectedDuration: 1*time.Hour + 1*time.Minute + 1*time.Second,
		},
		{
			Seat: myfirestore.SeatDoc{
				State:                  myfirestore.WorkState,
				CurrentStateStartedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				DailyCumulativeWorkSec: 0,
			},
			Now:              time.Date(2021, 1, 1, 0, 1, 0, 0, JapanLocation()),
			ExpectedDuration: 1 * time.Minute,
		},

		{
			Seat: myfirestore.SeatDoc{
				State:                  myfirestore.WorkState,
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

func TestDateRange(t *testing.T) {
	type TestCase struct {
		Name     string
		From     time.Time
		To       time.Time
		Expected []time.Time
	}
	testCases := [...]TestCase{
		{
			Name:     "from = toなら空スライスを返すこと",
			From:     time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			To:       time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			Expected: []time.Time{},
		},
		{
			Name: "連続2日なら1日目だけ返すこと",
			From: time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			To:   time.Date(2021, 1, 2, 0, 0, 0, 0, JapanLocation()),
			Expected: []time.Time{
				time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			},
		},
		{
			Name: "連続3日なら1日目と2日目だけ返すこと",
			From: time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
			To:   time.Date(2021, 1, 3, 0, 0, 0, 0, JapanLocation()),
			Expected: []time.Time{
				time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				time.Date(2021, 1, 2, 0, 0, 0, 0, JapanLocation()),
			},
		},
		{
			Name: "時間が設定されていても無視すること",
			From: time.Date(2021, 1, 1, 11, 11, 11, 11, JapanLocation()),
			To:   time.Date(2021, 1, 3, 1, 1, 1, 0, JapanLocation()),
			Expected: []time.Time{
				time.Date(2021, 1, 1, 0, 0, 0, 0, JapanLocation()),
				time.Date(2021, 1, 2, 0, 0, 0, 0, JapanLocation()),
			},
		},
	}
	for _, testCase := range testCases {
		out := DateRange(testCase.From, testCase.To)
		if len(out) != len(testCase.Expected) {
			t.Errorf("name: %s, expected: %#v, result: %#v", testCase.Name, testCase.Expected, out)
		}
		for i, v := range out {
			if !v.Equal(testCase.Expected[i]) {
				t.Errorf("name: %s, expected: %#v, result: %#v", testCase.Name, testCase.Expected, out)
			}
		}
	}
}
