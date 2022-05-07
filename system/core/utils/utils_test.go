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
