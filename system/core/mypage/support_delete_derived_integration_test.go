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

func emulatorDerivedFixture(t *testing.T) (*FirestoreDeletionStore, supportdelete.Start, *DerivedDeletionEffects, *derivedFake, supportdelete.Execution) {
	t.Helper()
	store, start, _ := deletionFixture(t)
	ctx := context.Background()
	e, err := store.Acquire(ctx, start)
	require.NoError(t, err)
	e = advanceDeletion(t, store, e, derivedDeleteCursor(), start.Now)
	a, _, fake, _ := derivedFixture()
	fixture, ok := a.Inventory.(derivedFixtureInventory)
	require.True(t, ok)
	s := fixture.snapshot
	s.Execution = e
	s.CaptureOperationID = supportdelete.NewOperation(e, start.Now).ID
	s.CapturedAt = start.Now
	fake.objects = map[string]*derivedObject{}
	for si := range s.Sources {
		for ii := range s.Sources[si].Items {
			item := &s.Sources[si].Items[ii]
			item.OwnerChannels[0] = e.Selector.Target.ChannelID
			fake.objects[item.ObjectID] = &derivedObject{item: *item, exists: true, owners: append([]string(nil), item.OwnerChannels...)}
		}
	}
	s.InventoryRef = derivedInventoryRef(s)
	a.Store = store
	a.Inventory = derivedFixtureInventory{s}
	a.Clock = func() time.Time { return start.Now }
	a.Guard = derivedFixtureGuard{start.Now}
	t.Cleanup(func() {
		for _, collection := range []string{supportDeleteDerivedInventory, supportDeleteDerivedBinding} {
			_, err := store.Client.Collection(collection).Doc(e.Selector.ExecutionRef).Delete(ctx)
			require.NoError(t, err)
		}
	})
	return store, start, a, fake, e
}

func derivedInspectCursor() int {
	for i, step := range supportdelete.Steps() {
		if step == (supportdelete.Step{Action: "inspect", Scope: "derived-vendor"}) {
			return i
		}
	}
	panic("derived inspection step missing")
}

func TestDerivedEmulatorRecoveryMarkerAndModerationPreservation(t *testing.T) {
	s, start, a, fake, e := emulatorDerivedFixture(t)
	ctx := context.Background()
	fake.lostAck = true
	v, err := a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	captured, err := s.LoadDerivedInventory(ctx, e)
	require.NoError(t, err)
	require.True(t, captured.Deleted)
	old := e
	e, err = s.Commit(ctx, e, v, start.Now)
	require.NoError(t, err)
	require.ErrorIs(t, func() error { _, err := s.Commit(ctx, old, v, start.Now); return err }(), supportdelete.ErrConflict)
	recovery := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: digest("new-derived-owner"), ManifestRef: e.Manifest.Ref, TraceRef: digest("derived-worker-stopped"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
	require.NoError(t, s.Recover(ctx, recovery, start.Now))
	_, err = s.LoadDerivedInventory(ctx, e)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	start.OwnerRef = recovery.NewOwnerRef
	e, err = s.Acquire(ctx, start)
	require.NoError(t, err)
	snapshot, err := s.LoadDerivedInventory(ctx, e)
	require.NoError(t, err)
	require.Equal(t, v.OperationID, snapshot.CaptureOperationID)
	e = advanceDeletion(t, s, e, derivedInspectCursor(), start.Now)
	v, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
	require.NoError(t, err)
	e, err = s.Commit(ctx, e, v, start.Now)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, len(supportdelete.Steps()), start.Now)
	controls := &serviceaccess.FirestoreStore{Client: s.Client}
	_, err = controls.Change(ctx, e.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "SECURITY", Reference: digest("derived-moderation")}, start.Now)
	require.NoError(t, err)
	e, err = s.Refresh(ctx, e, start.Now)
	require.NoError(t, err)
	require.NoError(t, s.Finalize(ctx, e, start.Now))
	for _, collection := range []string{supportDeleteDerivedInventory, supportDeleteDerivedBinding} {
		_, err = s.Client.Collection(collection).Doc(e.Selector.ExecutionRef).Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	control, err := controls.Read(ctx, e.Selector.Target.ChannelID)
	require.NoError(t, err)
	require.True(t, control.Control.Moderation.Active)
	require.False(t, control.Control.PrivacyDeletion.Active)
}

func TestDerivedEmulatorMissingOrReplacedInventoryCannotRecapture(t *testing.T) {
	for _, condition := range []string{"snapshot-missing", "binding-missing", "both-missing", "marker-replaced", "unknown-field"} {
		t.Run(condition, func(t *testing.T) {
			s, start, a, _, e := emulatorDerivedFixture(t)
			ctx := context.Background()
			_, err := a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
			require.NoError(t, err)
			switch condition {
			case "snapshot-missing":
				_, err = s.Client.Collection(supportDeleteDerivedInventory).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "binding-missing":
				_, err = s.Client.Collection(supportDeleteDerivedBinding).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "both-missing":
				_, err = s.Client.Collection(supportDeleteDerivedInventory).Doc(e.Selector.ExecutionRef).Delete(ctx)
				require.NoError(t, err)
				_, err = s.Client.Collection(supportDeleteDerivedBinding).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "marker-replaced":
				_, err = s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "derivedInventoryRef", Value: digest("replacement-derived-marker")}})
			case "unknown-field":
				_, err = s.Client.Collection(supportDeleteDerivedInventory).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "unknownField", Value: true}})
			}
			require.NoError(t, err)
			_, err = a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
			require.Error(t, err)
			_, err = s.LoadDerivedInventory(ctx, e)
			require.ErrorIs(t, err, supportdelete.ErrConflict)
		})
	}
}

func TestDerivedStoreRequiresInventoryAtDeleteAndAllowsPreCaptureRecovery(t *testing.T) {
	s, start, _ := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	// Earlier steps have no derived snapshot and may advance normally.
	e = advanceDeletion(t, s, e, derivedDeleteCursor(), start.Now)
	_, err = s.LoadDerivedInventory(ctx, e)
	require.ErrorIs(t, err, ErrDerivedInventoryMissing)
	proof := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: digest("pre-capture-derived-owner"), ManifestRef: e.Manifest.Ref, TraceRef: digest("pre-capture-worker-stopped"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
	require.NoError(t, s.Recover(ctx, proof, start.Now))
	start.OwnerRef = proof.NewOwnerRef
	e, err = s.Acquire(ctx, start)
	require.NoError(t, err)
	op := supportdelete.NewOperation(e, start.Now)
	fabricated, err := (deletionFixtureEffects{}).Apply(ctx, op)
	require.NoError(t, err)
	_, err = s.Commit(ctx, e, fabricated, start.Now)
	require.ErrorIs(t, err, supportdelete.ErrUnavailable)
	current, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	require.Equal(t, e, current)
	// Actual synthetic capture/readback permits the same transition.
	v, err := emptyDerivedAdapter(s, start.Now).Apply(ctx, op)
	require.NoError(t, err)
	next, err := s.Commit(ctx, e, v, start.Now)
	require.NoError(t, err)
	require.Equal(t, e.Cursor+1, next.Cursor)
}

func TestDerivedStoreMissingInventoryAfterDeleteBlocksInspectionRecoveryAndFinalize(t *testing.T) {
	for _, stage := range []string{"post-delete", "inspection", "finalize"} {
		t.Run(stage, func(t *testing.T) {
			s, start, _ := deletionFixture(t)
			ctx := context.Background()
			e, err := s.Acquire(ctx, start)
			require.NoError(t, err)
			switch stage {
			case "post-delete":
				e = advanceDeletion(t, s, e, derivedDeleteCursor()+1, start.Now)
			case "inspection":
				e = advanceDeletion(t, s, e, derivedInspectCursor(), start.Now)
			case "finalize":
				e = advanceDeletion(t, s, e, len(supportdelete.Steps()), start.Now)
			}
			for _, collection := range []string{supportDeleteDerivedInventory, supportDeleteDerivedBinding} {
				_, err = s.Client.Collection(collection).Doc(e.Selector.ExecutionRef).Delete(ctx)
				require.NoError(t, err)
			}
			_, err = s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "derivedInventoryRef", Value: firestore.Delete}})
			require.NoError(t, err)
			if stage == "finalize" {
				require.ErrorIs(t, s.Finalize(ctx, e, start.Now), supportdelete.ErrUnavailable)
			} else {
				v, effectErr := (deletionFixtureEffects{}).Apply(ctx, supportdelete.NewOperation(e, start.Now))
				require.NoError(t, effectErr)
				_, err = s.Commit(ctx, e, v, start.Now)
				require.ErrorIs(t, err, supportdelete.ErrUnavailable)
			}
			proof := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: digest("post-delete-derived-owner"), ManifestRef: e.Manifest.Ref, TraceRef: digest("post-delete-worker-stopped"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
			require.ErrorIs(t, s.Recover(ctx, proof, start.Now), supportdelete.ErrUnavailable)
			current, err := s.Acquire(ctx, start)
			require.NoError(t, err)
			require.Equal(t, e, current)
		})
	}
}

func TestDerivedStoreMismatchedSnapshotHalvesBlockCommit(t *testing.T) {
	for _, condition := range []string{"snapshot-missing", "binding-missing", "both-missing", "marker-missing", "marker-replaced"} {
		t.Run(condition, func(t *testing.T) {
			s, start, a, _, e := emulatorDerivedFixture(t)
			ctx := context.Background()
			v, err := a.Apply(ctx, supportdelete.NewOperation(e, start.Now))
			require.NoError(t, err)
			switch condition {
			case "snapshot-missing":
				_, err = s.Client.Collection(supportDeleteDerivedInventory).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "binding-missing":
				_, err = s.Client.Collection(supportDeleteDerivedBinding).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "both-missing":
				_, err = s.Client.Collection(supportDeleteDerivedInventory).Doc(e.Selector.ExecutionRef).Delete(ctx)
				require.NoError(t, err)
				_, err = s.Client.Collection(supportDeleteDerivedBinding).Doc(e.Selector.ExecutionRef).Delete(ctx)
			case "marker-missing":
				_, err = s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "derivedInventoryRef", Value: firestore.Delete}})
			case "marker-replaced":
				_, err = s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef).Update(ctx, []firestore.Update{{Path: "derivedInventoryRef", Value: digest("different-derived-inventory")}})
			}
			require.NoError(t, err)
			_, err = s.Commit(ctx, e, v, start.Now)
			require.ErrorIs(t, err, supportdelete.ErrConflict)
			proof := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: digest("mismatched-derived-owner"), ManifestRef: e.Manifest.Ref, TraceRef: digest("mismatched-worker-stopped"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
			require.ErrorIs(t, s.Recover(ctx, proof, start.Now), supportdelete.ErrConflict)
		})
	}
}
