package main

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"app.modules/core/mypage"
)

const (
	maxHistoryCoverageBytes     = 16 * 1024
	maxHistoryCoverageIntervals = 64
)

var errHistoryCoverage = errors.New("invalid history coverage")

// parseHistoryCoverage validates an operator-supplied completeness assertion;
// it cannot verify the historical inventory. Never fill gaps or extend an end.
func parseHistoryCoverage(value string) ([]mypage.Interval, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > maxHistoryCoverageBytes {
		return nil, errHistoryCoverage
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		return nil, errHistoryCoverage
	}
	var intervals []mypage.Interval
	for decoder.More() {
		if len(intervals) == maxHistoryCoverageIntervals {
			return nil, errHistoryCoverage
		}
		interval, err := decodeCoverageInterval(decoder)
		if err != nil || (len(intervals) > 0 && interval.Start.Before(intervals[len(intervals)-1].End)) {
			return nil, errHistoryCoverage
		}
		intervals = append(intervals, interval)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
		return nil, errHistoryCoverage
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errHistoryCoverage
	}
	return intervals, nil
}

func decodeCoverageInterval(decoder *json.Decoder) (mypage.Interval, error) {
	var interval mypage.Interval
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return interval, errHistoryCoverage
	}
	fields := make(map[string]time.Time, 2)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || (key != "start" && key != "end") {
			return interval, errHistoryCoverage
		}
		if _, duplicate := fields[key]; duplicate {
			return interval, errHistoryCoverage
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return interval, errHistoryCoverage
		}
		instant, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || instant.IsZero() || value != instant.UTC().Format(time.RFC3339Nano) {
			return interval, errHistoryCoverage
		}
		fields[key] = instant
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') || len(fields) != 2 {
		return interval, errHistoryCoverage
	}
	interval = mypage.Interval{Start: fields["start"], End: fields["end"]}
	if !interval.End.After(interval.Start) {
		return mypage.Interval{}, errHistoryCoverage
	}
	return interval, nil
}
