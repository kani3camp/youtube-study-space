package supportdelete

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MockSelector identifies built-in synthetic fixtures; input refs never create
// proof or inventory evidence. The operator manifest selects an existing case.
func MockSelector() Selector {
	base := strings.Repeat("0123456789abcdef", 4)
	return Selector{Target: Target{Environment: "development", ProjectID: "demo-youtube-study-space-ci", ChannelID: "UCsynthetic0000000000001"}, RequestRef: base, ExecutionRef: "1" + base[1:], ProofRef: "2" + base[1:]}
}

type mockStore struct {
	execution          Execution
	claimed, completed bool
}

func (s *mockStore) Acquire(_ context.Context, start Start) (Execution, error) {
	if start.Selector != MockSelector() {
		return Execution{}, ErrInvalid
	}
	if s.completed {
		return Execution{}, ErrCompleted
	}
	if s.claimed {
		if s.execution.Selector != start.Selector || s.execution.Manifest != start.Manifest {
			return Execution{}, ErrConflict
		}
		if s.execution.OwnerRef != start.OwnerRef {
			return Execution{}, ErrBusy
		}
		return s.execution, nil
	}
	now := start.Now.UTC().Truncate(time.Microsecond)
	s.execution = Execution{Selector: start.Selector, Manifest: start.Manifest, OwnerRef: start.OwnerRef, Revision: 1, Generation: 1, GuardSince: now, Cutoff: now, AcceptedAt: now.Add(-time.Hour), DeleteBy: now.Add(-time.Hour).Add(7 * 24 * time.Hour), UpdatedAt: now, EvidenceDigest: strings.Repeat("a", 64)}
	s.claimed = true
	return s.execution, nil
}

func (s *mockStore) Refresh(_ context.Context, e Execution, _ time.Time) (Execution, error) {
	if s.completed || e != s.execution {
		return Execution{}, ErrConflict
	}
	return e, nil
}

func (s *mockStore) Commit(_ context.Context, e Execution, v Evidence, now time.Time) (Execution, error) {
	if s.completed || e != s.execution || e.Cursor >= len(Steps()) || v.Validate(NewOperation(e, e.UpdatedAt), now) != nil {
		return Execution{}, ErrConflict
	}
	e.Cursor++
	e.Revision++
	e.EvidenceDigest = EvidenceDigest(e.EvidenceDigest, v)
	e.UpdatedAt = now
	s.execution = e
	return e, nil
}

func (s *mockStore) Finalize(_ context.Context, e Execution, _ time.Time) error {
	if s.completed {
		return ErrCompleted
	}
	if e != s.execution || e.Cursor != len(Steps()) {
		return ErrConflict
	}
	s.completed = true
	return nil
}

func (s *mockStore) Recover(_ context.Context, v Recovery, now time.Time) error {
	e := s.execution
	if s.completed || !s.claimed {
		return ErrConflict
	}
	if !v.Stopped || !ValidRef(v.TraceRef) || !ValidRef(v.NewOwnerRef) || v.NewOwnerRef == v.PreviousOwnerRef || v.Selector != e.Selector || v.PreviousOwnerRef != e.OwnerRef || v.Revision != e.Revision || v.Generation != e.Generation || v.ManifestRef != e.Manifest.Ref || v.ObservedAt.Before(now.Add(-time.Minute)) || v.ObservedAt.After(now) || now.Before(e.UpdatedAt) || v.ObservedAt.Before(e.UpdatedAt) {
		return ErrEvidence
	}
	s.execution.OwnerRef = v.NewOwnerRef
	s.execution.Revision++
	return nil
}

type mockEffects struct{}

func (mockEffects) Apply(_ context.Context, op Operation) (Evidence, error) {
	e := op.Execution
	return Evidence{Selector: e.Selector, ManifestRef: e.Manifest.Ref, OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope, Mode: "mock", TraceRef: strings.Repeat("e", 64), Generation: e.Generation, Cutoff: e.Cutoff, ObservedAt: op.ObservedAfter, Known: true, Complete: true, AllInstances: true, OldQueueRejected: true, RestoreExcluded: true}, nil
}

func RunMock(ctx context.Context, selector Selector, recoverExecution bool) (Outcome, error) {
	if selector != MockSelector() {
		return Outcome{}, ErrInvalid
	}
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	manifest := Manifest{Ref: strings.Repeat("b", 64), Mode: "mock"}
	owner := strings.Repeat("c", 64)
	store := &mockStore{}
	if recoverExecution {
		old := strings.Repeat("d", 64)
		e, err := store.Acquire(ctx, Start{Selector: MockSelector(), Manifest: manifest, OwnerRef: old, Now: now})
		if err != nil {
			return Outcome{}, err
		}
		// The built-in stopped-worker fixture supplies evidence; no CLI field is
		// interpreted as an assertion about a real previous process.
		if err = store.Recover(ctx, Recovery{Selector: MockSelector(), PreviousOwnerRef: old, NewOwnerRef: owner, ManifestRef: manifest.Ref, TraceRef: strings.Repeat("e", 64), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: now}, now); err != nil {
			return Outcome{}, err
		}
	}
	return (&Engine{Store: store, Effects: mockEffects{}, Manifest: manifest, Clock: func() time.Time { return now }}).Run(ctx, selector, owner)
}

// Registry dispatches required scopes only to explicitly injected adapters.
// Missing Auth/BQ/backups/vendor/log/runtime adapters always stop the workflow.
type Registry struct{ Adapters map[string]Effects }

func (r Registry) Apply(ctx context.Context, op Operation) (Evidence, error) {
	adapter := r.Adapters[op.Step.Scope]
	if adapter == nil {
		return Evidence{}, ErrUnavailable
	}
	v, err := adapter.Apply(ctx, op)
	if err != nil {
		return Evidence{}, fmt.Errorf("apply required deletion adapter: %w", err)
	}
	return v, nil
}
