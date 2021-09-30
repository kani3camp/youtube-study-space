package utils

import (
	"fmt"
	"github.com/kr/pretty"
	"reflect"
	"testing"
)

func TestGetRank(t *testing.T) {
	type TestCase struct {
		TotalSec int
		Expected Rank
	}
	testCases := []TestCase{
		{
			TotalSec: 0,
			Expected: Rank{
				GreaterThanOrEqualToHours: 0,
				LessThanHours:             5,
				ColorCode:                 "#fff",
			},
		},
		{
			TotalSec: 36000,
			Expected: Rank{
				GreaterThanOrEqualToHours: 10,
				LessThanHours:             20,
				ColorCode:                 "#FF9580",
			},
		},
		{
			TotalSec: 500000,	// = 138.888889 hours
			Expected: Rank{
				GreaterThanOrEqualToHours: 100,
				LessThanHours:             150,
				ColorCode:                 "#80FF95",
			},
		},
	}
	
	for _, testCase := range testCases {
		rank, err := GetRank(testCase.TotalSec)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(rank, testCase.Expected) {
			fmt.Printf("%# v\n", pretty.Formatter(rank))
			fmt.Printf("%# v\n", pretty.Formatter(testCase.Expected))
			t.Error("rank do not match.")
		}
	}
}
