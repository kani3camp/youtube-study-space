package utils

import (
	"time"
)

// TimeCalculator provides time calculation utilities with a fixed reference time
type TimeCalculator struct {
	now time.Time
}

// NewTimeCalculator creates a new TimeCalculator with current JST time
func NewTimeCalculator() *TimeCalculator {
	return &TimeCalculator{
		now: JstNow(),
	}
}

// NewTimeCalculatorWithTime creates a new TimeCalculator with specified time
func NewTimeCalculatorWithTime(t time.Time) *TimeCalculator {
	return &TimeCalculator{
		now: t,
	}
}

// RemainingMinutes calculates remaining minutes until the specified time
func (tc *TimeCalculator) RemainingMinutes(until time.Time) int {
	return int(NoNegativeDuration(until.Sub(tc.now)).Minutes())
}

// ElapsedMinutes calculates elapsed minutes since the specified time
func (tc *TimeCalculator) ElapsedMinutes(from time.Time) int {
	return int(tc.now.Sub(from).Minutes())
}

// ElapsedSeconds calculates elapsed seconds since the specified time
func (tc *TimeCalculator) ElapsedSeconds(from time.Time) int {
	return int(tc.now.Sub(from).Seconds())
}

// IsExpired checks if the specified time has passed
func (tc *TimeCalculator) IsExpired(until time.Time) bool {
	return tc.now.After(until)
}

// IsWithinDuration checks if the specified time is within the given duration from now
func (tc *TimeCalculator) IsWithinDuration(target time.Time, duration time.Duration) bool {
	diff := tc.now.Sub(target)
	if diff < 0 {
		diff = -diff
	}
	return diff <= duration
}

// AddMinutes returns a new time with the specified minutes added to the current time
func (tc *TimeCalculator) AddMinutes(minutes int) time.Time {
	return tc.now.Add(time.Duration(minutes) * time.Minute)
}

// GetCurrentTime returns the current reference time
func (tc *TimeCalculator) GetCurrentTime() time.Time {
	return tc.now
}