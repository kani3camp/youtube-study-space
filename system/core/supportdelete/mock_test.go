package supportdelete

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mockStart() Start {
	return Start{Selector: MockSelector(), Manifest: Manifest{Ref: syntheticRef("mock-manifest"), Mode: "mock"}, OwnerRef: syntheticRef("mock-owner"), Now: syntheticTime()}
}

func TestMockStoreInputNeverCreatesProofAndClaimedExecutionRemainsFenced(t *testing.T) {
	changes := map[string]func(*Start){
		"proof-replay-another-execution": func(s *Start) { s.Selector.ExecutionRef = syntheticRef("replayed-execution") },
		"request":                        func(s *Start) { s.Selector.RequestRef = syntheticRef("other-request") },
		"proof":                          func(s *Start) { s.Selector.ProofRef = syntheticRef("other-proof") },
		"environment":                    func(s *Start) { s.Selector.Target.Environment = "production" },
		"project":                        func(s *Start) { s.Selector.Target.ProjectID = "demo-other" },
		"channel":                        func(s *Start) { s.Selector.Target.ChannelID = "UCsynthetic0000000000002" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			store := &mockStore{}
			start := mockStart()
			change(&start)
			if _, err := store.Acquire(context.Background(), start); !errors.Is(err, ErrInvalid) || store.claimed {
				t.Fatalf("input created proof or claimed invalid synthetic fixture: %v", err)
			}
		})
	}
	store := &mockStore{}
	start := mockStart()
	e, err := store.Acquire(context.Background(), start)
	if err != nil || e.Validate() != nil {
		t.Fatalf("trusted synthetic fixture did not claim a valid checkpoint: %v", err)
	}
	for _, change := range []func(*Start){
		func(s *Start) { s.OwnerRef = syntheticRef("second-owner") },
		func(s *Start) { s.Manifest.Ref = syntheticRef("changed-fleet") },
		func(s *Start) { s.Manifest.Mode = "emulator" },
	} {
		changed := start
		changed.Now = start.Now.Add(365 * 24 * time.Hour)
		change(&changed)
		if _, err := store.Acquire(context.Background(), changed); err == nil || store.execution != e {
			t.Fatalf("elapsed time or changed owner/fleet stole active checkpoint: %v", err)
		}
	}
	start.Now = start.Now.Add(365 * 24 * time.Hour)
	resumed, err := store.Acquire(context.Background(), start)
	if err != nil || resumed != e {
		t.Fatalf("same execution resume reset cutoff, deadline, evidence or cursor: %v", err)
	}
}

func TestMockStoreRejectsTamperedEvidenceWithoutAdvancingCheckpoint(t *testing.T) {
	changes := map[string]func(*Evidence){
		"unknown":         func(v *Evidence) { v.Known = false },
		"incomplete":      func(v *Evidence) { v.Complete = false },
		"execution":       func(v *Evidence) { v.Selector.ExecutionRef = syntheticRef("other-execution") },
		"proof":           func(v *Evidence) { v.Selector.ProofRef = syntheticRef("other-proof") },
		"manifest":        func(v *Evidence) { v.ManifestRef = syntheticRef("other-manifest") },
		"operation":       func(v *Evidence) { v.OperationID = syntheticRef("other-operation") },
		"trace":           func(v *Evidence) { v.TraceRef = "unknown" },
		"generation":      func(v *Evidence) { v.Generation++ },
		"action":          func(v *Evidence) { v.Action = "delete" },
		"scope":           func(v *Evidence) { v.Scope = "platform-logs" },
		"cutoff":          func(v *Evidence) { v.Cutoff = v.Cutoff.Add(time.Second) },
		"old-observation": func(v *Evidence) { v.ObservedAt = v.ObservedAt.Add(-time.Second) },
		"remaining":       func(v *Evidence) { v.Remaining = 1 },
		"pending":         func(v *Evidence) { v.Pending = 1 },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			store := &mockStore{}
			start := mockStart()
			e, err := store.Acquire(context.Background(), start)
			if err != nil {
				t.Fatal(err)
			}
			v := completeEvidence(NewOperation(e, start.Now))
			change(&v)
			if _, err := store.Commit(context.Background(), e, v, start.Now); err == nil || store.execution != e {
				t.Fatalf("untrusted evidence advanced mock checkpoint: %v", err)
			}
		})
	}
	store := &mockStore{}
	start := mockStart()
	e, err := store.Acquire(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	v := completeEvidence(NewOperation(e, start.Now))
	committed, err := store.Commit(context.Background(), e, v, start.Now)
	if err != nil || committed.Cursor != 1 || committed.Revision != e.Revision+1 || committed.EvidenceDigest == e.EvidenceDigest || !ValidRef(committed.EvidenceDigest) {
		t.Fatalf("valid evidence did not durably advance checkpoint chain: %v", err)
	}
	if _, err := store.Commit(context.Background(), e, v, start.Now); err == nil || store.execution != committed {
		t.Fatalf("stale checkpoint replay committed twice: %v", err)
	}
}

func TestMockStoreExplicitRecoveryRequiresFreshStoppedBoundEvidence(t *testing.T) {
	changes := map[string]func(*Recovery){
		"worker-not-stopped": func(r *Recovery) { r.Stopped = false },
		"trace":              func(r *Recovery) { r.TraceRef = "unknown" },
		"new-owner":          func(r *Recovery) { r.NewOwnerRef = "invalid" },
		"same-owner":         func(r *Recovery) { r.NewOwnerRef = r.PreviousOwnerRef },
		"previous-owner":     func(r *Recovery) { r.PreviousOwnerRef = syntheticRef("wrong-previous-owner") },
		"proof":              func(r *Recovery) { r.Selector.ProofRef = syntheticRef("other-proof") },
		"execution":          func(r *Recovery) { r.Selector.ExecutionRef = syntheticRef("other-execution") },
		"revision":           func(r *Recovery) { r.Revision++ },
		"generation":         func(r *Recovery) { r.Generation++ },
		"manifest":           func(r *Recovery) { r.ManifestRef = syntheticRef("other-fleet") },
		"stale-stop-trace":   func(r *Recovery) { r.ObservedAt = r.ObservedAt.Add(-time.Minute - time.Nanosecond) },
		"future-stop-trace":  func(r *Recovery) { r.ObservedAt = r.ObservedAt.Add(time.Nanosecond) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			store := &mockStore{}
			start := mockStart()
			e, err := store.Acquire(context.Background(), start)
			if err != nil {
				t.Fatal(err)
			}
			r := Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: syntheticRef("recovered-mock-owner"), ManifestRef: e.Manifest.Ref, TraceRef: syntheticRef("stopped-trace"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
			change(&r)
			if err := store.Recover(context.Background(), r, start.Now); err == nil || store.execution != e {
				t.Fatalf("unbound recovery changed execution: %v", err)
			}
		})
	}
	store := &mockStore{}
	start := mockStart()
	e, err := store.Acquire(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	r := Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: syntheticRef("recovered-mock-owner"), ManifestRef: e.Manifest.Ref, TraceRef: syntheticRef("stopped-trace"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
	if err := store.Recover(context.Background(), r, start.Now); err != nil {
		t.Fatal(err)
	}
	recovered := store.execution
	if recovered.Selector != e.Selector || recovered.Cursor != e.Cursor || recovered.Cutoff != e.Cutoff || recovered.DeleteBy != e.DeleteBy || recovered.EvidenceDigest != e.EvidenceDigest || recovered.OwnerRef != r.NewOwnerRef || recovered.Revision != e.Revision+1 {
		t.Fatal("explicit recovery reset identity/cutoff/evidence instead of transferring owner")
	}
	if err := store.Recover(context.Background(), r, start.Now); err == nil || store.execution != recovered {
		t.Fatalf("stopped evidence replay bypassed revision fence: %v", err)
	}
}

func TestMockCompletionIsIdempotentButCannotBeRecovered(t *testing.T) {
	store := &mockStore{}
	start := mockStart()
	effects := newScriptedEffects()
	engine := &Engine{Store: store, Effects: effects, Manifest: start.Manifest, Clock: syntheticTime}
	if _, err := engine.Run(context.Background(), start.Selector, start.OwnerRef); err != nil {
		t.Fatal(err)
	}
	calls := len(effects.operations())
	out, err := engine.Run(context.Background(), start.Selector, syntheticRef("fresh-completion-reader"))
	if err != nil || out.Code != "MOCK_COMPLETED" || len(effects.operations()) != calls {
		t.Fatalf("terminal completion response repeated effects or required erased owner data: %#v %v", out, err)
	}
	e := store.execution
	r := Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: syntheticRef("recovery-after-completion"), ManifestRef: e.Manifest.Ref, TraceRef: syntheticRef("stopped-trace"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: start.Now}
	if err := store.Recover(context.Background(), r, start.Now); err == nil || store.execution != e {
		t.Fatalf("completed execution admitted recovery: %v", err)
	}
}

func TestRegistryMissingAnyRequiredScopeStopsWorkflow(t *testing.T) {
	scopes := append(RuntimeScopes(), DeletionScopes()...)
	for _, missing := range scopes {
		t.Run(missing, func(t *testing.T) {
			store := &mockStore{}
			start := mockStart()
			effects := newScriptedEffects()
			registry := Registry{Adapters: map[string]Effects{}}
			for _, scope := range scopes {
				if scope != missing {
					registry.Adapters[scope] = effects
				}
			}
			engine := &Engine{Store: store, Effects: registry, Manifest: start.Manifest, Clock: syntheticTime}
			out, err := engine.Run(context.Background(), start.Selector, start.OwnerRef)
			if !errors.Is(err, ErrUnavailable) || out.Code != "WORKFLOW_BLOCKED" || store.completed {
				t.Fatalf("missing %s adapter authorized completion: %#v %v", missing, out, err)
			}
			firstMissing := 0
			for cursor, step := range Steps() {
				if step.Scope == missing {
					firstMissing = cursor
					break
				}
			}
			if store.execution.Cursor != firstMissing || len(effects.operations()) != firstMissing {
				t.Fatalf("missing adapter did not block before its first required effect: checkpoint=%d calls=%d", store.execution.Cursor, len(effects.operations()))
			}
		})
	}
}

func TestRunMockClearlyLabelsFixedSyntheticExecutionAndRecovery(t *testing.T) {
	for _, recoverExecution := range []bool{false, true} {
		out, err := RunMock(context.Background(), MockSelector(), recoverExecution)
		if err != nil || out.Code != "MOCK_COMPLETED" || out.Stage != "completed" || out.Mode != "mock" || !out.Offline || out.ActualExecution {
			t.Fatalf("synthetic workflow reported actual/live execution: %#v %v", out, err)
		}
	}
	selector := MockSelector()
	selector.ProofRef = syntheticRef("input-cannot-create-proof")
	if _, err := RunMock(context.Background(), selector, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("operator input minted mock proof: %v", err)
	}
}
