package supportdelete

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type preparationStoreSpy struct {
	Store
	acquires, recoveries int
}

func (s *preparationStoreSpy) Acquire(ctx context.Context, start Start) (Execution, error) {
	s.acquires++
	e, err := s.Store.Acquire(ctx, start)
	if err != nil {
		return e, fmt.Errorf("synthetic acquire: %w", err)
	}
	return e, nil
}

func (s *preparationStoreSpy) Recover(ctx context.Context, recovery Recovery, now time.Time) error {
	s.recoveries++
	if err := s.Store.Recover(ctx, recovery, now); err != nil {
		return fmt.Errorf("synthetic recovery: %w", err)
	}
	return nil
}

func scriptedCatalog(store *scriptedStore, effects Effects) SyntheticCatalog {
	catalog := testCatalog(effects)
	catalog.Selector, catalog.Manifest = store.selector, store.manifest
	for i := range catalog.Registrations {
		catalog.Registrations[i].Target = store.selector.Target
	}
	return catalog
}

func TestPrepareMissingAnyScopeOrStepCallsNoFactoryAcquireOrEffect(t *testing.T) {
	catalog := mockCatalog()
	for index, registration := range catalog.Registrations {
		t.Run("scope/"+registration.Scope, func(t *testing.T) {
			catalog := mockCatalog()
			catalog.Registrations = append(catalog.Registrations[:index:index], catalog.Registrations[index+1:]...)
			assertPreparationHasNoEffects(t, catalog)
		})
		for actionIndex, action := range registration.Actions {
			t.Run(action+"/"+registration.Scope, func(t *testing.T) {
				catalog := mockCatalog()
				actions := catalog.Registrations[index].Actions
				catalog.Registrations[index].Actions = append(actions[:actionIndex:actionIndex], actions[actionIndex+1:]...)
				assertPreparationHasNoEffects(t, catalog)
			})
		}
	}
}

func assertPreparationHasNoEffects(t *testing.T, catalog SyntheticCatalog) {
	t.Helper()
	store := &preparationStoreSpy{Store: &mockStore{}}
	effects := newScriptedEffects()
	for i := range catalog.Registrations {
		catalog.Registrations[i].Effects = effects
	}
	factories := 0
	prepared, err := PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) {
		factories++
		return store, nil
	}, syntheticTime)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Nil(t, prepared)
	require.Zero(t, factories)
	require.Zero(t, store.acquires)
	require.Zero(t, store.recoveries)
	require.Empty(t, effects.operations())
}

func TestPrepareBindsFactoryTargetAndStartsOnlyOnRun(t *testing.T) {
	store, effects := newScriptedStore(), newScriptedEffects()
	spy := &preparationStoreSpy{Store: store}
	catalog := scriptedCatalog(store, effects)
	factories := 0
	prepared, err := PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(_ context.Context, target Target) (Store, error) {
		factories++
		require.Equal(t, catalog.Selector.Target, target)
		return spy, nil
	}, syntheticTime)
	require.NoError(t, err)
	require.Equal(t, 1, factories)
	require.Zero(t, spy.acquires)
	require.Empty(t, effects.operations())
	// Changes to caller-owned descriptor storage cannot change this assembly.
	catalog.Manifest.Mode = "live"
	catalog.Selector.ProofRef = syntheticRef("changed-input")
	catalog.Registrations[0].Effects = (*nilEffects)(nil)
	catalog.Registrations[0].Actions[0] = "unknown"
	out, err := prepared.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, "MOCK_COMPLETED", out.Code)
	require.True(t, out.Offline)
	require.False(t, out.ActualExecution)
	require.Len(t, effects.operations(), 29)
	calls := len(effects.operations())
	_, err = prepared.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, effects.operations(), calls, "terminal retry must not execute effects")
}

func TestPrepareRefusesLiveAndUnboundCatalogBeforeDiscovery(t *testing.T) {
	for _, change := range []func(*SyntheticCatalog){
		func(c *SyntheticCatalog) {
			c.Manifest.Mode = "live"
			for i := range c.Registrations {
				c.Registrations[i].Mode = "live"
			}
		},
		func(c *SyntheticCatalog) {
			c.Selector.Target.Environment = "production"
			for i := range c.Registrations {
				c.Registrations[i].Target = c.Selector.Target
			}
		},
		func(c *SyntheticCatalog) {
			c.Selector.Target.ProjectID = "synthetic-not-demo"
			for i := range c.Registrations {
				c.Registrations[i].Target = c.Selector.Target
			}
		},
	} {
		catalog := mockCatalog()
		change(&catalog)
		assertPreparationHasNoEffects(t, catalog)
	}
	catalog := mockCatalog()
	selector := catalog.Selector
	selector.ProofRef = syntheticRef("unbound-proof")
	_, err := PrepareDeletion(context.Background(), selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) {
		t.Fatal("unbound catalog constructed a store")
		return nil, nil
	}, syntheticTime)
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestPreparationErrorsCancellationAndNilStoreNeverAcquire(t *testing.T) {
	catalog := mockCatalog()
	sentinel := errors.New("synthetic private factory failure")
	_, err := PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) { return nil, sentinel }, syntheticTime)
	require.ErrorIs(t, err, sentinel)
	_, err = PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) { return (*mockStore)(nil), nil }, syntheticTime)
	require.ErrorIs(t, err, ErrUnavailable)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = PrepareDeletion(ctx, catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) {
		t.Fatal("canceled preparation invoked factory")
		return nil, nil
	}, syntheticTime)
	require.ErrorIs(t, err, context.Canceled)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store := &preparationStoreSpy{Store: &mockStore{}}
	_, err = PrepareDeletion(ctx, catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) { cancel(); return store, nil }, syntheticTime)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, store.acquires)
	var zero PreparedDeletion
	require.ErrorIs(t, zero.Recover(context.Background(), Recovery{}), ErrUnavailable)
	_, err = zero.Run(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestPreparedRegistryRetainsEveryStepIdentityAndDeadlineAcrossLostACK(t *testing.T) {
	for cursor, step := range Steps() {
		for _, failure := range []string{"effect", "checkpoint"} {
			t.Run(step.Action+"/"+step.Scope+"/"+failure, func(t *testing.T) {
				store, effects := newScriptedStore(), newScriptedEffects()
				if failure == "effect" {
					effects.faultCursor, effects.faultAfter, effects.faultPending = cursor, true, true
				} else {
					store.commitFaultCursor, store.commitFaultAfter, store.commitFaultPending = cursor, true, true
				}
				catalog := scriptedCatalog(store, effects)
				prepared, err := PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) { return store, nil }, syntheticTime)
				require.NoError(t, err)
				_, err = prepared.Run(context.Background())
				require.ErrorIs(t, err, errSyntheticFailure)
				before, _, _ := store.snapshot()
				out, err := prepared.Run(context.Background())
				require.NoError(t, err)
				require.Equal(t, "MOCK_COMPLETED", out.Code)
				after, guard, done := store.snapshot()
				require.Equal(t, before.Selector, after.Selector)
				require.Equal(t, before.Cutoff, after.Cutoff)
				require.Equal(t, before.AcceptedAt, after.AcceptedAt)
				require.Equal(t, before.DeleteBy, after.DeleteBy)
				require.False(t, guard)
				require.True(t, done)
				ids := make(map[int]string)
				for _, operation := range effects.operations() {
					if id, seen := ids[operation.Execution.Cursor]; seen {
						require.Equal(t, id, operation.ID)
					}
					ids[operation.Execution.Cursor] = operation.ID
				}
				require.Len(t, ids, 29)
				for _, id := range ids {
					require.Equal(t, 1, effects.applications[id])
				}
			})
		}
	}
}

func TestPreparedRecoveryRequiresStoppedFenceAndRejectsConcurrentReuse(t *testing.T) {
	store, effects := newScriptedStore(), newScriptedEffects()
	spy := &preparationStoreSpy{Store: store}
	blocked := &blockingEffects{underlying: effects, entered: make(chan Operation, 1), release: make(chan struct{})}
	catalog := scriptedCatalog(store, blocked)
	prepared, err := PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) { return spy, nil }, syntheticTime)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := prepared.Run(ctx); finished <- err }()
	var first Operation
	select {
	case first = <-blocked.entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	_, err = prepared.Run(context.Background())
	require.ErrorIs(t, err, ErrBusy)
	require.ErrorIs(t, prepared.Recover(context.Background(), Recovery{}), ErrBusy)
	require.Equal(t, 1, spy.acquires)
	require.Zero(t, spy.recoveries)
	cancel()
	close(blocked.release)
	select {
	case err := <-finished:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
	e, _, _ := store.snapshot()
	recovery := Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: syntheticRef("new-owner"), ManifestRef: e.Manifest.Ref, TraceRef: syntheticRef("stopped-trace"), Revision: e.Revision, Generation: e.Generation, ObservedAt: syntheticTime()}
	require.ErrorIs(t, prepared.Recover(context.Background(), recovery), ErrConflict, "a proposal without stopped proof cannot transfer ownership")
	recovery.Stopped = true
	wrong := recovery
	wrong.Selector.ProofRef = syntheticRef("wrong-proof")
	require.ErrorIs(t, prepared.Recover(context.Background(), wrong), ErrConflict)
	require.Equal(t, 1, spy.recoveries, "unbound recovery reached the store")
	require.NoError(t, prepared.Recover(context.Background(), recovery))
	require.ErrorIs(t, prepared.Recover(context.Background(), recovery), ErrConflict, "replayed recovery must not transfer ownership twice")
	// The same operation retries under the newly fenced owner.
	_, err = prepared.Run(context.Background())
	require.NoError(t, err)
	operations := effects.operations()
	require.Len(t, operations, 29)
	require.Equal(t, first.ID, operations[0].ID)
	_, err = (&Engine{Store: store, Effects: effects, Manifest: catalog.Manifest, Clock: syntheticTime}).Run(context.Background(), catalog.Selector, recovery.PreviousOwnerRef)
	require.ErrorIs(t, err, ErrBusy)
}

func TestSyntheticEmulatorPreparationRetainsOfflineOutcome(t *testing.T) {
	store, effects := newScriptedStore(), newScriptedEffects()
	store.manifest.Mode = "emulator"
	e := syntheticExecution()
	e.Manifest = store.manifest
	store.e, store.guard = &e, true
	catalog := scriptedCatalog(store, effects)
	for i := range catalog.Registrations {
		catalog.Registrations[i].Mode = "emulator"
	}
	prepared, err := PrepareDeletion(context.Background(), catalog.Selector, e.OwnerRef, catalog, func(context.Context, Target) (Store, error) { return store, nil }, syntheticTime)
	require.NoError(t, err)
	out, err := prepared.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, "emulator", out.Mode)
	require.True(t, out.Offline)
	require.False(t, out.ActualExecution)
}

func TestPreparedRegistryPreservesGenerationFenceAndPostResumeData(t *testing.T) {
	for _, cursor := range []int{InspectionStart() + 3, ResumeStart()} {
		step := Steps()[cursor]
		t.Run(step.Action+"/"+step.Scope, func(t *testing.T) {
			store, effects := newScriptedStore(), newScriptedEffects()
			changed := false
			effects.after = func(op Operation) {
				if op.Execution.Cursor == cursor && !changed {
					changed = true
					store.moderationChanged()
				}
			}
			catalog := scriptedCatalog(store, effects)
			prepared, err := PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) { return store, nil }, syntheticTime)
			require.NoError(t, err)
			_, err = prepared.Run(context.Background())
			require.ErrorIs(t, err, ErrConflict)
			_, err = prepared.Run(context.Background())
			require.NoError(t, err)
			for _, op := range effects.operations() {
				if op.Execution.Generation == 2 {
					require.GreaterOrEqual(t, op.Execution.Cursor, InspectionStart(), "new generation repeated a delete")
				}
			}
			for _, users := range effects.users {
				require.True(t, users["new-legitimate-fixture-user"])
			}
		})
	}
	store, effects := newScriptedStore(), newScriptedEffects()
	store.finalizeFailures = 1
	catalog := scriptedCatalog(store, effects)
	prepared, err := PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) { return store, nil }, syntheticTime)
	require.NoError(t, err)
	_, err = prepared.Run(context.Background())
	require.ErrorIs(t, err, errSyntheticFailure)
	store.moderationChanged()
	_, err = prepared.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, effects.operations(), 29, "finalize retry repeated deletion or inspection")
	for _, users := range effects.users {
		require.True(t, users["new-legitimate-fixture-user"])
	}
}

type recoveryLostAckStore struct {
	Store
	applied bool
}

func (s *recoveryLostAckStore) Recover(ctx context.Context, proposal Recovery, now time.Time) error {
	if s.applied {
		if err := s.Store.Recover(ctx, proposal, now); err != nil {
			return fmt.Errorf("synthetic recovery: %w", err)
		}
	}
	return errSyntheticFailure
}

func TestPreparedRecoveryLostACKRequiresAuthoritativeOwnerRead(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(fmt.Sprint(applied), func(t *testing.T) {
			store, effects := newScriptedStore(), newScriptedEffects()
			catalog := scriptedCatalog(store, effects)
			owner := syntheticRef("owner")
			factory := func(context.Context, Target) (Store, error) {
				return &recoveryLostAckStore{Store: store, applied: applied}, nil
			}
			prepared, err := PrepareDeletion(context.Background(), catalog.Selector, owner, catalog, factory, syntheticTime)
			require.NoError(t, err)
			e, err := store.Acquire(context.Background(), Start{Selector: catalog.Selector, Manifest: catalog.Manifest, OwnerRef: owner, Now: syntheticTime()})
			require.NoError(t, err)
			proposal := Recovery{Selector: e.Selector, PreviousOwnerRef: owner, NewOwnerRef: syntheticRef("new-owner"), ManifestRef: e.Manifest.Ref, TraceRef: syntheticRef("stopped-trace"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: syntheticTime()}
			require.ErrorIs(t, prepared.Recover(context.Background(), proposal), errSyntheticFailure)
			require.Equal(t, owner, prepared.owner, "lost ACK must not imply successful recovery")
			restarted, err := PrepareDeletion(context.Background(), catalog.Selector, proposal.NewOwnerRef, catalog, factory, syntheticTime)
			require.NoError(t, err)
			_, err = restarted.Run(context.Background())
			if applied {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrBusy)
				require.Empty(t, effects.operations())
			}
		})
	}
}

func TestPreparationCancellationDuringClockDoesNotConstructStore(t *testing.T) {
	catalog := mockCatalog()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := PrepareDeletion(ctx, catalog.Selector, syntheticRef("owner"), catalog,
		func(context.Context, Target) (Store, error) {
			t.Fatal("expired preparation invoked factory")
			return nil, nil
		},
		func() time.Time { cancel(); return syntheticTime() })
	require.ErrorIs(t, err, context.Canceled)
}
