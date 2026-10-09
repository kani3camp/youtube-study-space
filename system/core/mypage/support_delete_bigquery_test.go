package mypage

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/mybigquery"
	"app.modules/core/supportdelete"
)

type fakeBigQueryInventory struct {
	snapshot BigQueryInventorySnapshot
	calls    int
}

func (f *fakeBigQueryInventory) Capture(context.Context, supportdelete.Operation) (BigQueryInventorySnapshot, error) {
	f.calls++
	return f.snapshot, nil
}

type fakeBigQueryPersistence struct {
	snapshot   *BigQueryInventorySnapshot
	bound      bool
	confirmErr error
	checks     int
	onCheck    func(int)
}

func (f *fakeBigQueryPersistence) LoadBigQueryInventory(_ context.Context, e supportdelete.Execution) (BigQueryInventorySnapshot, error) {
	if f.snapshot == nil {
		if f.bound {
			return BigQueryInventorySnapshot{}, supportdelete.ErrConflict
		}
		return BigQueryInventorySnapshot{}, ErrBigQueryInventoryMissing
	}
	if f.snapshot.Execution != e {
		return BigQueryInventorySnapshot{}, supportdelete.ErrConflict
	}
	return *f.snapshot, nil
}

func (f *fakeBigQueryPersistence) CaptureBigQueryInventory(_ context.Context, s BigQueryInventorySnapshot) error {
	if f.bound {
		return supportdelete.ErrConflict
	}
	f.snapshot, f.bound = &s, true
	return nil
}

func (f *fakeBigQueryPersistence) CheckBigQueryInventory(_ context.Context, s BigQueryInventorySnapshot) error {
	f.checks++
	if f.onCheck != nil {
		f.onCheck(f.checks)
	}
	if f.snapshot == nil || f.snapshot.Execution != s.Execution {
		return supportdelete.ErrConflict
	}
	return nil
}

func (f *fakeBigQueryPersistence) ConfirmBigQueryInventory(_ context.Context, s BigQueryInventorySnapshot, inspect bool) error {
	if f.snapshot == nil || f.snapshot.Execution != s.Execution {
		return supportdelete.ErrConflict
	}
	if f.confirmErr != nil {
		return f.confirmErr
	}
	f.snapshot.Deleted = true
	f.snapshot.Inspected = f.snapshot.Inspected || inspect
	return nil
}

type fakeBigQueryGuard struct {
	denied, noCallback, noLock, noJobs, copiesUnknown, swallowError, twice bool
	lock                                                                   sync.Mutex
}

func (f *fakeBigQueryGuard) WithGuard(ctx context.Context, op supportdelete.Operation, _ BigQueryInventorySnapshot, apply func(context.Context, BigQueryDeletionPermit) error) error {
	if f.denied {
		return supportdelete.ErrEvidence
	}
	if f.noCallback {
		return nil
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	e := op.Execution
	permit := BigQueryDeletionPermit{Evidence: supportdelete.Evidence{Selector: e.Selector, ManifestRef: e.Manifest.Ref, OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope, Mode: e.Manifest.Mode, TraceRef: digest("synthetic-bq-guard:" + op.ID), Generation: e.Generation, Cutoff: e.Cutoff, ObservedAt: op.ObservedAfter, Known: true, Complete: true, RestoreExcluded: true}, CatalogLocked: !f.noLock, JobsFenced: !f.noJobs, ExternalCopiesDisposed: !f.copiesUnknown}
	err := apply(ctx, permit)
	if f.twice {
		return apply(ctx, permit)
	}
	if f.swallowError {
		return nil
	}
	return err
}

// A stateful synthetic system: rows, metadata and remote jobs live independently
// of adapter/persistence objects. Requests ending do not settle remote jobs.
type fakeBigQueryExecutor struct {
	tables                                                                                           []BigQueryTable
	source                                                                                           []BigQuerySourceJobObservation
	rows                                                                                             map[string][]map[string]string
	jobs                                                                                             map[string]BigQueryTargetJobObservation
	now                                                                                              time.Time
	submitCalls                                                                                      int
	submitted                                                                                        []BigQueryTargetJob
	submitErr, lookupErr, countErr, catalogErr                                                       error
	pending, failJob, lookupUnknown, keepRows, incomplete, wrongCount, staleCount, submissionMissing bool
	onSubmit                                                                                         func(BigQueryTargetJob)
}

func (f *fakeBigQueryExecutor) InspectCatalog(_ context.Context, s BigQueryInventorySnapshot) (BigQueryCatalogObservation, error) {
	return BigQueryCatalogObservation{Known: true, Complete: !f.incomplete, InventoryRef: s.InventoryRef, Tables: f.tables, SourceJobs: f.source, ObservedAt: f.now}, f.catalogErr
}

func (f *fakeBigQueryExecutor) LookupJob(_ context.Context, id BigQueryJobIdentity) (BigQueryTargetJobObservation, error) {
	if f.lookupErr != nil {
		return BigQueryTargetJobObservation{}, f.lookupErr
	}
	job, ok := f.jobs[id.JobID]
	if !ok {
		return job, ErrSyntheticBigQueryJobMissing
	}
	if f.lookupUnknown {
		job.Known = false
	}
	return job, nil
}

func (f *fakeBigQueryExecutor) SubmitTargetJob(_ context.Context, job BigQueryTargetJob) error {
	f.submitCalls++
	f.submitted = append(f.submitted, job)
	if f.submissionMissing {
		return context.DeadlineExceeded
	}
	if prior, ok := f.jobs[job.Identity.JobID]; ok {
		if prior.Identity != job.Identity {
			return supportdelete.ErrConflict
		}
		return f.submitErr
	}
	f.jobs[job.Identity.JobID] = BigQueryTargetJobObservation{Identity: job.Identity, Known: true, Terminal: !f.pending, Succeeded: !f.failJob}
	if !f.pending && !f.failJob && !f.keepRows {
		f.removeTarget(job.Table, job.ChannelID)
	}
	if f.onSubmit != nil {
		f.onSubmit(job)
	}
	return f.submitErr
}

func (f *fakeBigQueryExecutor) removeTarget(table BigQueryTable, channel string) {
	var kept []map[string]string
	for _, row := range f.rows[table.key()] {
		if row[table.TargetField] != channel {
			kept = append(kept, row)
		}
	}
	f.rows[table.key()] = kept
}

func (f *fakeBigQueryExecutor) CountTargetRows(_ context.Context, table BigQueryTable, channel string) (BigQueryTargetRows, error) {
	var remaining int64
	for _, row := range f.rows[table.key()] {
		if row[table.TargetField] == channel {
			remaining++
		}
	}
	now := f.now
	if f.staleCount {
		now = now.Add(-time.Minute)
	}
	if f.wrongCount {
		channel = "UCsynthetic0000000000002"
	}
	return BigQueryTargetRows{Table: table, ChannelID: channel, Known: true, Remaining: remaining, ObservedAt: now}, f.countErr
}

func bigQueryFixture(e supportdelete.Execution, now time.Time) (*BigQueryDeletionEffects, *fakeBigQueryPersistence, *fakeBigQueryInventory, *fakeBigQueryExecutor, *fakeBigQueryGuard, supportdelete.Operation) {
	e.Cursor = bigQueryDeleteCursor()
	op := supportdelete.NewOperation(e, now)
	tables := []BigQueryTable{}
	for _, spec := range []struct{ role, table, field string }{
		{"activity", mybigquery.UserActivityHistoryMainTableName, "user_id"},
		{"orders", mybigquery.OrderHistoryMainTableName, "user_id"},
		{"raw", mybigquery.LiveChatHistoryMainTableName, "author_channel_id"},
		{"tmp", mybigquery.TemporaryTableName, "user_id"},
		{"result", "synthetic_result", "user_id"},
	} {
		tables = append(tables, BigQueryTable{ProjectID: e.Selector.Target.ProjectID, Location: "asia-southeast2", DatasetID: mybigquery.DatasetName, TableID: spec.table, Role: spec.role, TargetField: spec.field, Exists: true, SchemaRef: digest(spec.table + ":schema"), IncarnationRef: digest(spec.table + ":incarnation")})
	}
	oldJob := BigQueryJobIdentity{ProjectID: e.Selector.Target.ProjectID, Location: "asia-southeast2", JobID: "synthetic_old_import", ConfigurationRef: digest("synthetic-old-import-config")}
	inventory := &fakeBigQueryInventory{snapshot: BigQueryInventorySnapshot{SchemaVersion: 1, Execution: e, CaptureOperationID: op.ID, TraceRef: digest("independent-complete-bq-inventory"), CapturedAt: now, Complete: true, Location: "asia-southeast2", Tables: tables, SourceJobs: []BigQueryJobIdentity{oldJob}, ExternalCopyRefs: []string{digest("sealed-external-export")}}}
	executor := &fakeBigQueryExecutor{tables: append([]BigQueryTable(nil), tables...), source: []BigQuerySourceJobObservation{{Identity: oldJob, Known: true, Terminal: true, ReplayExcluded: true}}, rows: map[string][]map[string]string{}, jobs: map[string]BigQueryTargetJobObservation{}, now: now}
	for _, table := range tables {
		executor.rows[table.key()] = []map[string]string{{table.TargetField: e.Selector.Target.ChannelID}, {table.TargetField: "UCsynthetic0000000000002"}}
	}
	store, guard := &fakeBigQueryPersistence{}, &fakeBigQueryGuard{}
	a := &BigQueryDeletionEffects{Store: store, Inventory: inventory, Scope: SyntheticBigQueryScope{ProjectID: e.Selector.Target.ProjectID, Location: "asia-southeast2", Executor: executor}, Guard: guard, Clock: func() time.Time { return executor.now }}
	return a, store, inventory, executor, guard, op
}

func unitBigQueryFixture() (*BigQueryDeletionEffects, *fakeBigQueryPersistence, *fakeBigQueryInventory, *fakeBigQueryExecutor, *fakeBigQueryGuard, supportdelete.Operation) {
	_, _, _, _, _, authOp := authAdapterFixture()
	return bigQueryFixture(authOp.Execution, authOp.ObservedAfter)
}

func advanceFakeBigQuery(store *fakeBigQueryPersistence, op supportdelete.Operation, action string) supportdelete.Operation {
	e := op.Execution
	for i, step := range supportdelete.Steps() {
		if step == (supportdelete.Step{Action: action, Scope: "bigquery"}) {
			e.Cursor = i
		}
	}
	e.Revision++
	e.UpdatedAt = op.ObservedAfter
	store.snapshot.Execution = e
	return supportdelete.NewOperation(e, op.ObservedAfter)
}

func TestBigQueryDeletionPreservesOtherChannelAndReusesDurablePlan(t *testing.T) {
	a, store, inventory, remote, _, op := unitBigQueryFixture()
	v, err := a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.NoError(t, v.Validate(op, a.Clock()))
	require.True(t, store.snapshot.Deleted)
	for _, rows := range remote.rows {
		require.Len(t, rows, 1)
		for _, row := range rows {
			for _, owner := range row {
				require.Equal(t, "UCsynthetic0000000000002", owner)
			}
		}
	}
	require.Equal(t, 5, remote.submitCalls)
	for _, job := range remote.submitted {
		require.NotContains(t, job.SQL, op.Execution.Selector.Target.ChannelID)
		require.Contains(t, job.SQL, "= @channel")
		require.Equal(t, op.Execution.Selector.Target.ChannelID, job.ChannelID)
	}
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.Equal(t, 5, remote.submitCalls)
	require.Equal(t, 1, inventory.calls)
	op = advanceFakeBigQuery(store, op, "inspect")
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.True(t, store.snapshot.Inspected)
	require.Equal(t, 5, remote.submitCalls, "inspection never submits jobs")
}

func TestBigQueryIncompleteOrChangedInventoryNeverStartsMutation(t *testing.T) {
	for _, condition := range []string{"missing", "incomplete", "project", "scope-project", "location", "scope-location", "schema", "incarnation", "duplicate", "missing-main", "unknown-role", "wrong-field", "future", "wrong-binding", "live", "catalog-incomplete", "catalog-timeout", "catalog-table", "source-pending", "source-unknown", "source-replay", "source-config", "unlisted-job", "guard", "no-lock", "no-jobs", "no-callback"} {
		t.Run(condition, func(t *testing.T) {
			a, _, inventory, remote, guard, op := unitBigQueryFixture()
			switch condition {
			case "missing":
				a.Inventory = nil
			case "incomplete":
				inventory.snapshot.Complete = false
			case "project":
				inventory.snapshot.Tables[0].ProjectID = "demo-other-project"
			case "scope-project":
				a.Scope.ProjectID = "demo-other-project"
			case "location":
				inventory.snapshot.Tables[0].Location = "US"
			case "scope-location":
				a.Scope.Location = "US"
			case "schema":
				inventory.snapshot.Tables[0].SchemaRef = ""
			case "incarnation":
				inventory.snapshot.Tables[0].IncarnationRef = ""
			case "duplicate":
				inventory.snapshot.Tables = append(inventory.snapshot.Tables, inventory.snapshot.Tables[0])
			case "missing-main":
				inventory.snapshot.Tables = inventory.snapshot.Tables[1:]
			case "unknown-role":
				inventory.snapshot.Tables[0].Role = "arbitrary"
			case "wrong-field":
				inventory.snapshot.Tables[0].TargetField = "author_channel_id"
			case "future":
				inventory.snapshot.CapturedAt = a.Clock().Add(time.Second)
			case "wrong-binding":
				inventory.snapshot.Execution.Selector.RequestRef = digest("other-request")
			case "live":
				op.Execution.Manifest.Mode = "live"
				op = supportdelete.NewOperation(op.Execution, op.ObservedAfter)
			case "catalog-incomplete":
				remote.incomplete = true
			case "catalog-timeout":
				remote.catalogErr = context.DeadlineExceeded
			case "catalog-table":
				remote.tables[0].SchemaRef = digest("changed-schema")
			case "source-pending":
				remote.source[0].Terminal = false
			case "source-unknown":
				remote.source[0].Known = false
			case "source-replay":
				remote.source[0].ReplayExcluded = false
			case "source-config":
				remote.source[0].Identity.ConfigurationRef = digest("changed-job")
			case "unlisted-job":
				remote.source = append(remote.source, remote.source[0])
			case "guard":
				guard.denied = true
			case "no-lock":
				guard.noLock = true
			case "no-jobs":
				guard.noJobs = true
			case "no-callback":
				guard.noCallback = true
			}
			_, err := a.Apply(context.Background(), op)
			require.Error(t, err)
			require.Zero(t, remote.submitCalls)
			for _, rows := range remote.rows {
				require.Len(t, rows, 2)
			}
		})
	}
}

func TestBigQueryJobUncertaintyCannotBecomeKnownZero(t *testing.T) {
	for _, condition := range []string{"pending", "failed", "lookup-timeout", "lookup-unknown", "count-timeout", "remaining", "wrong-count", "stale-count", "confirm-failed", "swallowed", "double-callback"} {
		t.Run(condition, func(t *testing.T) {
			a, store, _, remote, guard, op := unitBigQueryFixture()
			switch condition {
			case "pending":
				remote.pending = true
			case "failed":
				remote.failJob = true
			case "lookup-timeout":
				remote.lookupErr = context.DeadlineExceeded
			case "lookup-unknown":
				remote.lookupUnknown = true
			case "count-timeout":
				remote.countErr = context.DeadlineExceeded
			case "remaining":
				remote.keepRows = true
			case "wrong-count":
				remote.wrongCount = true
			case "stale-count":
				remote.staleCount = true
			case "confirm-failed":
				store.confirmErr = supportdelete.ErrUnavailable
			case "swallowed":
				remote.pending = true
				guard.swallowError = true
			case "double-callback":
				guard.twice = true
			}
			v, err := a.Apply(context.Background(), op)
			require.Error(t, err)
			require.Equal(t, supportdelete.Evidence{}, v)
		})
	}
}

func TestBigQueryLostSubmissionAcknowledgementReconcilesDefinitiveJob(t *testing.T) {
	a, _, _, remote, _, op := unitBigQueryFixture()
	remote.submitErr = context.DeadlineExceeded
	_, err := a.Apply(context.Background(), op)
	require.NoError(t, err, "definitive same-job readback reconciles lost submission ack")
	require.Equal(t, 5, remote.submitCalls)
}

func TestBigQueryPendingJobSurvivesWorkerRecoveryWithoutResubmission(t *testing.T) {
	a, store, inventory, remote, _, op := unitBigQueryFixture()
	remote.pending = true
	_, err := a.Apply(context.Background(), op)
	require.Error(t, err)
	job := remote.submitted[0]
	oldRef := store.snapshot.InventoryRef
	e := op.Execution
	e.OwnerRef = digest("recovered-owner")
	e.Revision++
	e.Generation++
	store.snapshot.Execution = e
	op = supportdelete.NewOperation(e, op.ObservedAfter)
	require.Equal(t, job.Identity, func() BigQueryJobIdentity {
		j, err := bigQueryTargetJob(*store.snapshot, job.Table)
		require.NoError(t, err)
		return j.Identity
	}())
	remote.pending = false
	result := remote.jobs[job.Identity.JobID]
	result.Terminal = true
	remote.jobs[job.Identity.JobID] = result
	remote.removeTarget(job.Table, job.ChannelID)
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.Equal(t, 5, remote.submitCalls, "first pending job reused; only other four submitted")
	require.Equal(t, oldRef, store.snapshot.InventoryRef)
	require.Equal(t, 1, inventory.calls)
}

func TestBigQueryMissingSnapshotAfterPartialEffectCannotRecapture(t *testing.T) {
	a, store, inventory, remote, _, op := unitBigQueryFixture()
	remote.pending = true
	_, err := a.Apply(context.Background(), op)
	require.Error(t, err)
	store.snapshot = nil
	_, err = a.Apply(context.Background(), op)
	require.Error(t, err)
	require.Equal(t, 1, inventory.calls)
	require.Equal(t, 1, remote.submitCalls)
}

func TestBigQueryReplacedJobOrFenceAndExternalCopyRemainBlocking(t *testing.T) {
	for _, condition := range []string{"job-config", "owner-during-submit", "external-copy", "missing-job-inspect", "remaining-inspect"} {
		t.Run(condition, func(t *testing.T) {
			a, store, _, remote, guard, op := unitBigQueryFixture()
			if condition == "owner-during-submit" {
				remote.onSubmit = func(BigQueryTargetJob) { store.snapshot.Execution.OwnerRef = digest("other-owner") }
				_, err := a.Apply(context.Background(), op)
				require.Error(t, err)
				require.Equal(t, 1, remote.submitCalls)
				return
			}
			_, err := a.Apply(context.Background(), op)
			require.NoError(t, err)
			op = advanceFakeBigQuery(store, op, "inspect")
			switch condition {
			case "job-config":
				j := remote.submitted[0]
				v := remote.jobs[j.Identity.JobID]
				v.Identity.ConfigurationRef = digest("replacement-job")
				remote.jobs[j.Identity.JobID] = v
			case "external-copy":
				guard.copiesUnknown = true
			case "missing-job-inspect":
				delete(remote.jobs, remote.submitted[0].Identity.JobID)
			case "remaining-inspect":
				table := remote.tables[0]
				remote.rows[table.key()] = append(remote.rows[table.key()], map[string]string{table.TargetField: op.Execution.Selector.Target.ChannelID})
			}
			_, err = a.Apply(context.Background(), op)
			require.Error(t, err)
			require.False(t, store.snapshot.Inspected)
			require.Equal(t, 5, remote.submitCalls)
		})
	}
}

func TestBigQueryExplicitAbsentRetiredTableAndEmptyTableAreSeparateFromUnknownSchema(t *testing.T) {
	a, _, inventory, remote, _, op := unitBigQueryFixture()
	raw := inventory.snapshot.Tables[2]
	raw.Exists, raw.SchemaRef, raw.IncarnationRef = false, "", ""
	inventory.snapshot.Tables[2], remote.tables[2] = raw, raw
	remote.rows[raw.key()] = nil
	remote.rows[remote.tables[3].key()] = nil
	_, err := a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.Equal(t, 4, remote.submitCalls)
	require.True(t, reflect.DeepEqual(remote.tables, inventory.snapshot.Tables))
}

func TestBigQuerySubmissionUnseenAndLateCrossTableReappearanceNeverComplete(t *testing.T) {
	t.Run("submission-unseen", func(t *testing.T) {
		a, store, inventory, remote, _, op := unitBigQueryFixture()
		remote.submissionMissing = true
		_, err := a.Apply(context.Background(), op)
		require.Error(t, err)
		require.False(t, store.snapshot.Deleted)
		first := remote.submitted[0]
		remote.submissionMissing = false
		_, err = a.Apply(context.Background(), op)
		require.NoError(t, err)
		require.Equal(t, first.Identity, remote.submitted[1].Identity)
		require.Equal(t, 1, inventory.calls)
	})
	t.Run("late-earlier-table-row", func(t *testing.T) {
		a, store, _, remote, _, op := unitBigQueryFixture()
		first := remote.tables[0]
		remote.onSubmit = func(job BigQueryTargetJob) {
			if job.Table != first {
				remote.rows[first.key()] = append(remote.rows[first.key()], map[string]string{first.TargetField: op.Execution.Selector.Target.ChannelID})
			}
		}
		_, err := a.Apply(context.Background(), op)
		require.Error(t, err)
		require.False(t, store.snapshot.Deleted, "final scan must revisit the early table after later effects")
	})
}
