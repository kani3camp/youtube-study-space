package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"app.modules/core/mybigquery"
	"app.modules/core/supportdelete"
)

var (
	ErrBigQueryInventoryMissing    = errors.New("deletion BigQuery inventory missing")
	ErrSyntheticBigQueryJobMissing = errors.New("synthetic BigQuery job definitively missing")
	bqIdentifier                   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,127}$`)
	bqLocation                     = regexp.MustCompile(`^(US|EU|[a-z][a-z0-9-]{1,62})$`)
)

// BigQueryTableは、明示的に不存在の廃止済みテーブルを含め、スキーマと実体世代を固定する。
// 空・不明のスキーマを対象行0件の証拠にしない。
type BigQueryTable struct {
	ProjectID      string `firestore:"projectID"`
	Location       string `firestore:"location"`
	DatasetID      string `firestore:"datasetID"`
	TableID        string `firestore:"tableID"`
	Role           string `firestore:"role"`
	TargetField    string `firestore:"targetField"`
	Exists         bool   `firestore:"exists"`
	SchemaRef      string `firestore:"schemaRef"`
	IncarnationRef string `firestore:"incarnationRef"`
}

func (t BigQueryTable) key() string {
	return t.ProjectID + ":" + t.Location + ":" + t.DatasetID + ":" + t.TableID
}

type BigQueryJobIdentity struct {
	ProjectID        string `firestore:"projectID"`
	Location         string `firestore:"location"`
	JobID            string `firestore:"jobID"`
	ConfigurationRef string `firestore:"configurationRef"`
}

// BigQueryInventorySnapshotはサーバー専用の一時状態である。
// 不変の一覧と確定的な計画は所有者復旧・世代変更後も維持し、
// 実行保護情報はワークフローのチェックポイントとアトミックに進める。
type BigQueryInventorySnapshot struct {
	SchemaVersion      int64                   `firestore:"schemaVersion"`
	Execution          supportdelete.Execution `firestore:"execution"`
	CaptureOperationID string                  `firestore:"captureOperationID"`
	TraceRef           string                  `firestore:"traceRef"`
	CapturedAt         time.Time               `firestore:"capturedAt"`
	Complete           bool                    `firestore:"complete"`
	Location           string                  `firestore:"location"`
	Tables             []BigQueryTable         `firestore:"tables"`
	SourceJobs         []BigQueryJobIdentity   `firestore:"sourceJobs"`
	ExternalCopyRefs   []string                `firestore:"externalCopyRefs"`
	InventoryRef       string                  `firestore:"inventoryRef"`
	Deleted            bool                    `firestore:"deleted"`
	Inspected          bool                    `firestore:"inspected"`
}

func bigQueryDeleteCursor() int {
	for i, step := range supportdelete.Steps() {
		if step == (supportdelete.Step{Action: "delete", Scope: "bigquery"}) {
			return i
		}
	}
	panic("required BigQuery step missing")
}

func bigQueryInventoryRef(s BigQueryInventorySnapshot) string {
	// owner/revision/generation/cursorは可変の実行保護情報であり、ジョブ識別には使わない。
	b, err := json.Marshal(struct {
		Selector                               supportdelete.Selector
		Manifest                               supportdelete.Manifest
		Cutoff                                 time.Time
		CaptureOperationID, TraceRef, Location string
		CapturedAt                             time.Time
		Tables                                 []BigQueryTable
		SourceJobs                             []BigQueryJobIdentity
		ExternalCopyRefs                       []string
	}{s.Execution.Selector, s.Execution.Manifest, s.Execution.Cutoff, s.CaptureOperationID, s.TraceRef, s.Location, s.CapturedAt, s.Tables, s.SourceJobs, s.ExternalCopyRefs})
	if err != nil {
		return ""
	}
	return digest(string(b))
}

func (s BigQueryInventorySnapshot) validate() error {
	e := s.Execution
	initial := e
	initial.Cursor = bigQueryDeleteCursor()
	if s.SchemaVersion != 1 || e.Validate() != nil || e.Manifest.Mode == "live" || !s.Complete || !bqLocation.MatchString(s.Location) || !supportdelete.ValidRef(s.TraceRef) || s.CapturedAt.Before(e.Cutoff) || s.CaptureOperationID != supportdelete.NewOperation(initial, e.Cutoff).ID || !supportdelete.ValidRef(s.InventoryRef) || s.InventoryRef != bigQueryInventoryRef(s) || len(s.Tables) < 4 || len(s.Tables) > 32 || len(s.SourceJobs) > 64 || len(s.ExternalCopyRefs) > 64 || (s.Inspected && !s.Deleted) {
		return supportdelete.ErrEvidence
	}
	seen, roles := map[string]bool{}, map[string]bool{}
	for _, t := range s.Tables {
		if t.ProjectID != e.Selector.Target.ProjectID || t.Location != s.Location || !bqIdentifier.MatchString(t.DatasetID) || !bqIdentifier.MatchString(t.TableID) || seen[t.key()] || (t.Exists && (!supportdelete.ValidRef(t.SchemaRef) || !supportdelete.ValidRef(t.IncarnationRef))) || (!t.Exists && (t.SchemaRef != "" || t.IncarnationRef != "")) {
			return supportdelete.ErrEvidence
		}
		seen[t.key()] = true
		expected := map[string]struct{ table, field string }{
			"activity": {mybigquery.UserActivityHistoryMainTableName, "user_id"},
			"orders":   {mybigquery.OrderHistoryMainTableName, "user_id"},
			"raw":      {mybigquery.LiveChatHistoryMainTableName, "author_channel_id"},
			"tmp":      {mybigquery.TemporaryTableName, t.TargetField},
		}
		if required, ok := expected[t.Role]; ok {
			if roles[t.Role] || t.DatasetID != mybigquery.DatasetName || t.TableID != required.table || t.TargetField != required.field {
				return supportdelete.ErrEvidence
			}
			roles[t.Role] = true
		} else if t.Role != "result" {
			return supportdelete.ErrEvidence
		}
		if t.TargetField != "user_id" && t.TargetField != "author_channel_id" {
			return supportdelete.ErrEvidence
		}
	}
	if len(roles) != 4 {
		return supportdelete.ErrEvidence
	}
	seen = map[string]bool{}
	for _, j := range s.SourceJobs {
		if j.ProjectID != e.Selector.Target.ProjectID || j.Location != s.Location || !bqIdentifier.MatchString(j.JobID) || !supportdelete.ValidRef(j.ConfigurationRef) || seen[j.JobID] {
			return supportdelete.ErrEvidence
		}
		seen[j.JobID] = true
	}
	seen = map[string]bool{}
	for _, ref := range s.ExternalCopyRefs {
		if !supportdelete.ValidRef(ref) || seen[ref] {
			return supportdelete.ErrEvidence
		}
		seen[ref] = true
	}
	return nil
}

type BigQueryDeletionInventory interface {
	Capture(context.Context, supportdelete.Operation) (BigQueryInventorySnapshot, error)
}

type BigQueryDeletionPersistence interface {
	LoadBigQueryInventory(context.Context, supportdelete.Execution) (BigQueryInventorySnapshot, error)
	CaptureBigQueryInventory(context.Context, BigQueryInventorySnapshot) error
	CheckBigQueryInventory(context.Context, BigQueryInventorySnapshot) error
	ConfirmBigQueryInventory(context.Context, BigQueryInventorySnapshot, bool) error
}

type BigQuerySourceJobObservation struct {
	Identity                        BigQueryJobIdentity
	Known, Terminal, ReplayExcluded bool
}

type BigQueryCatalogObservation struct {
	Known, Complete bool
	InventoryRef    string
	ObservedAt      time.Time
	Tables          []BigQueryTable
	SourceJobs      []BigQuerySourceJobObservation
}

type BigQueryTargetJob struct {
	Identity                                  BigQueryJobIdentity
	Table                                     BigQueryTable
	OperationID, InventoryRef, ChannelID, SQL string
}

type BigQueryTargetJobObservation struct {
	Identity                   BigQueryJobIdentity
	Known, Terminal, Succeeded bool
}

type BigQueryTargetRows struct {
	Table      BigQueryTable
	ChannelID  string
	Known      bool
	Remaining  int64
	ObservedAt time.Time
}

// SyntheticBigQueryExecutorはSDK実装や初期化を持たない。
// 一覧の完全性・終了済みジョブ・対象件数は、注入した合成システムの確定的な観測を必要とし、
// リクエストのキャンセルから推測しない。
type SyntheticBigQueryExecutor interface {
	InspectCatalog(context.Context, BigQueryInventorySnapshot) (BigQueryCatalogObservation, error)
	LookupJob(context.Context, BigQueryJobIdentity) (BigQueryTargetJobObservation, error)
	SubmitTargetJob(context.Context, BigQueryTargetJob) error
	CountTargetRows(context.Context, BigQueryTable, string) (BigQueryTargetRows, error)
}

type SyntheticBigQueryScope struct {
	ProjectID, Location string
	Executor            SyntheticBigQueryExecutor
}

// 独立した信頼済み保護処理は、コールバック中のスキーマ・実体世代の変更、
// 新ジョブ・import・restore、旧ソースの再実行を排除する。
// 外部コピーは最終BQ調査前に、それぞれの担当スコープで処置する。
type BigQueryDeletionPermit struct {
	Evidence                                          supportdelete.Evidence
	CatalogLocked, JobsFenced, ExternalCopiesDisposed bool
}
type BigQueryDeletionGuard interface {
	WithGuard(context.Context, supportdelete.Operation, BigQueryInventorySnapshot, func(context.Context, BigQueryDeletionPermit) error) error
}

type BigQueryDeletionEffects struct {
	Store     BigQueryDeletionPersistence
	Inventory BigQueryDeletionInventory
	Scope     SyntheticBigQueryScope
	Guard     BigQueryDeletionGuard
	Clock     func() time.Time
}

func bigQueryTargetJob(s BigQueryInventorySnapshot, t BigQueryTable) (BigQueryTargetJob, error) {
	sql, err := mybigquery.TargetDeletionSQL(t.ProjectID, t.DatasetID, t.TableID, t.TargetField)
	if err != nil {
		return BigQueryTargetJob{}, supportdelete.ErrEvidence
	}
	j := BigQueryTargetJob{Table: t, OperationID: s.CaptureOperationID, InventoryRef: s.InventoryRef, ChannelID: s.Execution.Selector.Target.ChannelID, SQL: sql}
	j.Identity = BigQueryJobIdentity{ProjectID: t.ProjectID, Location: t.Location, JobID: "delete_" + digest(s.CaptureOperationID+":"+s.InventoryRef+":"+t.key())}
	b, err := json.Marshal(j)
	if err != nil {
		return BigQueryTargetJob{}, supportdelete.ErrEvidence
	}
	j.Identity.ConfigurationRef = digest(string(b))
	return j, nil
}

func (a *BigQueryDeletionEffects) snapshot(ctx context.Context, op supportdelete.Operation) (BigQueryInventorySnapshot, error) {
	s, err := a.Store.LoadBigQueryInventory(ctx, op.Execution)
	if errors.Is(err, ErrBigQueryInventoryMissing) {
		if op.Execution.Cursor != bigQueryDeleteCursor() || a.Inventory == nil {
			return s, supportdelete.ErrEvidence
		}
		s, err = a.Inventory.Capture(ctx, op)
		if err != nil || s.Execution != op.Execution || s.Deleted || s.Inspected || s.CapturedAt.Before(op.ObservedAfter) || s.CapturedAt.After(a.Clock()) {
			return s, supportdelete.ErrEvidence
		}
		s.CapturedAt = s.CapturedAt.UTC().Truncate(time.Microsecond)
		s.InventoryRef = bigQueryInventoryRef(s)
		if s.validate() != nil {
			return s, supportdelete.ErrEvidence
		}
		if err = a.Store.CaptureBigQueryInventory(ctx, s); err != nil {
			return s, fmt.Errorf("capture BigQuery inventory: %w", err)
		}
		s, err = a.Store.LoadBigQueryInventory(ctx, op.Execution)
	}
	if err != nil {
		return s, fmt.Errorf("load BigQuery inventory: %w", err)
	}
	if s.Execution != op.Execution || s.validate() != nil || s.CapturedAt.After(a.Clock()) {
		return s, supportdelete.ErrConflict
	}
	return s, nil
}

func (a *BigQueryDeletionEffects) checkCatalog(ctx context.Context, op supportdelete.Operation, s BigQueryInventorySnapshot) error {
	v, err := a.Scope.Executor.InspectCatalog(ctx, s)
	if err != nil {
		return supportdelete.ErrUnavailable
	}
	if !v.Known || !v.Complete || v.InventoryRef != s.InventoryRef || v.ObservedAt.Before(op.ObservedAfter) || v.ObservedAt.After(a.Clock()) || !reflect.DeepEqual(v.Tables, s.Tables) || len(v.SourceJobs) != len(s.SourceJobs) {
		return supportdelete.ErrEvidence
	}
	for i, job := range v.SourceJobs {
		if job.Identity != s.SourceJobs[i] || !job.Known || !job.Terminal || !job.ReplayExcluded {
			return supportdelete.ErrEvidence
		}
	}
	if err := a.Store.CheckBigQueryInventory(ctx, s); err != nil {
		return fmt.Errorf("check BigQuery catalog fence: %w", err)
	}
	return nil
}

func (a *BigQueryDeletionEffects) reconcileJob(ctx context.Context, job BigQueryTargetJob, inspect bool) error {
	v, err := a.Scope.Executor.LookupJob(ctx, job.Identity)
	if errors.Is(err, ErrSyntheticBigQueryJobMissing) && !inspect {
		// 送信ACKは証拠にしない。送信がタイムアウトやACK消失で終わっても、安定した識別情報で結果を照合する。
		submitErr := a.Scope.Executor.SubmitTargetJob(ctx, job)
		v, err = a.Scope.Executor.LookupJob(ctx, job.Identity)
		if err != nil && submitErr != nil {
			return supportdelete.ErrUnavailable
		}
	}
	if err != nil {
		return supportdelete.ErrUnavailable
	}
	if v.Identity != job.Identity || !v.Known || !v.Terminal || !v.Succeeded {
		return supportdelete.ErrEvidence
	}
	return nil
}

func (a *BigQueryDeletionEffects) verifyRows(ctx context.Context, op supportdelete.Operation, table BigQueryTable) error {
	rows, err := a.Scope.Executor.CountTargetRows(ctx, table, op.Execution.Selector.Target.ChannelID)
	if err != nil {
		return supportdelete.ErrUnavailable
	}
	if rows.Table != table || rows.ChannelID != op.Execution.Selector.Target.ChannelID || !rows.Known || rows.Remaining != 0 || rows.ObservedAt.Before(op.ObservedAfter) || rows.ObservedAt.After(a.Clock()) {
		return supportdelete.ErrEvidence
	}
	return nil
}

func (a *BigQueryDeletionEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	if a == nil || a.Store == nil || a.Guard == nil || a.Clock == nil || a.Scope.Executor == nil || op.Execution.Validate() != nil || op.Execution.Manifest.Mode == "live" || op.Execution.Cursor >= len(supportdelete.Steps()) || op != supportdelete.NewOperation(op.Execution, op.ObservedAfter) || op.ObservedAfter.Before(op.Execution.UpdatedAt) || op.ObservedAfter.After(a.Clock()) || op.Step.Scope != "bigquery" || (op.Step.Action != "delete" && op.Step.Action != "inspect") || a.Scope.ProjectID != op.Execution.Selector.Target.ProjectID {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	s, err := a.snapshot(ctx, op)
	if err != nil {
		return supportdelete.Evidence{}, err
	}
	if a.Scope.Location != s.Location || (op.Step.Action == "inspect" && !s.Deleted) {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	var evidence supportdelete.Evidence
	var callbackErr error
	called, completed := false, false
	err = a.Guard.WithGuard(ctx, op, s, func(guardCtx context.Context, permit BigQueryDeletionPermit) error {
		if called {
			callbackErr = supportdelete.ErrEvidence
			return callbackErr
		}
		called = true
		callbackErr = func() error {
			if permit.Evidence.Validate(op, a.Clock()) != nil || !permit.CatalogLocked || !permit.JobsFenced || (op.Step.Action == "inspect" && len(s.ExternalCopyRefs) > 0 && !permit.ExternalCopiesDisposed) {
				return supportdelete.ErrEvidence
			}
			if err := a.checkCatalog(guardCtx, op, s); err != nil {
				return fmt.Errorf("confirm BigQuery postcondition: %w", err)
			}
			for _, table := range s.Tables {
				if guardCtx.Err() != nil {
					return supportdelete.ErrUnavailable
				}
				if err := a.Store.CheckBigQueryInventory(guardCtx, s); err != nil {
					return fmt.Errorf("check BigQuery table fence: %w", err)
				}
				if table.Exists {
					job, err := bigQueryTargetJob(s, table)
					if err != nil {
						return err
					}
					if err = a.reconcileJob(guardCtx, job, op.Step.Action == "inspect"); err != nil {
						return err
					}
				}
				if err := a.verifyRows(guardCtx, op, table); err != nil {
					return err
				}
			}
			if err := a.checkCatalog(guardCtx, op, s); err != nil {
				return err
			}
			// 最後のテーブル操作後に、全ジョブと件数の確定状態を再確認する。
			// 以前の0件を最終的なテーブル横断の不存在観測にしない。
			for _, table := range s.Tables {
				if table.Exists {
					job, err := bigQueryTargetJob(s, table)
					if err != nil {
						return err
					}
					if err = a.reconcileJob(guardCtx, job, true); err != nil {
						return err
					}
				}
				if err := a.verifyRows(guardCtx, op, table); err != nil {
					return err
				}
			}
			if guardCtx.Err() != nil {
				return supportdelete.ErrUnavailable
			}
			if err := a.Store.ConfirmBigQueryInventory(guardCtx, s, op.Step.Action == "inspect"); err != nil {
				return fmt.Errorf("confirm BigQuery postcondition: %w", err)
			}
			evidence = permit.Evidence
			evidence.ObservedAt = a.Clock()
			completed = true
			return nil
		}()
		return callbackErr
	})
	if callbackErr != nil {
		return supportdelete.Evidence{}, callbackErr
	}
	if err != nil {
		return supportdelete.Evidence{}, fmt.Errorf("guard BigQuery deletion: %w", err)
	}
	if !called || !completed {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	if evidence.Validate(op, a.Clock()) != nil {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	return evidence, nil
}
