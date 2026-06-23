package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateRetryIntervalSec(t *testing.T) {
	tests := []struct {
		name                string
		numContinuousFailed int
		want                float64
	}{
		{
			name:                "zero_failures",
			numContinuousFailed: 0,
			want:                1,
		},
		{
			name:                "one_failure",
			numContinuousFailed: 1,
			want:                1.2,
		},
		{
			name:                "two_failures",
			numContinuousFailed: 2,
			want:                1.44,
		},
		{
			name:                "three_failures",
			numContinuousFailed: 3,
			want:                1.728,
		},
		{
			name:                "four_failures",
			numContinuousFailed: 4,
			want:                2.0736,
		},
		{
			name:                "five_failures",
			numContinuousFailed: 5,
			want:                2.48832,
		},
		{
			name:                "ten_failures",
			numContinuousFailed: 10,
			want:                6.191736422,
		},
		{
			name:                "twenty_failures",
			numContinuousFailed: 20,
			want:                38.337599924474700,
		},
		{ // 単純に計算すると300を超えるが、最大値は300
			name:                "caps_at_300_seconds",
			numContinuousFailed: 50,
			want:                300,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDeltaf(
				t,
				tt.want,
				CalculateRetryIntervalSec(RetryIntervalCalculationBase, tt.numContinuousFailed),
				0.1,
				"CalculateRetryIntervalSec(%v)",
				tt.numContinuousFailed,
			)
		})
	}
}
