package supportdelete

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var errSyntheticFailure = errors.New("synthetic lost acknowledgement")

// scriptedStore is an independent fenced adapter double. Tests below exercise
// Engine orchestration against failures; production storage atomicity is covered
// by its own emulator tests, not by this double's admission policy.
type scriptedStore struct {
	mu                                   sync.Mutex
	selector                             Selector
	manifest                             Manifest
	e                                    *Execution
	done, guard                          bool
	generation                           int64
	commitFaultCursor                    int
	commitFaultAfter, commitFaultPending bool
	finalizeFailures                     int
}

func newScriptedStore() *scriptedStore {
	e := syntheticExecution()
	return &scriptedStore{selector: e.Selector, manifest: e.Manifest, generation: 1}
}

func (s *scriptedStore) Acquire(_ context.Context, start Start) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if start.Selector != s.selector || start.Manifest != s.manifest || !ValidRef(start.OwnerRef) {
		return Execution{}, ErrConflict
	}
	if s.e == nil {
		e := syntheticExecution()
		e.OwnerRef = start.OwnerRef
		e.GuardSince, e.Cutoff, e.UpdatedAt = start.Now, start.Now, start.Now
		s.e, s.guard = &e, true
	}
	if start.OwnerRef != s.e.OwnerRef {
		return Execution{}, ErrBusy
	}
	if s.done {
		return Execution{}, ErrCompleted
	}
	return *s.e, nil
}

func (s *scriptedStore) Refresh(_ context.Context, expected Execution, now time.Time) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.e == nil || expected != *s.e {
		return Execution{}, ErrConflict
	}
	if s.e.Generation != s.generation {
		if s.e.Cursor > InspectionStart() && s.e.Cursor < ResumeStart() {
			s.e.Cursor = InspectionStart()
		}
		s.e.Generation, s.e.UpdatedAt = s.generation, now
		s.e.Revision++
	}
	return *s.e, nil
}

func (s *scriptedStore) Commit(_ context.Context, expected Execution, evidence Evidence, now time.Time) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.e == nil || expected != *s.e || expected.Generation != s.generation {
		return Execution{}, ErrConflict
	}
	op := NewOperation(expected, evidence.ObservedAt)
	if evidence.Validate(op, now) != nil {
		return Execution{}, ErrEvidence
	}
	fault := s.commitFaultPending && expected.Cursor == s.commitFaultCursor
	if fault && !s.commitFaultAfter {
		s.commitFaultPending = false
		return Execution{}, errSyntheticFailure
	}
	s.e.Cursor++
	s.e.Revision++
	s.e.UpdatedAt = now
	s.e.EvidenceDigest = syntheticRef(s.e.EvidenceDigest + evidence.OperationID + fmt.Sprint(evidence.Generation))
	if fault {
		s.commitFaultPending = false
		return Execution{}, errSyntheticFailure
	}
	return *s.e, nil
}

func (s *scriptedStore) Finalize(_ context.Context, expected Execution, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.e == nil || expected != *s.e || expected.Generation != s.generation || expected.Cursor != len(Steps()) {
		return ErrConflict
	}
	if s.finalizeFailures > 0 {
		s.finalizeFailures--
		return errSyntheticFailure
	}
	s.done, s.guard = true, false
	return nil
}

func (s *scriptedStore) Recover(_ context.Context, recovery Recovery, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.e == nil || !recovery.Stopped || recovery.Selector != s.e.Selector || recovery.PreviousOwnerRef != s.e.OwnerRef || !ValidRef(recovery.NewOwnerRef) || recovery.ManifestRef != s.e.Manifest.Ref || !ValidRef(recovery.TraceRef) || recovery.Revision != s.e.Revision || recovery.Generation != s.generation || recovery.ObservedAt != now {
		return ErrConflict
	}
	s.e.OwnerRef = recovery.NewOwnerRef
	s.e.Revision++
	s.e.UpdatedAt = now
	return nil
}

func (s *scriptedStore) snapshot() (Execution, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.e == nil {
		return Execution{}, s.guard, s.done
	}
	return *s.e, s.guard, s.done
}

func (s *scriptedStore) moderationChanged() {
	s.mu.Lock()
	s.generation++
	s.mu.Unlock()
}

type scriptedEffects struct {
	mu                       sync.Mutex
	calls                    []Operation
	applications             map[string]int
	users                    map[string]map[string]bool
	faultCursor              int
	faultAfter, faultPending bool
	evidenceFaultCursor      int
	evidenceFault            func(*Evidence)
	after                    func(Operation)
}

func newScriptedEffects() *scriptedEffects {
	e := &scriptedEffects{applications: map[string]int{}, users: map[string]map[string]bool{}}
	for _, scope := range DeletionScopes() {
		e.users[scope] = map[string]bool{"old-fixture-user": true}
	}
	return e
}

func (s *scriptedEffects) Apply(_ context.Context, op Operation) (Evidence, error) {
	s.mu.Lock()
	s.calls = append(s.calls, op)
	fault := s.faultPending && op.Execution.Cursor == s.faultCursor
	if fault && !s.faultAfter {
		s.faultPending = false
		s.mu.Unlock()
		return Evidence{}, errSyntheticFailure
	}
	if s.applications[op.ID] == 0 {
		s.applications[op.ID]++
		if op.Step.Action == "delete" {
			clear(s.users[op.Step.Scope])
		}
		if op.Step.Action == "resume" {
			for _, users := range s.users {
				users["new-legitimate-fixture-user"] = true
			}
		}
	}
	v := completeEvidence(op)
	if s.evidenceFault != nil && op.Execution.Cursor == s.evidenceFaultCursor {
		s.evidenceFault(&v)
	}
	after := s.after
	if fault {
		s.faultPending = false
	}
	s.mu.Unlock()
	if after != nil {
		after(op)
	}
	if fault {
		return Evidence{}, errSyntheticFailure
	}
	return v, nil
}

func (s *scriptedEffects) operations() []Operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Operation(nil), s.calls...)
}

func newScriptedEngine(store *scriptedStore, effects *scriptedEffects) *Engine {
	return &Engine{Store: store, Effects: effects, Manifest: syntheticExecution().Manifest, Clock: syntheticTime}
}

func assertBlockedAt(t *testing.T, store *scriptedStore, effects *scriptedEffects, cursor int, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("workflow completed despite failed required evidence/effect/checkpoint")
	}
	e, guard, done := store.snapshot()
	if e.Cursor != cursor || !guard || done {
		t.Fatalf("failure advanced checkpoint or released privacy guard: cursor=%d guard=%t done=%t", e.Cursor, guard, done)
	}
	for _, op := range effects.operations() {
		if op.Execution.Cursor > cursor {
			t.Fatalf("effect %s/%s ran after blocked step", op.Step.Action, op.Step.Scope)
		}
	}
}

func TestEngineResumesEveryStepAcrossEffectAndCheckpointLostAcknowledgements(t *testing.T) {
	for cursor, step := range Steps() {
		for _, fault := range []string{"before-effect", "after-effect", "before-commit", "after-commit"} {
			t.Run(step.Action+"/"+step.Scope+"/"+fault, func(t *testing.T) {
				store, effects := newScriptedStore(), newScriptedEffects()
				if fault == "before-effect" || fault == "after-effect" {
					effects.faultCursor, effects.faultAfter, effects.faultPending = cursor, fault == "after-effect", true
				} else {
					store.commitFaultCursor, store.commitFaultAfter, store.commitFaultPending = cursor, fault == "after-commit", true
				}
				engine := newScriptedEngine(store, effects)
				out, err := engine.Run(context.Background(), store.selector, syntheticRef("owner"))
				if !errors.Is(err, errSyntheticFailure) || out.Code != "WORKFLOW_BLOCKED" {
					t.Fatalf("failure lost identity or reported success: %#v %v", out, err)
				}
				wantCursor := cursor
				if fault == "after-commit" {
					wantCursor++
				}
				assertBlockedAt(t, store, effects, wantCursor, err)
				before, _, _ := store.snapshot()
				out, err = engine.Run(context.Background(), store.selector, syntheticRef("owner"))
				if err != nil || out.Code != "MOCK_COMPLETED" || !out.Offline || out.ActualExecution {
					t.Fatalf("same execution did not resume: %#v %v", out, err)
				}
				after, guard, done := store.snapshot()
				if after.Selector != before.Selector || after.Cutoff != before.Cutoff || after.AcceptedAt != before.AcceptedAt || after.DeleteBy != before.DeleteBy || guard || !done {
					t.Fatal("retry changed request identity/deadline/cutoff or failed finalization")
				}
				counts := map[int]int{}
				ids := map[int]string{}
				for _, op := range effects.operations() {
					index := op.Execution.Cursor
					counts[index]++
					if ids[index] != "" && ids[index] != op.ID {
						t.Fatalf("retry changed operation ID at cursor %d", index)
					}
					ids[index] = op.ID
				}
				for index := range Steps() {
					want := 1
					if index == cursor && fault != "after-commit" {
						want = 2
					}
					if counts[index] != want || effects.applications[ids[index]] != 1 {
						t.Fatalf("step %d calls=%d applications=%d; want calls=%d application=1", index, counts[index], effects.applications[ids[index]], want)
					}
				}
				calls := len(effects.operations())
				if _, err = engine.Run(context.Background(), store.selector, syntheticRef("owner")); err != nil || len(effects.operations()) != calls {
					t.Fatal("completed retry repeated effects")
				}
			})
		}
	}
}

func TestEngineIncompleteRequiredEvidenceStopsBeforeNextEffect(t *testing.T) {
	for cursor, step := range Steps() {
		faults := map[string]func(*Evidence){"unknown": func(v *Evidence) { v.Known = false }}
		if step.Action == "pause" || step.Action == "drain" || step.Action == "resume" {
			faults["partial-fleet"] = func(v *Evidence) { v.AllInstances = false }
			faults["old-queue-unfenced"] = func(v *Evidence) { v.OldQueueRejected = false }
		}
		if step.Action == "delete" || step.Action == "inspect" {
			faults["restorable"] = func(v *Evidence) { v.RestoreExcluded = false }
			faults["remaining-data"] = func(v *Evidence) { v.Remaining = 1 }
		}
		for name, mutate := range faults {
			t.Run(step.Action+"/"+step.Scope+"/"+name, func(t *testing.T) {
				store, effects := newScriptedStore(), newScriptedEffects()
				engine := newScriptedEngine(store, effects)
				effects.evidenceFaultCursor, effects.evidenceFault = cursor, mutate
				_, err := engine.Run(context.Background(), store.selector, syntheticRef("owner"))
				if !errors.Is(err, ErrEvidence) {
					t.Fatalf("incomplete evidence accepted: %v", err)
				}
				assertBlockedAt(t, store, effects, cursor, err)
				effects.evidenceFault = nil
				if _, err := engine.Run(context.Background(), store.selector, syntheticRef("owner")); err != nil {
					t.Fatalf("trusted evidence could not resume same step: %v", err)
				}
			})
		}
	}
}

func TestEngineFinalizationFailureRetainsNewUserWithoutRepeatingDeletionOrInspection(t *testing.T) {
	store, effects := newScriptedStore(), newScriptedEffects()
	store.finalizeFailures = 2
	engine := newScriptedEngine(store, effects)
	for attempt := 0; attempt < 2; attempt++ {
		_, err := engine.Run(context.Background(), store.selector, syntheticRef("owner"))
		if !errors.Is(err, errSyntheticFailure) {
			t.Fatalf("finalization failure was hidden: %v", err)
		}
		assertBlockedAt(t, store, effects, len(Steps()), err)
		if len(effects.operations()) != len(Steps()) {
			t.Fatal("finalization retry repeated deletion, inspection or runtime effects")
		}
		for scope, users := range effects.users {
			if users["old-fixture-user"] || !users["new-legitimate-fixture-user"] {
				t.Fatalf("resumed legitimate fixture user was lost in %s", scope)
			}
		}
		// Independent moderation can advance the fence after services resume.
		// It must never turn finalization into another deletion/inspection pass.
		store.moderationChanged()
	}
	out, err := engine.Run(context.Background(), store.selector, syntheticRef("owner"))
	if err != nil || out.Code != "MOCK_COMPLETED" || len(effects.operations()) != len(Steps()) {
		t.Fatalf("safe finalization did not complete: %#v %v", out, err)
	}
	for scope, users := range effects.users {
		if !users["new-legitimate-fixture-user"] {
			t.Fatalf("completion removed the new fixture user in %s", scope)
		}
	}
}

func TestEngineModerationFenceRestartsPartialInspectionOnlyBeforeDurableResumeIntent(t *testing.T) {
	for _, cursor := range []int{InspectionStart(), InspectionStart() + 3, ResumeStart() - 1, ResumeStart(), ResumeStart() + 1} {
		step := Steps()[cursor]
		t.Run(fmt.Sprintf("%s/%s/cursor-%d", step.Action, step.Scope, cursor), func(t *testing.T) {
			store, effects := newScriptedStore(), newScriptedEffects()
			changed := false
			effects.after = func(op Operation) {
				if op.Execution.Cursor == cursor && !changed {
					changed = true
					store.moderationChanged()
				}
			}
			engine := newScriptedEngine(store, effects)
			_, err := engine.Run(context.Background(), store.selector, syntheticRef("owner"))
			if !errors.Is(err, ErrConflict) {
				t.Fatalf("concurrent generation mutation did not fence commit: %v", err)
			}
			assertBlockedAt(t, store, effects, cursor, err)
			out, err := engine.Run(context.Background(), store.selector, syntheticRef("owner"))
			if err != nil || out.Code != "MOCK_COMPLETED" {
				t.Fatalf("reconciled generation did not complete: %#v %v", out, err)
			}
			counts := map[int]int{}
			for _, op := range effects.operations() {
				counts[op.Execution.Cursor]++
				if op.Execution.Generation == 2 && op.Execution.Cursor < InspectionStart() {
					t.Fatalf("generation change repeated deletion/runtime suspension: %#v", op.Step)
				}
			}
			for index, planned := range Steps() {
				want := 1
				if cursor < ResumeStart() && index >= InspectionStart() && index <= cursor {
					want++
				}
				if cursor >= ResumeStart() && index == cursor {
					want++
				}
				if counts[index] != want {
					t.Fatalf("%s/%s calls=%d want=%d after generation refresh", planned.Action, planned.Scope, counts[index], want)
				}
			}
			for scope, users := range effects.users {
				if !users["new-legitimate-fixture-user"] {
					t.Fatalf("generation refresh deleted new legitimate fixture user in %s", scope)
				}
			}
		})
	}
}

func TestEngineRejectsCachedInspectionEvidenceFromPreviousGeneration(t *testing.T) {
	store, effects := newScriptedStore(), newScriptedEffects()
	cursor := InspectionStart() + 2
	changed := false
	effects.after = func(op Operation) {
		if op.Execution.Cursor == cursor && !changed {
			changed = true
			store.moderationChanged()
		}
	}
	engine := newScriptedEngine(store, effects)
	if _, err := engine.Run(context.Background(), store.selector, syntheticRef("owner")); !errors.Is(err, ErrConflict) {
		t.Fatalf("missing generation fence: %v", err)
	}
	effects.evidenceFaultCursor = InspectionStart()
	effects.evidenceFault = func(v *Evidence) { v.Generation = 1 }
	_, err := engine.Run(context.Background(), store.selector, syntheticRef("owner"))
	if !errors.Is(err, ErrEvidence) {
		t.Fatalf("cached old-generation proof authorized inspection: %v", err)
	}
	e, guard, done := store.snapshot()
	if e.Cursor != InspectionStart() || !guard || done {
		t.Fatal("cached evidence advanced checkpoint or released guard")
	}
	for _, op := range effects.operations() {
		if op.Execution.Generation == 2 && op.Execution.Cursor > InspectionStart() {
			t.Fatal("old-generation cached evidence authorized a later effect")
		}
	}
	effects.evidenceFault = nil
	if _, err := engine.Run(context.Background(), store.selector, syntheticRef("owner")); err != nil {
		t.Fatalf("fresh inspection evidence could not resume: %v", err)
	}
}

func TestEngineAdmissionRejectsProofReplayChangedSelectorAndElapsedTimeTakeover(t *testing.T) {
	store, effects := newScriptedStore(), newScriptedEffects()
	effects.faultCursor, effects.faultPending = InspectionStart(), true
	engine := newScriptedEngine(store, effects)
	if _, err := engine.Run(context.Background(), store.selector, syntheticRef("owner")); !errors.Is(err, errSyntheticFailure) {
		t.Fatalf("could not seed interrupted execution: %v", err)
	}
	original, _, _ := store.snapshot()
	calls := len(effects.operations())
	// Long passage of time is not an authorization source for taking ownership.
	engine.Clock = func() time.Time { return syntheticTime().Add(365 * 24 * time.Hour) }
	changes := map[string]func(*Selector, *string){
		"proof-replay-for-another-execution": func(s *Selector, _ *string) { s.ExecutionRef = syntheticRef("replay-execution") },
		"request":                            func(s *Selector, _ *string) { s.RequestRef = syntheticRef("other-request") },
		"proof":                              func(s *Selector, _ *string) { s.ProofRef = syntheticRef("other-proof") },
		"environment":                        func(s *Selector, _ *string) { s.Target.Environment = "production" },
		"project":                            func(s *Selector, _ *string) { s.Target.ProjectID = "demo-other" },
		"channel":                            func(s *Selector, _ *string) { s.Target.ChannelID = "UCsynthetic0000000000002" },
		"owner-after-one-year":               func(_ *Selector, owner *string) { *owner = syntheticRef("other-owner") },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			selector, owner := store.selector, syntheticRef("owner")
			change(&selector, &owner)
			out, err := engine.Run(context.Background(), selector, owner)
			if err == nil || out.Code != "WORKFLOW_BLOCKED" || len(effects.operations()) != calls {
				t.Fatalf("unauthorized admission reached effects: %#v %v", out, err)
			}
			current, guard, done := store.snapshot()
			if current != original || !guard || done {
				t.Fatal("failed admission changed owned checkpoint")
			}
		})
	}
	engine.Clock = syntheticTime
	if _, err := engine.Run(context.Background(), store.selector, syntheticRef("owner")); err != nil {
		t.Fatalf("same execution/owner could not resume: %v", err)
	}
}

type blockingEffects struct {
	underlying *scriptedEffects
	entered    chan Operation
	release    chan struct{}
	once       sync.Once
}

func (s *blockingEffects) Apply(ctx context.Context, op Operation) (Evidence, error) {
	s.once.Do(func() {
		s.entered <- op
		select {
		case <-s.release:
		case <-ctx.Done():
		}
	})
	if ctx.Err() != nil {
		return Evidence{}, fmt.Errorf("fixture worker interrupted: %w", ctx.Err())
	}
	return s.underlying.Apply(ctx, op)
}

func TestEngineConcurrentOwnerCannotExecuteUntilExplicitStoppedRecovery(t *testing.T) {
	store, effects := newScriptedStore(), newScriptedEffects()
	blocked := &blockingEffects{underlying: effects, entered: make(chan Operation, 1), release: make(chan struct{})}
	engine := newScriptedEngine(store, effects)
	engine.Effects = blocked
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := engine.Run(ctx, store.selector, syntheticRef("owner")); finished <- err }()
	var first Operation
	select {
	case first = <-blocked.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first owner did not reach effect")
	}
	second := newScriptedEngine(store, effects)
	second.Clock = func() time.Time { return syntheticTime().Add(365 * 24 * time.Hour) }
	if _, err := second.Run(context.Background(), store.selector, syntheticRef("recovered-owner")); !errors.Is(err, ErrBusy) {
		t.Fatalf("concurrent owner gained elapsed-time takeover: %v", err)
	}
	if len(effects.operations()) != 0 {
		t.Fatal("competing owner ran effects")
	}
	// Stop and wait for the old worker before admitting an explicit recovery.
	cancel()
	close(blocked.release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("old worker failed to acknowledge stop: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("old worker did not stop")
	}
	e, _, _ := store.snapshot()
	recovery := Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: syntheticRef("recovered-owner"), ManifestRef: e.Manifest.Ref, TraceRef: syntheticRef("stopped-worker-trace"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: syntheticTime()}
	if err := store.Recover(context.Background(), recovery, syntheticTime()); err != nil {
		t.Fatalf("explicit stopped recovery failed: %v", err)
	}
	second.Clock = syntheticTime
	if _, err := second.Run(context.Background(), store.selector, recovery.NewOwnerRef); err != nil {
		t.Fatalf("recovered owner could not resume execution: %v", err)
	}
	ops := effects.operations()
	if len(ops) != len(Steps()) || ops[0].ID != first.ID {
		t.Fatal("recovery changed execution operation identity or repeated completed work")
	}
	if _, err := engine.Run(context.Background(), store.selector, recovery.PreviousOwnerRef); !errors.Is(err, ErrBusy) {
		t.Fatalf("old owner retained authority after recovery: %v", err)
	}
}
