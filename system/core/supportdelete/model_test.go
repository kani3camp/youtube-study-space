package supportdelete

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// These tests use synthetic selectors and no SDK/bootstrap. The fixed plan and
// bound evidence are the observable contract, including retry after lost ack.
func syntheticRef(label string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte("support-delete-test:"+label)))
}

func syntheticSelector() Selector {
	return Selector{
		Target:       Target{Environment: "development", ProjectID: "demo-synthetic", ChannelID: "UCsynthetic0000000000001"},
		RequestRef:   syntheticRef("request"),
		ExecutionRef: syntheticRef("execution"),
		ProofRef:     syntheticRef("proof"),
	}
}

func syntheticTime() time.Time { return time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC) }

func syntheticExecution() Execution {
	now := syntheticTime()
	accepted := now.Add(-8 * time.Hour)
	return Execution{
		Selector: syntheticSelector(), Manifest: Manifest{Ref: syntheticRef("manifest"), Mode: "mock"},
		OwnerRef: syntheticRef("owner"), Revision: 1, Generation: 1,
		EvidenceDigest: syntheticRef("initial-proof-execution"),
		GuardSince:     now, Cutoff: now, AcceptedAt: accepted, DeleteBy: accepted.Add(7 * 24 * time.Hour), UpdatedAt: now,
	}
}

func completeEvidence(op Operation) Evidence {
	return Evidence{
		Selector: op.Execution.Selector, ManifestRef: op.Execution.Manifest.Ref,
		OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope,
		Mode: op.Execution.Manifest.Mode, TraceRef: syntheticRef("trace:" + op.ID),
		Generation: op.Execution.Generation, Cutoff: op.Execution.Cutoff, ObservedAt: op.ObservedAfter,
		Known: true, Complete: true, AllInstances: true, OldQueueRejected: true, RestoreExcluded: true,
	}
}

func TestStepPlanRequiresEveryScopeAndPostconditionBeforeResume(t *testing.T) {
	runtime := []string{"runtime-my-page", "runtime-legacy"}
	deleted := []string{"seat-state", "firestore-primary", "firestore-web", "firestore-oauth", "firestore-support-relations", "legacy-mappings", "firebase-auth", "bigquery", "backups-exports", "derived-vendor", "platform-logs"}
	if !reflect.DeepEqual(RuntimeScopes(), runtime) || !reflect.DeepEqual(DeletionScopes(), deleted) {
		t.Fatal("trusted deletion inventory changed; every scope requires explicit contract review")
	}
	want := []Step{{Action: "revoke", Scope: "firebase-auth"}}
	for _, action := range []string{"pause", "drain"} {
		for _, scope := range runtime {
			want = append(want, Step{Action: action, Scope: scope})
		}
	}
	for _, scope := range deleted {
		if scope != "firebase-auth" {
			want = append(want, Step{Action: "delete", Scope: scope})
		}
	}
	want = append(want, Step{Action: "delete", Scope: "firebase-auth"})
	for _, scope := range deleted {
		want = append(want, Step{Action: "inspect", Scope: scope})
	}
	for _, scope := range runtime {
		want = append(want, Step{Action: "resume", Scope: scope})
	}
	if !reflect.DeepEqual(Steps(), want) {
		t.Fatalf("workflow omitted or reordered a required fence/scope: got %#v", Steps())
	}
	if InspectionStart() != 16 || ResumeStart() != 27 {
		t.Fatalf("phase boundaries do not cover all scopes: inspect=%d resume=%d", InspectionStart(), ResumeStart())
	}
}

func TestEvidenceFailsClosedForEveryBindingAndPostcondition(t *testing.T) {
	changes := []struct {
		name   string
		mutate func(*Evidence)
	}{
		{"unknown", func(v *Evidence) { v.Known = false }},
		{"incomplete", func(v *Evidence) { v.Complete = false }},
		{"environment", func(v *Evidence) { v.Selector.Target.Environment = "production" }},
		{"project", func(v *Evidence) { v.Selector.Target.ProjectID = "demo-other" }},
		{"channel", func(v *Evidence) { v.Selector.Target.ChannelID = "UCsynthetic0000000000002" }},
		{"request", func(v *Evidence) { v.Selector.RequestRef = syntheticRef("other-request") }},
		{"execution", func(v *Evidence) { v.Selector.ExecutionRef = syntheticRef("other-execution") }},
		{"proof", func(v *Evidence) { v.Selector.ProofRef = syntheticRef("other-proof") }},
		{"manifest", func(v *Evidence) { v.ManifestRef = syntheticRef("other-manifest") }},
		{"mode", func(v *Evidence) { v.Mode = "live" }},
		{"operation", func(v *Evidence) { v.OperationID = syntheticRef("other-operation") }},
		{"trace", func(v *Evidence) { v.TraceRef = "missing" }},
		{"action", func(v *Evidence) { v.Action = "unrecognized" }},
		{"scope", func(v *Evidence) { v.Scope = "unknown-store" }},
		{"generation", func(v *Evidence) { v.Generation++ }},
		{"cutoff", func(v *Evidence) { v.Cutoff = v.Cutoff.Add(-time.Nanosecond) }},
		{"before-observation", func(v *Evidence) { v.ObservedAt = v.ObservedAt.Add(-time.Nanosecond) }},
		{"future-observation", func(v *Evidence) { v.ObservedAt = v.ObservedAt.Add(time.Minute + time.Nanosecond) }},
		{"remaining", func(v *Evidence) { v.Remaining = 1 }},
		{"unknown-negative-remaining", func(v *Evidence) { v.Remaining = -1 }},
		{"pending", func(v *Evidence) { v.Pending = 1 }},
		{"unknown-negative-pending", func(v *Evidence) { v.Pending = -1 }},
	}
	for cursor, step := range Steps() {
		t.Run(step.Action+"/"+step.Scope, func(t *testing.T) {
			e := syntheticExecution()
			e.Cursor = cursor
			op := NewOperation(e, syntheticTime())
			if err := completeEvidence(op).Validate(op, syntheticTime()); err != nil {
				t.Fatalf("complete bound postcondition rejected: %v", err)
			}
			for _, change := range changes {
				t.Run(change.name, func(t *testing.T) {
					v := completeEvidence(op)
					change.mutate(&v)
					if err := v.Validate(op, syntheticTime()); !errors.Is(err, ErrEvidence) {
						t.Fatalf("untrusted evidence authorized %s/%s: %v", step.Action, step.Scope, err)
					}
				})
			}
			if step.Action == "pause" || step.Action == "drain" || step.Action == "resume" {
				for _, flag := range []string{"all-instances", "old-queue-rejected"} {
					t.Run(flag, func(t *testing.T) {
						v := completeEvidence(op)
						if flag == "all-instances" {
							v.AllInstances = false
						} else {
							v.OldQueueRejected = false
						}
						if err := v.Validate(op, syntheticTime()); !errors.Is(err, ErrEvidence) {
							t.Fatalf("partial runtime fleet/old queue authorized %s: %v", step.Action, err)
						}
					})
				}
			}
			if step.Action == "delete" || step.Action == "inspect" {
				v := completeEvidence(op)
				v.RestoreExcluded = false
				if err := v.Validate(op, syntheticTime()); !errors.Is(err, ErrEvidence) {
					t.Fatalf("restorable deleted data authorized completion for %s: %v", step.Scope, err)
				}
			}
		})
	}
}

func TestOperationIDSurvivesRetryAndOwnerRecoveryButNotAnotherExecution(t *testing.T) {
	e := syntheticExecution()
	seen := map[string]Step{}
	for cursor := range Steps() {
		e.Cursor = cursor
		op := NewOperation(e, syntheticTime())
		if previous, exists := seen[op.ID]; exists {
			t.Fatalf("independent required effects share reconciliation identity: %#v and %#v", previous, op.Step)
		}
		seen[op.ID] = op.Step
		retried := e
		retried.OwnerRef = syntheticRef("recovered-owner")
		retried.Revision += 10
		retried.Generation++
		if got := NewOperation(retried, syntheticTime().Add(24*time.Hour)); got.ID != op.ID {
			t.Fatalf("%s/%s lost stable reconciliation identity", op.Step.Action, op.Step.Scope)
		}
		retried.Selector.ExecutionRef = syntheticRef("another-execution")
		if got := NewOperation(retried, syntheticTime()); got.ID == op.ID {
			t.Fatalf("%s/%s reused another execution's operation", op.Step.Action, op.Step.Scope)
		}
	}
}
