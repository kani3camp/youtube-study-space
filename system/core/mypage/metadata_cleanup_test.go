package mypage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cleanupStoreFake struct {
	scan  func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error)
	clear func(context.Context, MetadataCleanupCandidate, time.Time, time.Time) (string, error)
}

func (f cleanupStoreFake) Scan(ctx context.Context, cutoff time.Time, cursor *MetadataCleanupCursor, limit int) (MetadataCleanupPage, error) {
	return f.scan(ctx, cutoff, cursor, limit)
}

func (f cleanupStoreFake) Clear(ctx context.Context, c MetadataCleanupCandidate, cutoff, now time.Time) (string, error) {
	return f.clear(ctx, c, cutoff, now)
}

type cleanupRecorderFunc func(context.Context, MetadataCleanupObservation) error

func (f cleanupRecorderFunc) Record(ctx context.Context, o MetadataCleanupObservation) error {
	return f(ctx, o)
}

func cleanupCandidate(now time.Time, uid string) MetadataCleanupCandidate {
	return MetadataCleanupCandidate{UID: uid, FetchedAt: now.Add(-MetadataCleanupAge), Revision: now.Add(-time.Second)}
}

func TestCleanupPaginatesAt29DaysAndRecordsAnonymousCompleteObservation(t *testing.T) {
	now := time.Now().UTC()
	a := cleanupCandidate(now, "UCsynthetic0000000000001")
	b := cleanupCandidate(now, "UCsynthetic0000000000002")
	scans := 0
	clears := 0
	var output cleanupBufferWriter
	store := cleanupStoreFake{scan: func(_ context.Context, cutoff time.Time, cursor *MetadataCleanupCursor, limit int) (MetadataCleanupPage, error) {
		require.Equal(t, now.Add(-MetadataCleanupAge), cutoff)
		require.Equal(t, 1, limit)
		scans++
		if scans == 1 {
			require.Nil(t, cursor)
			return MetadataCleanupPage{Candidates: []MetadataCleanupCandidate{a}, More: true}, nil
		}
		if scans == 2 {
			require.Equal(t, a.UID, cursor.UID)
			return MetadataCleanupPage{Candidates: []MetadataCleanupCandidate{b}}, nil
		}
		require.Nil(t, cursor)
		return MetadataCleanupPage{}, nil
	}, clear: func(_ context.Context, c MetadataCleanupCandidate, cutoff, at time.Time) (string, error) {
		require.False(t, c.FetchedAt.After(cutoff))
		require.Equal(t, now, at)
		clears++
		return "cleared", nil
	}}
	job := MetadataCleanupJob{Store: store, Recorder: &JSONMetadataCleanupRecorder{Writer: &output}, Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 2}
	observation, err := job.Run(context.Background())
	require.NoError(t, err)
	require.True(t, observation.Succeeded)
	require.Equal(t, 2, clears)
	require.Equal(t, 3, scans)
	require.Equal(t, 0, observation.RetentionViolations)
	for _, private := range []string{a.UID, b.UID, "revision", "fetchedAt"} {
		require.NotContains(t, output.String(), private)
	}
	require.False(t, EvaluateMetadataCleanupHealth(&observation, observation.StartedAt, now).Alert)
}

func TestCleanupFailureBacklogPageBudgetAndCancellationAreObserved(t *testing.T) {
	now := time.Now().UTC()
	candidate := cleanupCandidate(now, "UCsynthetic0000000000001")
	for _, failure := range []string{"scan", "clear", "backlog", "budget", "cancel", "malformed-page"} {
		t.Run(failure, func(t *testing.T) {
			scans := 0
			var observations []MetadataCleanupObservation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancel" {
				cancel()
			}
			store := cleanupStoreFake{scan: func(ctx context.Context, _ time.Time, _ *MetadataCleanupCursor, _ int) (MetadataCleanupPage, error) {
				scans++
				if ctx.Err() != nil {
					return MetadataCleanupPage{}, ctx.Err()
				}
				if failure == "scan" {
					return MetadataCleanupPage{}, context.DeadlineExceeded
				}
				if scans > 1 && failure != "backlog" {
					return MetadataCleanupPage{}, nil
				}
				if failure == "malformed-page" {
					return MetadataCleanupPage{More: true}, nil
				}
				return MetadataCleanupPage{Candidates: []MetadataCleanupCandidate{candidate}, More: failure == "budget"}, nil
			}, clear: func(context.Context, MetadataCleanupCandidate, time.Time, time.Time) (string, error) {
				if failure == "clear" {
					return "", errors.New("synthetic private channel and work detail")
				}
				return "changed", nil
			}}
			job := MetadataCleanupJob{Store: store, Recorder: cleanupRecorderFunc(func(ctx context.Context, o MetadataCleanupObservation) error {
				require.NoError(t, ctx.Err())
				observations = append(observations, o)
				return nil
			}), Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 1}
			o, err := job.Run(ctx)
			require.Error(t, err)
			require.False(t, o.Succeeded)
			require.Len(t, observations, 1)
			require.False(t, observations[0].Succeeded)
			require.True(t, EvaluateMetadataCleanupHealth(&o, now.Add(-time.Hour), now).Alert)
			if failure == "scan" {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			}
			if failure == "cancel" {
				require.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}

func TestCleanupMonitoringFailureAndHeartbeatNeverBecomeHealthy(t *testing.T) {
	now := time.Now().UTC()
	store := cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
		return MetadataCleanupPage{}, nil
	}, clear: func(context.Context, MetadataCleanupCandidate, time.Time, time.Time) (string, error) {
		panic("empty scan cannot clear")
	}}
	job := MetadataCleanupJob{Store: store, Recorder: cleanupRecorderFunc(func(context.Context, MetadataCleanupObservation) error { return context.DeadlineExceeded }), Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 1}
	observation, err := job.Run(context.Background())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, observation.Succeeded)
	require.Equal(t, "monitoring", observation.Stage)
	healthy := MetadataCleanupObservation{StartedAt: now, CompletedAt: now, Succeeded: true, Stage: "complete"}
	require.Equal(t, "never_succeeded", EvaluateMetadataCleanupHealth(nil, time.Time{}, now).Reason)
	require.Equal(t, "stale_heartbeat", EvaluateMetadataCleanupHealth(&healthy, now.Add(-MetadataCleanupHeartbeat), now).Reason)
	healthy.RetentionViolations = 1
	require.Equal(t, "retention_deadline", EvaluateMetadataCleanupHealth(&healthy, now, now).Reason)
	require.Equal(t, "invalid_clock", EvaluateMetadataCleanupHealth(&healthy, now.Add(time.Second), now).Reason)
	var output cleanupBufferWriter
	bad := healthy
	bad.Stage = "synthetic-private-id"
	require.Error(t, (&JSONMetadataCleanupRecorder{Writer: &output}).Record(context.Background(), bad))
	require.Empty(t, output.String())
}

func TestCleanupRejectsFutureEligibleDataAndRecordsRetentionViolation(t *testing.T) {
	now := time.Now().UTC()
	candidate := cleanupCandidate(now, "UCsynthetic0000000000001")
	for _, age := range []time.Duration{MetadataCleanupAge - time.Second, MetadataRetentionLimit} {
		candidate.FetchedAt = now.Add(-age)
		scans := 0
		store := cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
			scans++
			if scans == 1 {
				return MetadataCleanupPage{Candidates: []MetadataCleanupCandidate{candidate}}, nil
			}
			return MetadataCleanupPage{}, nil
		}, clear: func(context.Context, MetadataCleanupCandidate, time.Time, time.Time) (string, error) {
			return "cleared", nil
		}}
		var output cleanupBufferWriter
		job := MetadataCleanupJob{Store: store, Recorder: &JSONMetadataCleanupRecorder{Writer: &output}, Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 1}
		o, err := job.Run(context.Background())
		if age < MetadataCleanupAge {
			require.Error(t, err)
			require.Zero(t, o.Cleared)
		} else {
			require.NoError(t, err)
			require.Equal(t, 1, o.RetentionViolations)
			require.True(t, EvaluateMetadataCleanupHealth(&o, now, now).Alert)
		}
		require.False(t, strings.Contains(output.String(), candidate.UID))
	}
}

func TestCleanupTimeBudgetAndInvalidConfigurationNeverRecordSuccess(t *testing.T) {
	now := time.Now().UTC()
	scans := 0
	clockCalls := 0
	var recorded MetadataCleanupObservation
	store := cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
		scans++
		return MetadataCleanupPage{}, nil
	}, clear: func(context.Context, MetadataCleanupCandidate, time.Time, time.Time) (string, error) {
		panic("empty scan")
	}}
	recorder := cleanupRecorderFunc(func(_ context.Context, o MetadataCleanupObservation) error { recorded = o; return nil })
	job := MetadataCleanupJob{Store: store, Recorder: recorder, Now: func() time.Time {
		clockCalls++
		if clockCalls == 1 {
			return now
		}
		return now.Add(MetadataCleanupBudget + time.Second)
	}, BatchSize: 1, MaxPages: 1}
	o, err := job.Run(context.Background())
	require.ErrorIs(t, err, ErrMetadataCleanupIncomplete)
	require.False(t, o.Succeeded)
	require.False(t, recorded.Succeeded)
	require.Equal(t, "budget", o.Stage)
	scans = 0
	job.Now = func() time.Time { return now }
	job.BatchSize = 0
	o, err = job.Run(context.Background())
	require.Error(t, err)
	require.Equal(t, "configuration", o.Stage)
	require.Zero(t, scans)
	job.BatchSize = 1
	job.Recorder = nil
	_, err = job.Run(context.Background())
	require.Error(t, err)
	require.Zero(t, scans)
}

// This nonblocking in-memory sink is only a test fixture. Production sinks
// must actually honor SetWriteDeadline while Write is in progress.
type cleanupBufferWriter struct{ bytes.Buffer }

func (*cleanupBufferWriter) SetWriteDeadline(time.Time) error { return nil }

func TestCleanupRecorderRejectsUnboundedWriterAndLateSuccessfulWrite(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() { require.NoError(t, reader.Close()); require.NoError(t, writer.Close()) })
	observation := MetadataCleanupObservation{Stage: "complete", Succeeded: true}
	require.ErrorIs(t, (&JSONMetadataCleanupRecorder{Writer: writer}).Record(context.Background(), observation), ErrMetadataCleanupIncomplete)
	late := &cleanupLateWriter{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, (&JSONMetadataCleanupRecorder{Writer: late}).Record(ctx, observation), context.DeadlineExceeded)
}

type cleanupLateWriter struct{}

func (*cleanupLateWriter) SetWriteDeadline(time.Time) error { return nil }
func (*cleanupLateWriter) Write(p []byte) (int, error) {
	time.Sleep(20 * time.Millisecond)
	return len(p), nil
}

func TestCleanupRecorderDeadlineAndCancellationInterruptBlockedPipe(t *testing.T) {
	for _, cancelEarly := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelEarly), func(t *testing.T) {
			reader, writer, err := os.Pipe()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reader.Close()); require.NoError(t, writer.Close()) })
			require.NoError(t, writer.SetWriteDeadline(time.Now().Add(20*time.Millisecond)))
			// Fill the pipe without a reader until the kernel write blocks.
			_, err = writer.Write(make([]byte, 1<<20))
			require.ErrorIs(t, err, os.ErrDeadlineExceeded)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			entered := make(chan struct{})
			sink := &cleanupSignaledPipeWriter{File: writer, entered: entered}
			go func() {
				done <- (&JSONMetadataCleanupRecorder{Writer: sink}).Record(ctx, MetadataCleanupObservation{Stage: "complete", Succeeded: true})
			}()
			if cancelEarly {
				<-entered
				cancel()
			}
			select {
			case err := <-done:
				require.Error(t, err)
				if cancelEarly {
					require.ErrorIs(t, err, context.Canceled)
				}
			case <-time.After(time.Second):
				t.Fatal("blocked telemetry ignored its deadline/cancellation")
			}
		})
	}
}

type cleanupSignaledPipeWriter struct {
	*os.File
	entered chan struct{}
}

func (w *cleanupSignaledPipeWriter) Write(p []byte) (int, error) {
	close(w.entered)
	n, err := w.File.Write(p)
	if err != nil {
		return n, fmt.Errorf("write synthetic pipe: %w", err)
	}
	return n, nil
}

func TestCleanupRunReturnsMonitoringFailureWithinBudgetOnBlockedSink(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()); require.NoError(t, writer.Close()) })
	require.NoError(t, writer.SetWriteDeadline(time.Now().Add(20*time.Millisecond)))
	_, err = writer.Write(make([]byte, 1<<20))
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	store := cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
		return MetadataCleanupPage{}, nil
	}}
	job := MetadataCleanupJob{Store: store, Recorder: &JSONMetadataCleanupRecorder{Writer: writer}, Now: time.Now, BatchSize: 1, MaxPages: 1}
	type result struct {
		observation MetadataCleanupObservation
		err         error
	}
	done := make(chan result, 1)
	go func() {
		observation, err := job.Run(context.Background())
		done <- result{observation, err}
	}()
	select {
	case outcome := <-done:
		require.Error(t, outcome.err)
		require.False(t, outcome.observation.Succeeded)
		require.Equal(t, "monitoring", outcome.observation.Stage)
	case <-time.After(3 * time.Second):
		t.Fatal("Run remained blocked past its telemetry budget")
	}
}
