package mypage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// プロジェクトごとに匿名・上限付きのスナップショットを1つ持ち、利用者別・実行別の履歴は保存しない。
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

// スケジューラーのコールバックと独立監視は、この組み立て処理を共有する。
// 構築時には何も開始せず、認証情報やスケジューラーも作らない。
// 外部スケジューラーはRunを12時間以内の間隔で、CheckHealthを別途（例: 毎時）呼ぶ必要がある。
// これにより、削除処理の起動漏れを観測できる。
type MetadataCleanupRunner struct {
	Store               MetadataCleanupStore
	Monitor             MetadataCleanupMonitor
	Signals             MetadataCleanupHealthRecorder
	Now                 func() time.Time
	BatchSize, MaxPages int
}

func (r *MetadataCleanupRunner) Run(ctx context.Context) (MetadataCleanupObservation, error) {
	// どちらかの監視処理が欠けていれば、データの消去を拒否する。
	if r.Store == nil || r.Monitor == nil || r.Signals == nil || r.Now == nil {
		return MetadataCleanupObservation{Stage: "configuration"}, ErrMetadataCleanupIncomplete
	}
	job := MetadataCleanupJob{Store: r.Store, Recorder: r.Monitor, Now: r.Now, BatchSize: r.BatchSize, MaxPages: r.MaxPages}
	observation, runErr := job.Run(ctx)
	// キャンセルや失敗したジョブでも、匿名の失敗シグナルを送るよう試みる。
	// 監視書き込みの拒否を、古い正常スナップショットで隠さない。
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

// CheckHealthはアカウントの走査や消去を行わない。
// 出力先が受理した後だけ、保留中の保持期限違反にACKする。
// 削除処理とは独立して起動する必要があり、Runの後に呼ぶだけでは起動漏れやプロセス停止を監視できない。
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
	// 新しい正常走査で、出力先に一度も届いていない期限違反を隠さない。
	// 保存失敗や並行実行による最新観測の置換があっても、この実行自身の違反も通知する。
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
	// Firestoreは時刻をマイクロ秒精度で切り捨てる。
	// 再送入力と保存状態の比較前に正規化し、開始時刻が同じ場合も同様に扱う。
	next.StartedAt = next.StartedAt.UTC().Truncate(time.Microsecond)
	next.CompletedAt = next.CompletedAt.UTC().Truncate(time.Microsecond)
	return next
}

// 走査開始時刻で更新し、完了の遅延や観測の再送で新しい失敗を隠さない。同じ開始時刻なら失敗を優先する。
// 完全に終了した走査だけで対象時刻を進め、完了時刻をハートビートの基準にしない。
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
