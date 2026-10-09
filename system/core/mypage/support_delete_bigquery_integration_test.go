//go:build integration

package mypage

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/core/supportdelete"
)

func emulatorBigQueryFixture(t *testing.T) (*FirestoreDeletionStore, supportdelete.Start, *BigQueryDeletionEffects, *fakeBigQueryInventory, *fakeBigQueryExecutor, supportdelete.Execution) {
	t.Helper()
	store, start, _ := deletionFixture(t)
	e, err := store.Acquire(context.Background(), start)
	require.NoError(t, err)
	e = advanceDeletion(t, store, e, bigQueryDeleteCursor(), start.Now)
	a, _, inventory, remote, _, op := bigQueryFixture(e, start.Now)
	a.Store = store
	return store, start, a, inventory, remote, op.Execution
}

func TestBigQueryEmulatorInventoryMarkerLostAckAndOwnerRecoveryKeepSameJobs(t *testing.T) {
	s, start, a, inventory, remote, e := emulatorBigQueryFixture(t)
	ctx := context.Background()
	remote.pending = true
	_, err := a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.Error(t, err)
	snapshot, err := s.LoadBigQueryInventory(ctx, e)
	require.NoError(t, err)
	require.False(t, snapshot.Deleted)
	first := remote.submitted[0]
	_, err = s.Commit(ctx, e, func() supportdelete.Evidence {
		v, err := (deletionFixtureEffects{}).Apply(ctx, supportdelete.NewOperation(e, start.Now))
		require.NoError(t, err)
		return v
	}(), start.Now)
	require.Error(t, err, "even fabricated complete evidence cannot advance an unconfirmed captured BQ snapshot")
	proof := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: digest(e.OwnerRef + ":recovered"), ManifestRef: e.Manifest.Ref, TraceRef: digest("synthetic-stopped-worker-with-bq-job-trace"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
	require.NoError(t, s.Recover(ctx, proof, start.Now))
	_, err = s.LoadBigQueryInventory(ctx, e)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	start.OwnerRef = proof.NewOwnerRef
	e, err = s.Acquire(ctx, start)
	require.NoError(t, err)
	snapshot, err = s.LoadBigQueryInventory(ctx, e)
	require.NoError(t, err)
	derived, err := bigQueryTargetJob(snapshot, first.Table)
	require.NoError(t, err)
	require.Equal(t, first.Identity, derived.Identity)
	remote.pending = false
	result := remote.jobs[first.Identity.JobID]
	result.Terminal = true
	remote.jobs[first.Identity.JobID] = result
	remote.removeTarget(first.Table, first.ChannelID)
	v, err := a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	next, err := s.Commit(ctx, e, v, start.Now)
	require.NoError(t, err)
	_, err = s.Commit(ctx, e, v, start.Now)
	require.ErrorIs(t, err, supportdelete.ErrConflict, "lost checkpoint acknowledgement cannot reuse the old fence")
	current, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	require.Equal(t, next, current)
	require.Equal(t, 5, remote.submitCalls)
	require.Equal(t, 1, inventory.calls)
}

func TestBigQueryEmulatorMissingHalfOrReplacedInventoryCannotRecapture(t *testing.T) {
	for _, condition := range []string{"snapshot-missing", "binding-missing", "binding-replaced", "both-missing", "execution-marker-replaced", "proof-claim-missing", "receipt-proof-replaced", "canonical-index-missing", "unknown-schema-field"} {
		t.Run(condition, func(t *testing.T) {
			s, start, a, inventory, remote, e := emulatorBigQueryFixture(t)
			ctx := context.Background()
			remote.pending = true
			_, err := a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
			require.Error(t, err)
			switch condition {
			case "snapshot-missing":
				_, err = s.Client.Collection(supportDeleteBigQueryInventory).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "binding-missing":
				_, err = s.Client.Collection(supportDeleteBigQueryBinding).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "both-missing":
				_, err = s.Client.Collection(supportDeleteBigQueryInventory).Doc(e.Selector.ExecutionRef).Delete(ctx)
				require.NoError(t, err)
				_, err = s.Client.Collection(supportDeleteBigQueryBinding).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "proof-claim-missing":
				_, err = s.Client.Collection(supportDeleteClaims).Doc(e.Selector.ProofRef).Delete(ctx)
			case "receipt-proof-replaced":
				_, err = s.Client.Collection("support-requests").Doc(e.Selector.RequestRef).Update(ctx, []firestore.Update{{Path: "proofRef", Value: digest("other-proof")}})
			case "canonical-index-missing":
				receiptDoc, readErr := s.Client.Collection("support-requests").Doc(e.Selector.RequestRef).Get(ctx)
				require.NoError(t, readErr)
				var receipt SupportRequest
				require.NoError(t, receiptDoc.DataTo(&receipt))
				_, err = s.Client.Collection("support-request-ids").Doc(digest(receipt.Environment + ":" + receipt.RequestID)).Delete(ctx)
			case "execution-marker-replaced":
				_, err = s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "bigQueryInventoryRef", Value: digest("replacement-execution-marker")}})
			case "binding-replaced":
				_, err = s.Client.Collection(supportDeleteBigQueryBinding).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "inventoryRef", Value: digest("replacement-inventory")}})
			case "unknown-schema-field":
				_, err = s.Client.Collection(supportDeleteBigQueryInventory).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "unknownField", Value: true}})
			}
			require.NoError(t, err)
			_, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
			require.Error(t, err)
			require.Equal(t, 1, inventory.calls)
			require.Equal(t, 1, remote.submitCalls)
		})
	}
}

func TestBigQueryEmulatorAuthAndBigQuerySnapshotsMoveAndFinalizeAtomically(t *testing.T) {
	s, start, oauthRef := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	start.Now = start.Now.Add(2 * time.Second)
	authAdapter, _, _, _ := authEmulatorFixture(t, s, e, oauthRef, start.Now)
	_, err = authAdapter.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, bigQueryDeleteCursor(), start.Now)
	a, _, _, remote, _, _ := bigQueryFixture(e, start.Now)
	a.Store = s
	v, err := a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	e, err = s.Commit(ctx, e, v, start.Now)
	require.NoError(t, err)
	authSnapshot, err := s.LoadAuthOwnership(ctx, e)
	require.NoError(t, err)
	bqSnapshot, err := s.LoadBigQueryInventory(ctx, e)
	require.NoError(t, err)
	require.Equal(t, e, authSnapshot.Execution)
	require.Equal(t, e, bqSnapshot.Execution)
	firstJob := remote.submitted[0]
	controls := &serviceaccess.FirestoreStore{Client: s.Client}
	_, err = controls.Change(ctx, e.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "SECURITY", Reference: digest("independent-bq-moderation")}, start.Now)
	require.NoError(t, err)
	e, err = s.Refresh(ctx, e, start.Now)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, bigQueryInspectionCursor(), start.Now)
	v, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	_, err = controls.Change(ctx, e.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: false}, start.Now)
	require.NoError(t, err)
	_, err = s.Commit(ctx, e, v, start.Now)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	e, err = s.Refresh(ctx, e, start.Now)
	require.NoError(t, err)
	// At this cursor Refresh still requires all prior inspection to be repeated.
	e = advanceDeletion(t, s, e, bigQueryInspectionCursor(), start.Now)
	bqSnapshot, err = s.LoadBigQueryInventory(ctx, e)
	require.NoError(t, err)
	require.False(t, bqSnapshot.Inspected)
	derived, err := bigQueryTargetJob(bqSnapshot, firstJob.Table)
	require.NoError(t, err)
	require.Equal(t, firstJob.Identity, derived.Identity)
	v, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	e, err = s.Commit(ctx, e, v, start.Now)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, len(supportdelete.Steps()), start.Now)
	_, err = controls.Change(ctx, e.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "SECURITY", Reference: digest("terminal-independent-moderation")}, start.Now)
	require.NoError(t, err)
	e, err = s.Refresh(ctx, e, start.Now)
	require.NoError(t, err)
	receipt := s.Client.Collection("support-requests").Doc(e.Selector.RequestRef)
	_, err = receipt.Update(ctx, []firestore.Update{{Path: "proofRef", Value: digest("drifted-bq-proof")}})
	require.NoError(t, err)
	require.ErrorIs(t, s.Finalize(ctx, e, start.Now), supportdelete.ErrConflict)
	_, err = s.LoadAuthOwnership(ctx, e)
	require.NoError(t, err)
	_, err = s.LoadBigQueryInventory(ctx, e)
	require.NoError(t, err)
	_, err = receipt.Update(ctx, []firestore.Update{{Path: "proofRef", Value: e.Selector.ProofRef}})
	require.NoError(t, err)
	require.NoError(t, s.Finalize(ctx, e, start.Now))
	for _, collection := range []string{supportDeleteAuthOwnership, supportDeleteBigQueryInventory, supportDeleteBigQueryBinding} {
		_, err = s.Client.Collection(collection).Doc(e.Selector.ExecutionRef).Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	control, err := controls.Read(ctx, e.Selector.Target.ChannelID)
	require.NoError(t, err)
	require.False(t, control.Control.PrivacyDeletion.Active)
	require.True(t, control.Control.Moderation.Active)
	require.Greater(t, control.Control.Generation, e.Generation)
	require.Equal(t, 5, remote.submitCalls)
}

func bigQueryInspectionCursor() int {
	for i, step := range supportdelete.Steps() {
		if step == (supportdelete.Step{Action: "inspect", Scope: "bigquery"}) {
			return i
		}
	}
	panic("required BQ inspection missing")
}

type bigQueryCheckpointFailures struct {
	supportdelete.Store
	loseBQCommit, failFinalize bool
}

func (s *bigQueryCheckpointFailures) Commit(ctx context.Context, e supportdelete.Execution, v supportdelete.Evidence, now time.Time) (supportdelete.Execution, error) {
	next, err := s.Store.Commit(ctx, e, v, now)
	if err == nil && s.loseBQCommit && e.Cursor == bigQueryDeleteCursor() {
		s.loseBQCommit = false
		return supportdelete.Execution{}, supportdelete.ErrUnavailable
	}
	return next, err
}

func (s *bigQueryCheckpointFailures) Finalize(ctx context.Context, e supportdelete.Execution, now time.Time) error {
	if s.failFinalize {
		s.failFinalize = false
		return supportdelete.ErrUnavailable
	}
	return s.Store.Finalize(ctx, e, now)
}

type trackedFixtureEffects struct {
	calls    []supportdelete.Step
	onResume func()
}

func (f *trackedFixtureEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	f.calls = append(f.calls, op.Step)
	if op.Step.Action == "resume" && f.onResume != nil {
		f.onResume()
	}
	return (deletionFixtureEffects{}).Apply(ctx, op)
}

func TestBigQueryEmulatorEngineBlocksUnknownAndResumesLostCheckpointWithoutDeletingFreshUse(t *testing.T) {
	s, start, a, _, remote, e := emulatorBigQueryFixture(t)
	ctx := context.Background()
	tracked := &trackedFixtureEffects{}
	adapters := map[string]supportdelete.Effects{}
	for _, scope := range append(supportdelete.RuntimeScopes(), supportdelete.DeletionScopes()...) {
		adapters[scope] = tracked
	}
	adapters["bigquery"] = a
	checkpoint := &bigQueryCheckpointFailures{Store: s, loseBQCommit: true, failFinalize: true}
	engine := &supportdelete.Engine{Store: checkpoint, Effects: supportdelete.Registry{Adapters: adapters}, Manifest: start.Manifest, Clock: a.Clock}
	remote.pending = true
	out, err := engine.Run(ctx, start.Selector, start.OwnerRef)
	require.Error(t, err)
	require.Equal(t, "WORKFLOW_BLOCKED", out.Code)
	require.Empty(t, tracked.calls, "unknown BQ job cannot reach Auth delete, resume or finalize")
	current, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	require.Equal(t, e.Cursor, current.Cursor)
	remote.pending = false
	first := remote.submitted[0]
	result := remote.jobs[first.Identity.JobID]
	result.Terminal = true
	remote.jobs[first.Identity.JobID] = result
	remote.removeTarget(first.Table, first.ChannelID)
	_, err = engine.Run(ctx, start.Selector, start.OwnerRef)
	require.Error(t, err, "lost BQ checkpoint acknowledgement interrupts this run")
	require.Empty(t, tracked.calls)
	current, err = s.Acquire(ctx, start)
	require.NoError(t, err)
	require.Equal(t, e.Cursor+1, current.Cursor)
	remote.lookupUnknown = true
	_, err = engine.Run(ctx, start.Selector, start.OwnerRef)
	require.Error(t, err)
	for _, step := range tracked.calls {
		require.NotEqual(t, "resume", step.Action)
	}
	control, err := (&serviceaccess.FirestoreStore{Client: s.Client}).Read(ctx, start.Selector.Target.ChannelID)
	require.NoError(t, err)
	require.True(t, control.Control.PrivacyDeletion.Active)
	remote.lookupUnknown = false
	fresh := remote.tables[0]
	tracked.onResume = func() {
		remote.rows[fresh.key()] = append(remote.rows[fresh.key()], map[string]string{fresh.TargetField: start.Selector.Target.ChannelID})
	}
	_, err = engine.Run(ctx, start.Selector, start.OwnerRef)
	require.Error(t, err, "finalization failure retains durable resume intent")
	calls := remote.submitCalls
	rows := len(remote.rows[fresh.key()])
	out, err = engine.Run(ctx, start.Selector, start.OwnerRef)
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", out.Code)
	require.True(t, out.Offline)
	require.False(t, out.ActualExecution)
	require.Equal(t, calls, remote.submitCalls)
	require.Len(t, remote.rows[fresh.key()], rows, "finalize retry never returns to deletion of new legitimate use")
}
