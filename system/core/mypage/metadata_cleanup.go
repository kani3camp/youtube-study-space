package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	MetadataCleanupAge     = 29 * 24 * time.Hour
	MetadataRetentionLimit = 30 * 24 * time.Hour
	MetadataCleanupBudget  = 10 * time.Minute
	// 29 日経過して対象になった直後に走査を逃した場合でも、30 日の上限までに完了できる余裕を残す。
	MetadataCleanupHeartbeat = 23 * time.Hour
)

var ErrMetadataCleanupIncomplete = errors.New("metadata cleanup incomplete")

type MetadataCleanupCandidate struct {
	UID                 string
	FetchedAt, Revision time.Time
}
type MetadataCleanupCursor struct {
	UID       string
	FetchedAt time.Time
}
type MetadataCleanupPage struct {
	Candidates []MetadataCleanupCandidate
	More       bool
}
type MetadataCleanupStore interface {
	Scan(context.Context, time.Time, *MetadataCleanupCursor, int) (MetadataCleanupPage, error)
	Clear(context.Context, MetadataCleanupCandidate, time.Time, time.Time) (string, error)
}

// 観測には匿名の件数と時刻だけを含め、IDや元のエラーは含めない。
type MetadataCleanupObservation struct {
	StartedAt           time.Time `json:"startedAt"`
	CompletedAt         time.Time `json:"completedAt"`
	Succeeded           bool      `json:"succeeded"`
	Stage               string    `json:"stage"`
	Scanned             int       `json:"scanned"`
	Cleared             int       `json:"cleared"`
	Changed             int       `json:"changed"`
	Missing             int       `json:"missing"`
	Failed              int       `json:"failed"`
	RetentionViolations int       `json:"retentionViolations"`
}
type MetadataCleanupRecorder interface {
	Record(context.Context, MetadataCleanupObservation) error
}
type MetadataCleanupJob struct {
	Store               MetadataCleanupStore
	Recorder            MetadataCleanupRecorder
	Now                 func() time.Time
	BatchSize, MaxPages int
}

func cursorAfter(next, previous MetadataCleanupCursor) bool {
	return next.FetchedAt.After(previous.FetchedAt) || (next.FetchedAt.Equal(previous.FetchedAt) && next.UID > previous.UID)
}

// Run は外部から呼び出せるジョブ本体であり、スケジューラーや実環境自体の初期化は行わない。
// 上限付きの最終走査でリビジョン競合や残件を検出し、途中までしか終わらなかった処理を完了とみなさない。
// また、未利用ユーザーのメタデータを再取得することもしない。
func (j *MetadataCleanupJob) Run(ctx context.Context) (MetadataCleanupObservation, error) {
	observation := MetadataCleanupObservation{Stage: "configuration"}
	if j.Now == nil || j.Recorder == nil {
		return observation, ErrMetadataCleanupIncomplete
	}
	observation.StartedAt = j.Now().UTC()
	operation := func() error {
		if j.Store == nil || observation.StartedAt.IsZero() || j.BatchSize < 1 || j.BatchSize > 1000 || j.MaxPages < 1 || j.MaxPages > 1000 {
			return ErrMetadataCleanupIncomplete
		}
		cutoff := observation.StartedAt.Add(-MetadataCleanupAge)
		work, cancel := context.WithTimeout(ctx, MetadataCleanupBudget)
		defer cancel()
		var cursor *MetadataCleanupCursor
		var failures error
		complete := false
		for range j.MaxPages {
			observation.Stage = "scan"
			page, err := j.Store.Scan(work, cutoff, cursor, j.BatchSize)
			if err != nil {
				return fmt.Errorf("scan cleanup candidates: %w", err)
			}
			if len(page.Candidates) > j.BatchSize || (page.More && len(page.Candidates) == 0) {
				return ErrMetadataCleanupIncomplete
			}
			for _, candidate := range page.Candidates {
				next := MetadataCleanupCursor{UID: candidate.UID, FetchedAt: candidate.FetchedAt}
				if !youtubeChannelID.MatchString(candidate.UID) || candidate.FetchedAt.IsZero() || candidate.Revision.IsZero() || candidate.FetchedAt.After(cutoff) || (cursor != nil && !cursorAfter(next, *cursor)) {
					return ErrMetadataCleanupIncomplete
				}
				cursor = &next
				observation.Scanned++
				observation.Stage = "clear"
				outcome, err := j.Store.Clear(work, candidate, cutoff, j.Now().UTC())
				if err != nil {
					observation.Failed++
					failures = errors.Join(failures, err)
				} else {
					switch outcome {
					case "cleared":
						observation.Cleared++
						if j.Now().Sub(candidate.FetchedAt) >= MetadataRetentionLimit {
							observation.RetentionViolations++
						}
					case "changed":
						observation.Changed++
					case "missing":
						observation.Missing++
					default:
						observation.Failed++
						failures = errors.Join(failures, ErrMetadataCleanupIncomplete)
					}
				}
				if work.Err() != nil {
					return work.Err()
				}
			}
			if !page.More {
				complete = true
				break
			}
		}
		if !complete {
			observation.Stage = "budget"
			return errors.Join(ErrMetadataCleanupIncomplete, failures)
		}
		observation.Stage = "backlog"
		remaining, err := j.Store.Scan(work, cutoff, nil, 1)
		if err != nil {
			return fmt.Errorf("verify cleanup backlog: %w", err)
		}
		if len(remaining.Candidates) != 0 || remaining.More || failures != nil {
			return errors.Join(ErrMetadataCleanupIncomplete, failures)
		}
		observation.Stage = "complete"
		observation.Succeeded = true
		return nil
	}
	err := operation()
	observation.CompletedAt = j.Now().UTC()
	if observation.CompletedAt.Before(observation.StartedAt) || observation.CompletedAt.Sub(observation.StartedAt) > MetadataCleanupBudget {
		observation.Succeeded = false
		observation.Stage = "budget"
		err = errors.Join(err, ErrMetadataCleanupIncomplete)
	}
	// キャンセル時も、監視出力が動けば失敗観測を記録する。
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if recordErr := j.Recorder.Record(recordCtx, observation); recordErr != nil {
		observation.Succeeded = false
		observation.Stage = "monitoring"
		return observation, errors.Join(err, fmt.Errorf("record cleanup outcome: %w", recordErr))
	}
	return observation, err
}

type JSONMetadataCleanupRecorder struct{ Writer io.Writer }

// 出力先には書き込み期限を設定できる専用ライターのみを許可する（例: パイプやソケットの *os.File）。
// 通常の io.Writer は書き込み前に拒否する（待機中の書き込みを無期限 goroutine へ逃がしても安全にキャンセルできないため）。
type cleanupDeadlineWriter interface {
	io.Writer
	SetWriteDeadline(time.Time) error
}

func (r *JSONMetadataCleanupRecorder) Record(ctx context.Context, value MetadataCleanupObservation) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup observation canceled: %w", err)
	}
	if r.Writer == nil || value.Scanned < 0 || value.Cleared < 0 || value.Changed < 0 || value.Missing < 0 || value.Failed < 0 || value.RetentionViolations < 0 {
		return ErrMetadataCleanupIncomplete
	}
	switch value.Stage {
	case "configuration", "scan", "clear", "budget", "backlog", "complete":
	default:
		return ErrMetadataCleanupIncomplete
	}
	return writeMetadataCleanupJSON(ctx, r.Writer, value)
}

// 実行記録と健全性記録は、同じ上限付き・期限対応の出力先を使う。
func writeMetadataCleanupJSON(ctx context.Context, writer io.Writer, value any) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup observation canceled: %w", err)
	}
	sink, ok := writer.(cleanupDeadlineWriter)
	if !ok {
		return ErrMetadataCleanupIncomplete
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	deadline, _ := writeCtx.Deadline()
	if err := sink.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set cleanup observation deadline: %w", err)
	}
	stopped := make(chan struct{})
	var cancelDeadlineErr error
	stop := context.AfterFunc(writeCtx, func() {
		cancelDeadlineErr = sink.SetWriteDeadline(time.Now())
		close(stopped)
	})
	// 許可済みのこの構造体だけをシリアライズし、依存先エラーは非公開にする。
	err := json.NewEncoder(sink).Encode(value)
	if !stop() {
		<-stopped
		err = errors.Join(err, cancelDeadlineErr)
	}
	if writeCtx.Err() != nil {
		return fmt.Errorf("cleanup observation canceled: %w", writeCtx.Err())
	}
	if !time.Now().Before(deadline) {
		return fmt.Errorf("cleanup observation deadline: %w", context.DeadlineExceeded)
	}
	if err != nil {
		return fmt.Errorf("write cleanup observation: %w", err)
	}
	return nil
}

type MetadataCleanupHealth struct {
	Alert  bool
	Reason string
}

// lastSuccessfulCoverage は成功した走査の開始時刻（StartedAt）であり、完了時刻ではない。
// 運用時は 1 日 2 回など、23 時間以内に走査対象を更新する必要がある。
// ハートビート欠如・実行失敗・期限違反を暗黙のうちに正常扱いしない。
func EvaluateMetadataCleanupHealth(latest *MetadataCleanupObservation, lastSuccessfulCoverage, now time.Time) MetadataCleanupHealth {
	alert := func(reason string) MetadataCleanupHealth { return MetadataCleanupHealth{Alert: true, Reason: reason} }
	if now.IsZero() || lastSuccessfulCoverage.After(now) || (latest != nil && (latest.CompletedAt.After(now) || latest.CompletedAt.Before(latest.StartedAt))) {
		return alert("invalid_clock")
	}
	if latest == nil || lastSuccessfulCoverage.IsZero() {
		return alert("never_succeeded")
	}
	if !latest.Succeeded {
		return alert("run_failed")
	}
	if latest.RetentionViolations > 0 {
		return alert("retention_deadline")
	}
	if now.Sub(lastSuccessfulCoverage) >= MetadataCleanupHeartbeat {
		return alert("stale_heartbeat")
	}
	if strings.TrimSpace(latest.Stage) != "complete" {
		return alert("incomplete_observation")
	}
	return MetadataCleanupHealth{Reason: "healthy"}
}
