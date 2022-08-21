package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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
