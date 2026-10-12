package supportdelete

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Engine struct {
	Store    Store
	Effects  Effects
	Manifest Manifest
	Clock    func() time.Time
}

func (r *Engine) Run(ctx context.Context, selector Selector, owner string) (Outcome, error) {
	blocked := Outcome{Code: "WORKFLOW_BLOCKED", Stage: "blocked", Mode: r.mode(), Offline: r.mode() != "live", ActualExecution: r.mode() == "live"}
	if r == nil || r.Store == nil || r.Effects == nil || r.Clock == nil || r.Manifest.Validate() != nil || selector.Validate() != nil || !ValidRef(owner) {
		return blocked, ErrUnavailable
	}
	e, err := r.Store.Acquire(ctx, Start{Selector: selector, Manifest: r.Manifest, OwnerRef: owner, Now: r.Clock()})
	if errors.Is(err, ErrCompleted) {
		return r.completed(), nil
	}
	if err != nil {
		return blocked, fmt.Errorf("acquire deletion checkpoint: %w", err)
	}
	for e.Cursor < len(Steps()) {
		if ctx.Err() != nil {
			return blocked, fmt.Errorf("deletion interrupted: %w", ctx.Err())
		}
		e, err = r.Store.Refresh(ctx, e, r.Clock())
		if err != nil {
			return blocked, fmt.Errorf("refresh deletion fence: %w", err)
		}
		if e.Validate() != nil {
			return blocked, ErrConflict
		}
		op := NewOperation(e, r.Clock())
		v, applyErr := r.Effects.Apply(ctx, op)
		if applyErr != nil {
			return blocked, fmt.Errorf("deletion effect unavailable: %w", applyErr)
		}
		if err = v.Validate(op, r.Clock()); err != nil {
			return blocked, err
		}
		e, err = r.Store.Commit(ctx, e, v, r.Clock())
		if err != nil {
			return blocked, fmt.Errorf("commit deletion checkpoint: %w", err)
		}
	}
	// Refresh may reconcile an independent moderation generation. Once resume
	// intent is durable, no retry can return to deletion/inspection of new users.
	e, err = r.Store.Refresh(ctx, e, r.Clock())
	if err != nil {
		return blocked, fmt.Errorf("refresh completion fence: %w", err)
	}
	if err = r.Store.Finalize(ctx, e, r.Clock()); err != nil {
		return blocked, fmt.Errorf("finalize deletion checkpoint: %w", err)
	}
	return r.completed(), nil
}

func (r *Engine) mode() string {
	if r == nil {
		return "unknown"
	}
	return r.Manifest.Mode
}

func (r *Engine) completed() Outcome {
	code := "COMPLETED"
	if r.Manifest.Mode == "mock" {
		code = "MOCK_COMPLETED"
	}
	return Outcome{Code: code, Stage: "completed", Mode: r.Manifest.Mode, Offline: r.Manifest.Mode != "live", ActualExecution: r.Manifest.Mode == "live"}
}
