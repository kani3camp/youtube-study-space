package supportdelete

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SyntheticCatalog comes from independently seeded fixtures, never operator
// input. The existing static source inventory is not a live execution catalog.
// Preparation checks wiring and binding, not proof provenance or postconditions.
type SyntheticCatalog struct {
	Selector      Selector
	Manifest      Manifest
	Registrations []AdapterRegistration
}

// StoreFactory is invoked only after complete synthetic wiring validation.
// The injected caller owns any client lifecycle; this package creates none.
type StoreFactory func(context.Context, Target) (Store, error)

// PreparedDeletion does not expose mutable engine/adapter configuration. It
// binds one selector/manifest and transfers owner only through explicit recovery.
type PreparedDeletion struct {
	mu       sync.Mutex
	engine   Engine
	selector Selector
	owner    string
}

func PrepareDeletion(ctx context.Context, selector Selector, owner string, catalog SyntheticCatalog, factory StoreFactory, clock func() time.Time) (*PreparedDeletion, error) {
	if ctx == nil || catalog.Selector != selector {
		return nil, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prepare deletion interrupted: %w", err)
	}
	registry, err := NewRegistry(selector, catalog.Manifest, catalog.Registrations)
	if err != nil {
		return nil, fmt.Errorf("prepare deletion registry: %w", err)
	}
	if !ValidRef(owner) || factory == nil || clock == nil || clock().IsZero() {
		return nil, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prepare deletion interrupted: %w", err)
	}
	store, err := factory(ctx, selector.Target)
	if err != nil {
		return nil, fmt.Errorf("prepare deletion store: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prepare deletion interrupted: %w", err)
	}
	if nilDependency(store) {
		return nil, ErrUnavailable
	}
	return &PreparedDeletion{
		engine:   Engine{Store: store, Effects: registry, Manifest: catalog.Manifest, Clock: clock},
		selector: selector, owner: owner,
	}, nil
}

func (p *PreparedDeletion) blocked() Outcome {
	mode := "unknown"
	if p != nil {
		mode = p.engine.mode()
	}
	return Outcome{Code: "WORKFLOW_BLOCKED", Stage: "blocked", Mode: mode, Offline: true, ActualExecution: false}
}

func (p *PreparedDeletion) Run(ctx context.Context) (Outcome, error) {
	if p == nil || ctx == nil {
		return p.blocked(), ErrUnavailable
	}
	if !p.mu.TryLock() {
		return p.blocked(), ErrBusy
	}
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return p.blocked(), fmt.Errorf("deletion interrupted: %w", err)
	}
	return p.engine.Run(ctx, p.selector, p.owner)
}

// A caller's recovery proposal is not stopped-worker evidence. The Store must
// independently validate its trace, durable revision/generation and timestamp.
// This local lock is never evidence that another process/fleet has drained.
func (p *PreparedDeletion) Recover(ctx context.Context, recovery Recovery) error {
	if p == nil || ctx == nil || nilDependency(p.engine.Store) || p.engine.Clock == nil {
		return ErrUnavailable
	}
	if !p.mu.TryLock() {
		return ErrBusy
	}
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("deletion recovery interrupted: %w", err)
	}
	if recovery.Selector != p.selector || recovery.ManifestRef != p.engine.Manifest.Ref ||
		recovery.PreviousOwnerRef != p.owner || !ValidRef(recovery.NewOwnerRef) || recovery.NewOwnerRef == p.owner {
		return ErrConflict
	}
	if err := p.engine.Store.Recover(ctx, recovery, p.engine.Clock()); err != nil {
		return fmt.Errorf("recover prepared deletion: %w", err)
	}
	p.owner = recovery.NewOwnerRef
	return nil
}
