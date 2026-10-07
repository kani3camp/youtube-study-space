package mypage

import (
	"context"
	"errors"
	"sync"
	"time"

	"app.modules/core/supportdelete"
)

var (
	ErrRuntimeFenced = errors.New("MyPage runtime work fenced")
	// An injected dependency can use this when it returned without proving
	// whether its external operation completed. Cancellation is also unknown.
	ErrRuntimeOutcomeUnknown = errors.New("MyPage runtime outcome unknown")
)

// RuntimeRegistry tracks actual work in one process, including detached work
// and uncertain SDK outcomes. It is never evidence of fleet completeness.
// Construct one registry per server; do not reset it when an HTTP caller exits.
type RuntimeRegistry struct {
	mu                   sync.Mutex
	publication          chan struct{}
	environment, project string
	serial               uint64
	works                map[*runtimeWork]bool
	fences               map[string]*runtimeFence
}

type runtimeFence struct {
	execution supportdelete.Execution
	cut       uint64
	paused    bool
	resumed   bool
}

type runtimeWork struct {
	registry *RuntimeRegistry
	parent   *runtimeWork
	channel  string
	admitted uint64
	unknown  bool
}

type runtimeContextKey struct{}

func NewRuntimeRegistry(environment, project string) *RuntimeRegistry {
	return &RuntimeRegistry{environment: environment, project: project, publication: make(chan struct{}, 1), works: make(map[*runtimeWork]bool), fences: make(map[string]*runtimeFence)}
}

// RuntimeObservation deliberately contains counts only. Unattributed work
// blocks every target until it is bound or its actual dependency returns.
type RuntimeObservation struct {
	Active, Pending, Unattributed int64
	Paused                        bool
}

func (r *RuntimeRegistry) Observe(channel string) RuntimeObservation {
	if r == nil {
		return RuntimeObservation{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.observeLocked(channel)
}

func (r *RuntimeRegistry) observeLocked(channel string) RuntimeObservation {
	o := RuntimeObservation{}
	if fence := r.fences[channel]; fence != nil {
		o.Paused = fence.paused
	}
	for work, active := range r.works {
		if work.channel != "" && work.channel != channel {
			continue
		}
		if active {
			o.Active++
		}
		if work.unknown {
			o.Pending++
		}
		if work.channel == "" {
			o.Unattributed++
		}
	}
	return o
}

func beginRuntime(ctx context.Context, r *RuntimeRegistry, channel string) (context.Context, *runtimeWork, error) {
	if r == nil {
		return ctx, nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.works) >= 4096 || r.publication == nil || r.works == nil || r.fences == nil {
		return ctx, nil, ErrRuntimeFenced
	}
	parent, parentFound := ctx.Value(runtimeContextKey{}).(*runtimeWork)
	if !parentFound {
		parent = nil
	}
	if parent != nil && parent.registry != r {
		return ctx, nil, ErrRuntimeFenced
	}
	r.serial++
	work := &runtimeWork{registry: r, parent: parent, channel: channel, admitted: r.serial}
	if parent != nil {
		work.admitted = parent.admitted
		if channel == "" {
			work.channel = parent.channel
		} else if parent.channel != "" && parent.channel != channel {
			return ctx, nil, ErrRuntimeFenced
		}
	}
	if !r.allowedLocked(work.channel, work.admitted) {
		return ctx, nil, ErrRuntimeFenced
	}
	r.works[work] = true
	return context.WithValue(ctx, runtimeContextKey{}, work), work, nil
}

func (r *RuntimeRegistry) allowedLocked(channel string, admitted uint64) bool {
	if channel == "" {
		for _, fence := range r.fences {
			if fence.paused || admitted <= fence.cut {
				return false
			}
		}
		return true
	}
	fence := r.fences[channel]
	return fence == nil || (!fence.paused && admitted > fence.cut)
}

func (work *runtimeWork) bind(channel string) error {
	if work == nil {
		return nil
	}
	r := work.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	if !youtubeChannelID.MatchString(channel) {
		return ErrRuntimeFenced
	}
	for current := work; current != nil; current = current.parent {
		if current.channel != "" && current.channel != channel {
			return ErrRuntimeFenced
		}
	}
	// Binding is retained even if its pre-cutoff work is now rejected. It must
	// cease blocking unrelated channels, while still blocking this target.
	for current := work; current != nil; current = current.parent {
		current.channel = channel
	}
	if !r.allowedLocked(channel, work.admitted) {
		return ErrRuntimeFenced
	}
	return nil
}

func (work *runtimeWork) check() error {
	if work == nil {
		return nil
	}
	r := work.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	if work.unknown || !r.allowedLocked(work.channel, work.admitted) {
		return ErrRuntimeFenced
	}
	return nil
}

func (work *runtimeWork) provenance(createdAt, now time.Time) error {
	if work == nil {
		return nil
	}
	r := work.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	if createdAt.IsZero() || now.IsZero() || createdAt.After(now) || work.channel == "" {
		return ErrRuntimeFenced
	}
	if fence := r.fences[work.channel]; fence != nil && !createdAt.After(fence.execution.Cutoff) {
		return ErrRuntimeFenced
	}
	return nil
}

func (work *runtimeWork) done() {
	if work == nil {
		return
	}
	r := work.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	if work.unknown {
		r.works[work] = false
	} else {
		delete(r.works, work)
	}
}

func (work *runtimeWork) markUnknown() {
	if work != nil {
		work.registry.mu.Lock()
		work.unknown = true
		work.registry.mu.Unlock()
	}
}

func runtimeUnknownOutcome(ctx context.Context, work *runtimeWork, err error) {
	if err != nil && (errors.Is(err, ErrRuntimeOutcomeUnknown) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil) {
		work.markUnknown()
	}
}

// Publication and pause share a cancellable gate. A write already admitted
// finishes before pause can acknowledge its cutoff; observation remains usable
// while that write blocks. No SDK call or elapsed timeout proves completion.
func (work *runtimeWork) fence(ctx context.Context, publish func() error) error {
	if work == nil {
		return publish()
	}
	r := work.registry
	select {
	case r.publication <- struct{}{}:
		defer func() { <-r.publication }()
	case <-ctx.Done():
		return ErrRuntimeFenced
	}
	if err := work.check(); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ErrRuntimeFenced
	}
	return publish()
}

func runtimeMutation(ctx context.Context, work *runtimeWork, mutate func() error) error {
	return work.fence(ctx, func() error {
		err := mutate()
		// Stable validation/access errors denote a rejected atomic store write.
		// Raw dependency errors and temporary failures can be lost commit acks.
		if err != nil && (errorCode(err) == "INTERNAL_ERROR" || errorCode(err) == "TEMPORARY_UNAVAILABLE") {
			work.markUnknown()
		}
		return err
	})
}

func runtimeVersion(work *runtimeWork) uint64 {
	if work == nil {
		return 0
	}
	r := work.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	if fence := r.fences[work.channel]; fence != nil {
		return fence.cut
	}
	return 0
}

func (r *RuntimeRegistry) cacheAllowed(channel string, admitted uint64) bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.allowedLocked(channel, admitted)
}

// RuntimeDrainEffects composes local postconditions with an independently
// injected trusted fleet/manifest guard. Local counts never set AllInstances.
// It is not wired into the live CLI, whose execute refusal remains unchanged.
type RuntimeDrainEffects struct {
	Registry   *RuntimeRegistry
	FleetGuard supportdelete.Effects
	Clock      func() time.Time
}

func (a *RuntimeDrainEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	if a == nil || a.Registry == nil || a.FleetGuard == nil || a.Clock == nil || op.Execution.Validate() != nil || op.Execution.Cursor >= len(supportdelete.Steps()) || supportdelete.Steps()[op.Execution.Cursor] != op.Step || op.ID != supportdelete.NewOperation(op.Execution, op.ObservedAfter).ID || op.ObservedAfter.IsZero() || op.Step.Scope != "runtime-my-page" || (op.Step.Action != "pause" && op.Step.Action != "drain" && op.Step.Action != "resume") {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	r := a.Registry
	if op.Execution.Selector.Target.Environment != r.environment || op.Execution.Selector.Target.ProjectID != r.project {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	// Only independently trusted full-operation evidence can admit a control
	// transition, including a later deletion of the same channel.
	evidence, err := a.FleetGuard.Apply(ctx, op)
	if err != nil || evidence.Validate(op, a.Clock()) != nil {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	if err := r.control(ctx, op); err != nil {
		return supportdelete.Evidence{}, err
	}
	evidence.ObservedAt = a.Clock()
	if evidence.Validate(op, a.Clock()) != nil {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	return evidence, nil
}

func (r *RuntimeRegistry) control(ctx context.Context, op supportdelete.Operation) error {
	select {
	case r.publication <- struct{}{}:
		defer func() { <-r.publication }()
	case <-ctx.Done():
		return supportdelete.ErrUnavailable
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if ctx.Err() != nil {
		return supportdelete.ErrUnavailable
	}
	e := op.Execution
	channel := e.Selector.Target.ChannelID
	fence := r.fences[channel]
	if fence != nil {
		prior := fence.execution
		if prior.Selector != e.Selector {
			if op.Step.Action != "pause" || !fence.resumed || !e.Cutoff.After(prior.Cutoff) || e.Generation <= prior.Generation {
				return supportdelete.ErrConflict
			}
			// The new case receives a new barrier; the serial high-water mark
			// continues rejecting every result from all previous cases.
			fence = nil
		} else if prior.Manifest != e.Manifest || (prior.OwnerRef != e.OwnerRef && prior.Revision >= e.Revision) || prior.Generation > e.Generation || prior.Revision > e.Revision || !prior.Cutoff.Equal(e.Cutoff) {
			return supportdelete.ErrConflict
		}
	}
	switch op.Step.Action {
	case "pause":
		if fence != nil && fence.resumed {
			return supportdelete.ErrConflict
		}
		if fence == nil {
			r.serial++
			fence = &runtimeFence{cut: r.serial, paused: true}
			r.fences[channel] = fence
		}
	case "drain", "resume":
		if fence == nil || (!fence.paused && !fence.resumed) {
			return supportdelete.ErrEvidence
		}
		if fence.resumed {
			if op.Step.Action != "resume" {
				return supportdelete.ErrConflict
			}
			// Stable resume replay does not pause or erase fresh admitted work.
			fence.execution = e
			return nil
		}
		o := r.observeLocked(channel)
		if o.Active != 0 || o.Pending != 0 || o.Unattributed != 0 {
			return supportdelete.ErrEvidence
		}
		if op.Step.Action == "resume" {
			fence.paused, fence.resumed = false, true
		}
	}
	fence.execution = e
	return nil
}
