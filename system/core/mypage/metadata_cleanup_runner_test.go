package mypage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cleanupMonitorFake struct {
	state                      MetadataCleanupMonitoringState
	recordErr, loadErr, ackErr error
}

func (m *cleanupMonitorFake) Record(ctx context.Context, next MetadataCleanupObservation) error {
	if m.recordErr != nil {
		return m.recordErr
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("synthetic record canceled: %w", err)
	}
	m.state = updateMetadataCleanupMonitoring(m.state, next)
	return nil
}

func (m *cleanupMonitorFake) Load(ctx context.Context) (MetadataCleanupMonitoringState, error) {
	if err := ctx.Err(); err != nil {
		return MetadataCleanupMonitoringState{}, fmt.Errorf("synthetic load canceled: %w", err)
	}
	return m.state, m.loadErr
}

func (m *cleanupMonitorFake) AcknowledgeRetention(ctx context.Context, startedAt time.Time) error {
	if m.ackErr != nil {
		return m.ackErr
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("synthetic acknowledgement canceled: %w", err)
	}
	if m.state.PendingRetentionStartedAt.Equal(startedAt) {
		m.state.PendingRetentionStartedAt = time.Time{}
	}
	return nil
}

type cleanupHealthRecorderFunc func(context.Context, MetadataCleanupHealthObservation) error

func (f cleanupHealthRecorderFunc) RecordHealth(ctx context.Context, next MetadataCleanupHealthObservation) error {
	return f(ctx, next)
}

func cleanupCompleteObservation(at time.Time) MetadataCleanupObservation {
	return MetadataCleanupObservation{StartedAt: at, CompletedAt: at.Add(time.Minute), Succeeded: true, Stage: "complete"}
}

func TestCleanupRunnerComposesJobDurableCoverageAndIndependentWatchdog(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	monitor := &cleanupMonitorFake{}
	var signals []MetadataCleanupHealthObservation
	scans := 0
	runner := MetadataCleanupRunner{
		Store: cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
			scans++
			return MetadataCleanupPage{}, nil
		}}, Monitor: monitor, Signals: cleanupHealthRecorderFunc(func(_ context.Context, o MetadataCleanupHealthObservation) error {
			signals = append(signals, o)
			return nil
		}), Now: func() time.Time { return now }, BatchSize: 10, MaxPages: 10,
	}
	signal, err := runner.CheckHealth(context.Background())
	require.NoError(t, err)
	require.Equal(t, "never_succeeded", signal.Reason)
	require.Zero(t, scans)
	o, err := runner.Run(context.Background())
	require.NoError(t, err)
	require.True(t, o.Succeeded)
	require.Equal(t, now, monitor.state.LastSuccessfulCoverage)
	require.Equal(t, "healthy", signals[1].Reason)
	before := scans
	now = now.Add(MetadataCleanupHeartbeat)
	signal, err = runner.CheckHealth(context.Background())
	require.NoError(t, err)
	require.Equal(t, "stale_heartbeat", signal.Reason)
	require.Equal(t, before, scans, "watchdog must never execute a cleanup")
	_, err = runner.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, "healthy", signals[len(signals)-1].Reason)
}

func TestCleanupRunnerFailureCannotUseOldHealthySnapshotAndSignalsAfterCancellation(t *testing.T) {
	for _, failure := range []string{"scan", "monitor-write", "monitor-read", "signal", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
			previous := cleanupCompleteObservation(now.Add(-time.Hour))
			monitor := &cleanupMonitorFake{state: MetadataCleanupMonitoringState{Latest: &previous, LastSuccessfulCoverage: previous.StartedAt}}
			dependencyErr := errors.New("synthetic-private-channel-token-data")
			if failure == "monitor-write" {
				monitor.recordErr = dependencyErr
			}
			if failure == "monitor-read" {
				monitor.loadErr = dependencyErr
			}
			var signals cleanupBufferWriter
			jsonSignals := &JSONMetadataCleanupHealthRecorder{Writer: &signals}
			runner := MetadataCleanupRunner{
				Store: cleanupStoreFake{scan: func(ctx context.Context, _ time.Time, _ *MetadataCleanupCursor, _ int) (MetadataCleanupPage, error) {
					if failure == "scan" {
						return MetadataCleanupPage{}, dependencyErr
					}
					if ctx.Err() != nil {
						return MetadataCleanupPage{}, ctx.Err()
					}
					return MetadataCleanupPage{}, nil
				}}, Monitor: monitor, Signals: cleanupHealthRecorderFunc(func(ctx context.Context, o MetadataCleanupHealthObservation) error {
					require.NoError(t, ctx.Err())
					if failure == "signal" {
						return dependencyErr
					}
					return jsonSignals.RecordHealth(ctx, o)
				}), Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 1,
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancel" {
				cancel()
			}
			o, err := runner.Run(ctx)
			require.Error(t, err)
			require.False(t, o.Succeeded)
			require.NotContains(t, signals.String(), dependencyErr.Error())
			if failure == "signal" {
				require.Equal(t, "monitoring", o.Stage)
				require.False(t, monitor.state.Latest.Succeeded)
				require.Equal(t, "monitoring", monitor.state.Latest.Stage)
				runner.Signals = jsonSignals
				signal, err := runner.CheckHealth(context.Background())
				require.NoError(t, err)
				require.True(t, signal.Alert)
				require.Equal(t, "monitoring_failed", signal.Reason)
				return
			}
			require.Contains(t, signals.String(), `"alert":true`)
			if failure == "monitor-write" || failure == "monitor-read" {
				require.Contains(t, signals.String(), "monitoring_failed")
			} else {
				require.Contains(t, signals.String(), "run_failed")
			}
			if failure == "cancel" {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.ErrorIs(t, err, dependencyErr)
			}
		})
	}
}

func TestCleanupRunnerRequiresBothMonitoringBoundariesBeforeScan(t *testing.T) {
	for _, missing := range []string{"monitor", "signals", "clock", "store"} {
		t.Run(missing, func(t *testing.T) {
			r := MetadataCleanupRunner{Store: cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
				t.Fatal("unconfigured runner scanned accounts")
				return MetadataCleanupPage{}, nil
			}}, Monitor: &cleanupMonitorFake{}, Signals: cleanupHealthRecorderFunc(func(context.Context, MetadataCleanupHealthObservation) error { return nil }), Now: time.Now, BatchSize: 1, MaxPages: 1}
			switch missing {
			case "monitor":
				r.Monitor = nil
			case "signals":
				r.Signals = nil
			case "clock":
				r.Now = nil
			case "store":
				r.Store = nil
			}
			_, err := r.Run(context.Background())
			require.ErrorIs(t, err, ErrMetadataCleanupIncomplete)
		})
	}
}

func TestCleanupMonitoringMergePreservesNewerFailureAndScanStartCoverage(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	newer := cleanupCompleteObservation(now)
	newer.Succeeded = false
	newer.Stage = "scan"
	older := cleanupCompleteObservation(now.Add(-time.Minute))
	older.CompletedAt = newer.CompletedAt.Add(time.Minute)
	state := updateMetadataCleanupMonitoring(MetadataCleanupMonitoringState{}, newer)
	state = updateMetadataCleanupMonitoring(state, older)
	require.Equal(t, newer, *state.Latest)
	require.Equal(t, older.StartedAt, state.LastSuccessfulCoverage)
	for _, firstFails := range []bool{false, true} {
		a, b := newer, cleanupCompleteObservation(now)
		if !firstFails {
			a, b = b, a
		}
		state := updateMetadataCleanupMonitoring(updateMetadataCleanupMonitoring(MetadataCleanupMonitoringState{}, a), b)
		require.False(t, state.Latest.Succeeded, "failure wins at identical coverage")
		require.True(t, validMetadataCleanupMonitoringState(state))
	}
}

func TestCleanupWatchdogRejectsMalformedStateAndUnknownPublicSignals(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	o := cleanupCompleteObservation(now.Add(-time.Hour))
	for _, mutate := range []func(*MetadataCleanupMonitoringState){
		func(s *MetadataCleanupMonitoringState) { s.Latest.Stage = "synthetic-private-id" },
		func(s *MetadataCleanupMonitoringState) { s.LastSuccessfulCoverage = now },
		func(s *MetadataCleanupMonitoringState) { s.Latest.Cleared = 1 },
		func(s *MetadataCleanupMonitoringState) { s.Latest.StartedAt = time.Time{} },
		func(s *MetadataCleanupMonitoringState) {
			s.Latest.CompletedAt = s.Latest.StartedAt.Add(MetadataCleanupBudget + time.Second)
		},
	} {
		copy := o
		state := MetadataCleanupMonitoringState{Latest: &copy, LastSuccessfulCoverage: o.StartedAt}
		mutate(&state)
		var signals cleanupBufferWriter
		r := MetadataCleanupRunner{Monitor: &cleanupMonitorFake{state: state}, Signals: &JSONMetadataCleanupHealthRecorder{Writer: &signals}, Now: func() time.Time { return now }}
		signal, err := r.CheckHealth(context.Background())
		require.ErrorIs(t, err, ErrMetadataCleanupIncomplete)
		require.True(t, signal.Alert)
		require.Equal(t, "monitoring_failed", signal.Reason)
		require.NotContains(t, signals.String(), "synthetic-private")
	}
	var output cleanupBufferWriter
	recorder := &JSONMetadataCleanupHealthRecorder{Writer: &output}
	for _, bad := range []MetadataCleanupHealthObservation{
		{CheckedAt: now, Alert: true, Reason: "synthetic-private-id"},
		{CheckedAt: now, Alert: false, Reason: "run_failed"},
		{CheckedAt: now, Alert: true, Reason: "healthy"},
		{Alert: true, Reason: "run_failed"},
	} {
		require.ErrorIs(t, recorder.RecordHealth(context.Background(), bad), ErrMetadataCleanupIncomplete)
	}
	require.Empty(t, output.String())
	require.ErrorIs(t, (&JSONMetadataCleanupHealthRecorder{Writer: &output.Buffer}).RecordHealth(context.Background(), MetadataCleanupHealthObservation{CheckedAt: now, Reason: "healthy"}), ErrMetadataCleanupIncomplete)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	require.ErrorIs(t, (&JSONMetadataCleanupHealthRecorder{Writer: &cleanupLateWriter{}}).RecordHealth(ctx, MetadataCleanupHealthObservation{CheckedAt: now, Reason: "healthy"}), context.DeadlineExceeded)
}

func TestCleanupRetentionSignalSurvivesNewerHealthyRunAndDeliveryFailure(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 123456789, time.UTC)
	violation := cleanupCompleteObservation(now.Add(-time.Hour))
	violation.Scanned, violation.Cleared, violation.RetentionViolations = 1, 1, 1
	healthy := cleanupCompleteObservation(now.Add(-time.Minute))
	for _, failure := range []string{"none", "sink", "ack"} {
		t.Run(failure, func(t *testing.T) {
			monitor := &cleanupMonitorFake{}
			require.NoError(t, monitor.Record(context.Background(), violation))
			require.NoError(t, monitor.Record(context.Background(), healthy))
			sentinel := errors.New("synthetic-private-signal-failure")
			if failure == "ack" {
				monitor.ackErr = sentinel
			}
			var output cleanupBufferWriter
			r := MetadataCleanupRunner{Monitor: monitor, Now: func() time.Time { return now }, Signals: cleanupHealthRecorderFunc(func(ctx context.Context, signal MetadataCleanupHealthObservation) error {
				require.Equal(t, "retention_deadline", signal.Reason)
				if failure == "sink" {
					return sentinel
				}
				return (&JSONMetadataCleanupHealthRecorder{Writer: &output}).RecordHealth(ctx, signal)
			})}
			signal, err := r.CheckHealth(context.Background())
			require.True(t, signal.Alert)
			require.Equal(t, "retention_deadline", signal.Reason)
			if failure == "none" {
				require.NoError(t, err)
				require.True(t, monitor.state.PendingRetentionStartedAt.IsZero())
			} else {
				require.ErrorIs(t, err, sentinel)
				require.Equal(t, violation.StartedAt.Truncate(time.Microsecond), monitor.state.PendingRetentionStartedAt)
				monitor.ackErr = nil
				r.Signals = &JSONMetadataCleanupHealthRecorder{Writer: &output}
				signal, err = r.CheckHealth(context.Background())
				require.NoError(t, err)
				require.Equal(t, "retention_deadline", signal.Reason)
				require.True(t, monitor.state.PendingRetentionStartedAt.IsZero())
			}
			r.Signals = &JSONMetadataCleanupHealthRecorder{Writer: &output}
			signal, err = r.CheckHealth(context.Background())
			require.NoError(t, err)
			require.Equal(t, "healthy", signal.Reason)
			require.NotContains(t, output.String(), sentinel.Error())
		})
	}
	// An already acknowledged/replaced durable snapshot still cannot hide the
	// current run's discovered violation when it is reported after a newer run.
	monitor := &cleanupMonitorFake{state: updateMetadataCleanupMonitoring(MetadataCleanupMonitoringState{}, healthy)}
	var output cleanupBufferWriter
	r := MetadataCleanupRunner{Monitor: monitor, Now: func() time.Time { return now }, Signals: &JSONMetadataCleanupHealthRecorder{Writer: &output}}
	signal, err := r.checkHealth(context.Background(), &violation)
	require.NoError(t, err)
	require.Equal(t, "retention_deadline", signal.Reason)
}

func TestCleanupMonitoringReplayAtFirestorePrecisionNeverReplacesSameStartFailure(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 123456789, time.UTC)
	success := cleanupCompleteObservation(now)
	failure := success
	failure.Succeeded, failure.Stage = false, "monitoring"
	state := updateMetadataCleanupMonitoring(MetadataCleanupMonitoringState{}, success)
	state = updateMetadataCleanupMonitoring(state, failure)
	state = updateMetadataCleanupMonitoring(state, success)
	require.False(t, state.Latest.Succeeded)
	require.Equal(t, "monitoring", state.Latest.Stage)
	require.Equal(t, now.Truncate(time.Microsecond), state.LastSuccessfulCoverage)
}

func TestCleanupRunnerRetriesUndeliveredRetentionAlertAfterNormalCleanup(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 123456789, time.UTC)
	monitor := &cleanupMonitorFake{}
	candidate := cleanupCandidate(now, "UCsynthetic0000000000001")
	candidate.FetchedAt = now.Add(-MetadataRetentionLimit)
	scans := 0
	var output cleanupBufferWriter
	failure := errors.New("synthetic private sink failure")
	runner := MetadataCleanupRunner{
		Store: cleanupStoreFake{scan: func(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error) {
			scans++
			if scans == 1 {
				return MetadataCleanupPage{Candidates: []MetadataCleanupCandidate{candidate}}, nil
			}
			return MetadataCleanupPage{}, nil
		}, clear: func(context.Context, MetadataCleanupCandidate, time.Time, time.Time) (string, error) {
			return "cleared", nil
		}},
		Monitor: monitor, Now: func() time.Time { return now }, BatchSize: 10, MaxPages: 10,
		Signals: cleanupHealthRecorderFunc(func(context.Context, MetadataCleanupHealthObservation) error { return failure }),
	}
	o, err := runner.Run(context.Background())
	require.ErrorIs(t, err, failure)
	require.False(t, o.Succeeded)
	require.Equal(t, 1, o.RetentionViolations)
	require.False(t, monitor.state.PendingRetentionStartedAt.IsZero())
	// Retry cleans no metadata, but must deliver the previous missed violation.
	now = now.Add(time.Minute)
	runner.Signals = &JSONMetadataCleanupHealthRecorder{Writer: &output}
	o, err = runner.Run(context.Background())
	require.NoError(t, err)
	require.True(t, o.Succeeded)
	require.Zero(t, o.RetentionViolations)
	require.Contains(t, output.String(), "retention_deadline")
	require.NotContains(t, output.String(), "healthy")
	require.True(t, monitor.state.PendingRetentionStartedAt.IsZero())
	output.Reset()
	now = now.Add(time.Minute)
	_, err = runner.Run(context.Background())
	require.NoError(t, err)
	require.Contains(t, output.String(), "healthy")
}
