package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"app.modules/core/mypage"
	"app.modules/core/repository"

	"github.com/stretchr/testify/require"
)

func coverageTime(t *testing.T, value string) time.Time {
	t.Helper()
	instant, err := time.Parse(time.RFC3339Nano, value)
	require.NoError(t, err)
	return instant
}

func coverageJSON(start, end string) string {
	return fmt.Sprintf(`{"start":%q,"end":%q}`, start, end)
}

func coverageSequence(count int) string {
	parts := make([]string, count)
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for i := range parts {
		parts[i] = coverageJSON(base.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano), base.Add(time.Duration(i+1)*time.Second).Format(time.RFC3339Nano))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func TestHistoryCoveragePreservesExplicitIntervalsAndUnknown(t *testing.T) {
	for _, value := range []string{"", "[]", " [ ]\n"} {
		intervals, err := parseHistoryCoverage(value)
		require.NoError(t, err)
		require.Nil(t, intervals, "empty input must not invent any history")
	}
	start, middle := "2026-09-29T15:00:00.123456789Z", "2026-10-01T00:00:00Z"
	gapEnd, end := "2026-10-02T00:00:00Z", "2026-10-06T03:00:00Z"
	got, err := parseHistoryCoverage("[" + coverageJSON(start, middle) + "," + coverageJSON(gapEnd, end) + "]")
	require.NoError(t, err)
	require.Equal(t, []mypage.Interval{
		{Start: coverageTime(t, start), End: coverageTime(t, middle)},
		{Start: coverageTime(t, gapEnd), End: coverageTime(t, end)},
	}, got, "preserve nanoseconds and gaps exactly")
	value := coverageSequence(maxHistoryCoverageIntervals)
	got, err = parseHistoryCoverage(value + strings.Repeat(" ", maxHistoryCoverageBytes-len(value)))
	require.NoError(t, err)
	require.Len(t, got, maxHistoryCoverageIntervals, "limit is inclusive and adjacent intervals are valid")
}

func TestInvalidHistoryCoverageStopsOfflineAndStartupBeforeBootstrap(t *testing.T) {
	valid := coverageJSON("2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z")
	for name, value := range map[string]string{
		"whitespace": " ", "null": "null", "object": "{}", "scalar": `"synthetic-private"`,
		"unfinished array": "[", "unfinished object": "[{", "trailing comma": "[" + valid + ",]",
		"trailing JSON": "[" + valid + "] []", "trailing text": "[" + valid + "] synthetic-private",
		"null interval": "[null]", "missing start": `[{"end":"2026-10-02T00:00:00Z"}]`,
		"missing end":      `[{"start":"2026-10-01T00:00:00Z"}]`,
		"unknown field":    "[" + strings.TrimSuffix(valid, "}") + `,"evidence":"synthetic-private"}]`,
		"duplicate field":  "[" + strings.TrimSuffix(valid, "}") + `,"start":"2026-09-01T00:00:00Z"}]`,
		"wrong field case": `[{"Start":"2026-10-01T00:00:00Z","end":"2026-10-02T00:00:00Z"}]`,
		"wrong type":       `[{"start":1,"end":"2026-10-02T00:00:00Z"}]`,
		"null time":        `[{"start":null,"end":"2026-10-02T00:00:00Z"}]`,
		"invalid time":     "[" + coverageJSON("synthetic-private", "2026-10-02T00:00:00Z") + "]",
		"zero time":        "[" + coverageJSON("0001-01-01T00:00:00Z", "2026-10-02T00:00:00Z") + "]",
		"local time":       "[" + coverageJSON("2026-10-01T00:00:00", "2026-10-02T00:00:00Z") + "]",
		"offset time":      "[" + coverageJSON("2026-10-01T09:00:00+09:00", "2026-10-02T00:00:00Z") + "]",
		"excess precision": "[" + coverageJSON("2026-10-01T00:00:00.1234567891Z", "2026-10-02T00:00:00Z") + "]",
		"empty range":      "[" + coverageJSON("2026-10-01T00:00:00Z", "2026-10-01T00:00:00Z") + "]",
		"reversed range":   "[" + coverageJSON("2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z") + "]",
		"overlap":          "[" + valid + "," + valid + "]",
		"out of order":     "[" + valid + "," + coverageJSON("2026-09-01T00:00:00Z", "2026-09-02T00:00:00Z") + "]",
		"oversized":        "[" + valid + "]" + strings.Repeat(" ", maxHistoryCoverageBytes),
		"too many":         coverageSequence(maxHistoryCoverageIntervals + 1),
	} {
		t.Run(name, func(t *testing.T) {
			values := syntheticConfiguration()
			values["MYPAGE_HISTORY_COVERAGE"] = value
			get := func(key string) string { return values[key] }
			var events []string
			err := runWith(context.Background(), get, syntheticBootstrap(t, "", &events), func(context.Context, *http.Server) error {
				t.Fatal("invalid coverage reached HTTP startup")
				return nil
			})
			require.EqualError(t, err, "MyPage configuration unavailable")
			require.Empty(t, events, "reject before discovering ADC or constructing SDKs")
			values["MYPAGE_INFRASTRUCTURE_READY"] = ""
			var stdout, stderr bytes.Buffer
			code := command([]string{"--check-config"}, get, &stdout, &stderr, func() error {
				t.Fatal("offline check reached bootstrap")
				return nil
			})
			require.Equal(t, 1, code)
			require.Empty(t, stderr.String())
			require.JSONEq(t, expectedReport("invalid", "pending"), stdout.String())
			require.NotContains(t, stdout.String(), "synthetic-private")
			require.NotContains(t, stdout.String(), "2026-")
		})
	}
}

func TestStartupTransfersCoverageToReaderAndPreservesMetricCompleteness(t *testing.T) {
	transport := http.DefaultTransport
	http.DefaultTransport = rejectingTransport{t: t}
	t.Cleanup(func() { http.DefaultTransport = transport })
	asOf := coverageTime(t, "2026-10-06T03:00:00Z")
	span := func(start, end string) mypage.Interval {
		return mypage.Interval{Start: coverageTime(t, start), End: coverageTime(t, end)}
	}
	for _, tc := range []struct {
		name         string
		coverage     []mypage.Interval
		today, week  bool
		completeDays []bool
	}{
		{"unset", nil, false, false, []bool{false, false, false, false, false, false, false}},
		{"complete", []mypage.Interval{span("2026-09-29T15:00:00Z", "2026-10-06T03:00:00Z")}, true, true, []bool{true, true, true, true, true, true, true}},
		{"today only", []mypage.Interval{span("2026-10-05T15:00:00Z", "2026-10-06T03:00:00Z")}, true, false, []bool{false, false, false, false, false, false, true}},
		{"gap on Monday", []mypage.Interval{span("2026-09-29T15:00:00Z", "2026-10-04T16:00:00Z"), span("2026-10-04T17:00:00Z", "2026-10-06T03:00:00Z")}, true, false, []bool{true, true, true, true, true, false, true}},
		{"expired end", []mypage.Interval{span("2026-09-29T15:00:00Z", "2026-10-06T02:00:00Z")}, false, false, []bool{true, true, true, true, true, true, false}},
		{"adjacent", []mypage.Interval{span("2026-09-29T15:00:00Z", "2026-10-04T15:00:00Z"), span("2026-10-04T15:00:00Z", "2026-10-06T03:00:00Z")}, true, true, []bool{true, true, true, true, true, true, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := syntheticConfiguration()
			if tc.coverage != nil {
				parts := make([]string, len(tc.coverage))
				for i, interval := range tc.coverage {
					parts[i] = coverageJSON(interval.Start.Format(time.RFC3339Nano), interval.End.Format(time.RFC3339Nano))
				}
				values["MYPAGE_HISTORY_COVERAGE"] = "[" + strings.Join(parts, ",") + "]"
			}
			get := func(key string) string { return values[key] }
			var stdout, stderr bytes.Buffer
			require.Zero(t, command([]string{"--check-config"}, get, &stdout, &stderr, func() error {
				t.Fatal("offline check reached bootstrap")
				return nil
			}))
			require.Empty(t, stderr.String())
			require.JSONEq(t, expectedReport("valid", "declared"), stdout.String())
			var events []string
			err := runWith(context.Background(), get, syntheticBootstrap(t, "", &events), func(_ context.Context, server *http.Server) error {
				handler, ok := server.Handler.(*mypage.HTTPHandler)
				require.True(t, ok)
				require.Nil(t, handler.Intake)
				reader, ok := handler.BFF.Reader.(*mypage.FirestoreSnapshotReader)
				require.True(t, ok)
				require.Equal(t, tc.coverage, reader.Coverage, "normal startup must transfer exact verified intervals")
				// Bootstrap is mocked: the zero Firestore client is never read.
				response, err := mypage.Aggregate(mypage.WorkSnapshot{
					AsOf: asOf, UID: "synthetic", User: &repository.UserDoc{TotalStudySec: 6000}, Coverage: reader.Coverage,
					Segments: []repository.WorkSegmentDoc{{UserID: "synthetic", SegmentType: repository.WorkState, StartedAt: asOf.Add(-3 * time.Hour), EndedAt: asOf.Add(-2 * time.Hour), DurationSec: 3600}},
				}, mypage.Section[mypage.Account]{Availability: mypage.Available, Data: &mypage.Account{DisplayName: "Sample"}})
				require.NoError(t, err)
				checkCoverageMetric(t, response.Summary.Data.Today, tc.today, 3600)
				checkCoverageMetric(t, response.Summary.Data.Week, tc.week, 3600)
				checkCoverageMetric(t, response.Summary.Data.Lifetime, true, 6000)
				require.Len(t, response.Recent7Days.Data, 7)
				for i, complete := range tc.completeDays {
					var seconds int64
					if i == 6 {
						seconds = 3600
					}
					checkCoverageMetric(t, response.Recent7Days.Data[i].Metric, complete, seconds)
				}
				require.Equal(t, asOf, response.GeneratedAt)
				require.Equal(t, mypage.Available, response.Current.Availability)
				return nil
			})
			require.NoError(t, err)
			require.Equal(t, []string{"credentials", "firebase", "database", "database closed"}, events)
		})
	}
}

func checkCoverageMetric(t *testing.T, metric mypage.Metric, complete bool, seconds int64) {
	t.Helper()
	if complete {
		require.Equal(t, mypage.Available, metric.Availability)
		require.Nil(t, metric.ReasonCode)
		require.NotNil(t, metric.WorkSec)
		require.Equal(t, seconds, *metric.WorkSec)
	} else {
		require.Equal(t, mypage.Unavailable, metric.Availability)
		require.NotNil(t, metric.ReasonCode)
		require.Equal(t, mypage.HistoryIncomplete, *metric.ReasonCode)
		require.Nil(t, metric.WorkSec, "unverified history must never become zero")
	}
}
