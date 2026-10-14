package mypage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// One bounded, anonymous snapshot per project; no per-user or per-run archive.
type MetadataCleanupMonitoringState struct {
	Latest                    *MetadataCleanupObservation `firestore:"latest"`
	LastSuccessfulCoverage    time.Time                   `firestore:"lastSuccessfulCoverage"`
	PendingRetentionStartedAt time.Time                   `firestore:"pendingRetentionStartedAt"`
}

type MetadataCleanupMonitor interface {
	MetadataCleanupRecorder
	Load(context.Context) (MetadataCleanupMonitoringState, error)
	AcknowledgeRetention(context.Context, time.Time) error
}

type MetadataCleanupHealthObservation struct {
	CheckedAt time.Time `json:"checkedAt"`
	Alert     bool      `json:"alert"`
	Reason    string    `json:"reason"`
}

type MetadataCleanupHealthRecorder interface {
	RecordHealth(context.Context, MetadataCleanupHealthObservation) error
}

// Scheduler callbacks and an independently scheduled watchdog share this
// assembly. Construction starts nothing and creates no credentials or scheduler.
// An external scheduler must invoke Run on a <=12h cadence and CheckHealth
// independently (e.g. hourly), so a missing cleanup trigger is observable.
type MetadataCleanupRunner struct {
	Store               MetadataCleanupStore
	Monitor             MetadataCleanupMonitor
	Signals             MetadataCleanupHealthRecorder
	Now                 func() time.Time
	BatchSize, MaxPages int
}

func (r *MetadataCleanupRunner) Run(ctx context.Context) (MetadataCleanupObservation, error) {
	// Refuse to clear any data if either monitoring boundary is absent.
	if r.Store == nil || r.Monitor == nil || r.Signals == nil || r.Now == nil {
		return MetadataCleanupObservation{Stage: "configuration"}, ErrMetadataCleanupIncomplete
	}
	job := MetadataCleanupJob{Store: r.Store, Recorder: r.Monitor, Now: r.Now, BatchSize: r.BatchSize, MaxPages: r.MaxPages}
	observation, runErr := job.Run(ctx)
	// Even a canceled/failed job must attempt its anonymous failure signal.
	// A rejected monitoring write cannot be hidden by an older healthy snapshot.
	_, healthErr := r.checkHealth(context.WithoutCancel(ctx), &observation)
	if healthErr != nil {
		observation.Succeeded = false
		observation.Stage = "monitoring"
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		recordErr := r.Monitor.Record(recordCtx, observation)
		cancel()
		if recordErr != nil {
			healthErr = errors.Join(healthErr, fmt.Errorf("record cleanup monitoring failure: %w", recordErr))
		}
	}
	return observation, errors.Join(runErr, healthErr)
}

// CheckHealth never scans or clears accounts. It acknowledges a pending
// retention signal only after the sink accepts it. The watchdog
// must be triggered independently of cleanup; merely calling it after Run does
// not monitor missed scheduler invocations or a stopped cleanup process.
func (r *MetadataCleanupRunner) CheckHealth(ctx context.Context) (MetadataCleanupHealthObservation, error) {
	return r.checkHealth(ctx, nil)
}

func (r *MetadataCleanupRunner) checkHealth(ctx context.Context, current *MetadataCleanupObservation) (MetadataCleanupHealthObservation, error) {
	if r.Monitor == nil || r.Signals == nil || r.Now == nil {
		return MetadataCleanupHealthObservation{}, ErrMetadataCleanupIncomplete
	}
	loadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	state, loadErr := r.Monitor.Load(loadCtx)
	loadErr = errors.Join(loadErr, loadCtx.Err())
	cancel()
	now := r.Now().UTC()
	health := MetadataCleanupHealth{Alert: true, Reason: "monitoring_failed"}
	if loadErr == nil {
		if !validMetadataCleanupMonitoringState(state) {
			loadErr = ErrMetadataCleanupIncomplete
		} else {
			health = EvaluateMetadataCleanupHealth(state.Latest, state.LastSuccessfulCoverage, now)
			if health.Reason == "run_failed" && state.Latest.Stage == "monitoring" {
				health.Reason = "monitoring_failed"
			}
		}
	}
	if current != nil && !current.Succeeded {
		health = MetadataCleanupHealth{Alert: true, Reason: "run_failed"}
		if current.Stage == "monitoring" || loadErr != nil {
			health.Reason = "monitoring_failed"
		}
	}
	// A newer successful scan cannot hide a deadline violation that has never
	// reached the signal sink. Also report this run's violation if persistence
	// failed or a concurrent run already replaced its latest observation.
	if health.Reason != "invalid_clock" && ((loadErr == nil && !state.PendingRetentionStartedAt.IsZero()) || (current != nil && current.RetentionViolations > 0)) {
		health = MetadataCleanupHealth{Alert: true, Reason: "retention_deadline"}
	}
	signal := MetadataCleanupHealthObservation{CheckedAt: now, Alert: health.Alert, Reason: health.Reason}
	signalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	signalErr := errors.Join(r.Signals.RecordHealth(signalCtx, signal), signalCtx.Err())
	if signalErr != nil {
		return signal, errors.Join(loadErr, fmt.Errorf("record cleanup health: %w", signalErr))
	}
	if loadErr != nil {
		return signal, fmt.Errorf("load cleanup monitoring: %w", loadErr)
	}
	if signal.Reason == "retention_deadline" && !state.PendingRetentionStartedAt.IsZero() {
		ackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		ackErr := errors.Join(r.Monitor.AcknowledgeRetention(ackCtx, state.PendingRetentionStartedAt), ackCtx.Err())
		cancel()
		if ackErr != nil {
			return signal, fmt.Errorf("acknowledge cleanup retention signal: %w", ackErr)
		}
	}
	return signal, nil
}

func validMetadataCleanupMonitoringObservation(o MetadataCleanupObservation) bool {
	if o.StartedAt.IsZero() || o.CompletedAt.IsZero() || o.CompletedAt.Before(o.StartedAt) ||
		o.Scanned < 0 || o.Cleared < 0 || o.Changed < 0 || o.Missing < 0 || o.Failed < 0 || o.RetentionViolations < 0 ||
		o.RetentionViolations > o.Cleared || o.Scanned != o.Cleared+o.Changed+o.Missing+o.Failed {
		return false
	}
	switch o.Stage {
	case "configuration", "scan", "clear", "budget", "backlog", "complete", "monitoring":
	default:
		return false
	}
	return !o.Succeeded || (o.Stage == "complete" && o.Failed == 0 && o.CompletedAt.Sub(o.StartedAt) <= MetadataCleanupBudget)
}

func validMetadataCleanupMonitoringState(state MetadataCleanupMonitoringState) bool {
	if state.Latest == nil {
		return state.LastSuccessfulCoverage.IsZero() && state.PendingRetentionStartedAt.IsZero()
	}
	return validMetadataCleanupMonitoringObservation(*state.Latest) &&
		!state.LastSuccessfulCoverage.After(state.Latest.StartedAt) &&
		!state.PendingRetentionStartedAt.After(state.Latest.StartedAt) &&
		(!state.Latest.Succeeded || state.LastSuccessfulCoverage.Equal(state.Latest.StartedAt))
}

func canonicalMetadataCleanupObservation(next MetadataCleanupObservation) MetadataCleanupObservation {
	// Firestore rounds timestamp precision down to microseconds. Normalize before
	// comparing replayed input with persisted state, including equal-start ties.
	next.StartedAt = next.StartedAt.UTC().Truncate(time.Microsecond)
	next.CompletedAt = next.CompletedAt.UTC().Truncate(time.Microsecond)
	return next
}

// Update by scan start, so delayed completion/replayed observations never mask a
// newer failure. At equal start times a failure wins. Coverage only moves forward
// from a fully completed scan; completion time is never the heartbeat origin.
func updateMetadataCleanupMonitoring(state MetadataCleanupMonitoringState, next MetadataCleanupObservation) MetadataCleanupMonitoringState {
	next = canonicalMetadataCleanupObservation(next)
	if next.RetentionViolations > 0 && next.StartedAt.After(state.PendingRetentionStartedAt) {
		state.PendingRetentionStartedAt = next.StartedAt
	}
	if state.Latest == nil || next.StartedAt.After(state.Latest.StartedAt) ||
		(next.StartedAt.Equal(state.Latest.StartedAt) && (!next.Succeeded || state.Latest.Succeeded)) {
		state.Latest = &next
	}
	if next.Succeeded && next.StartedAt.After(state.LastSuccessfulCoverage) {
		state.LastSuccessfulCoverage = next.StartedAt
	}
	return state
}

type JSONMetadataCleanupHealthRecorder struct{ Writer io.Writer }

func (r *JSONMetadataCleanupHealthRecorder) RecordHealth(ctx context.Context, value MetadataCleanupHealthObservation) error {
	if value.CheckedAt.IsZero() {
		return ErrMetadataCleanupIncomplete
	}
	switch value.Reason {
	case "healthy":
		if value.Alert {
			return ErrMetadataCleanupIncomplete
		}
	case "invalid_clock", "never_succeeded", "run_failed", "retention_deadline", "stale_heartbeat", "incomplete_observation", "monitoring_failed":
		if !value.Alert {
			return ErrMetadataCleanupIncomplete
		}
	default:
		return ErrMetadataCleanupIncomplete
	}
	return writeMetadataCleanupJSON(ctx, r.Writer, value)
}
