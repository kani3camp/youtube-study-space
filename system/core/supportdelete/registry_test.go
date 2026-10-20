package supportdelete

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func testCatalog(effect Effects) SyntheticCatalog {
	catalog := mockCatalog()
	for i := range catalog.Registrations {
		catalog.Registrations[i].Effects = effect
	}
	return catalog
}

type nilEffects struct{}

func (*nilEffects) Apply(context.Context, Operation) (Evidence, error) {
	panic("nil adapter must never run")
}

func TestRegistryFreezesConnectionsAndRejectsUnboundOperations(t *testing.T) {
	original, replacement := newScriptedEffects(), newScriptedEffects()
	catalog := testCatalog(original)
	registry, err := NewRegistry(catalog.Selector, catalog.Manifest, catalog.Registrations)
	require.NoError(t, err)
	for i := range catalog.Registrations {
		catalog.Registrations[i].Effects = replacement
		catalog.Registrations[i].Actions[0] = "changed-action"
		catalog.Registrations[i].Target.ProjectID = "demo-other"
	}
	e := syntheticExecution()
	e.Selector, e.Manifest = registry.selector, registry.manifest
	for cursor, step := range Steps() {
		e.Cursor = cursor
		op := NewOperation(e, syntheticTime())
		v, err := registry.Apply(context.Background(), op)
		require.NoError(t, err)
		require.Equal(t, step, Step{Action: v.Action, Scope: v.Scope})
		require.NoError(t, v.Validate(op, syntheticTime()))
	}
	require.Len(t, original.operations(), 29)
	require.Empty(t, replacement.operations())
	for _, mutate := range []func(*Operation){
		func(o *Operation) { o.Execution.Selector.ProofRef = syntheticRef("other-proof") },
		func(o *Operation) { o.Execution.Selector.Target.ChannelID = "UCsynthetic0000000000002" },
		func(o *Operation) { o.Execution.Manifest.Ref = syntheticRef("other-fleet") },
		func(o *Operation) { o.Execution.Manifest.Mode = "live" },
		func(o *Operation) { o.ID = syntheticRef("changed-operation") },
		func(o *Operation) { o.Step.Scope = "unknown" },
		func(o *Operation) { o.Execution.Cursor = len(Steps()) },
		func(o *Operation) { o.ObservedAfter = o.Execution.UpdatedAt.Add(-1) },
	} {
		e.Cursor = 0
		op := NewOperation(e, syntheticTime())
		mutate(&op)
		_, err := registry.Apply(context.Background(), op)
		require.ErrorIs(t, err, ErrUnavailable)
	}
	require.Len(t, original.operations(), 29, "unbound input must not reach an adapter")
}

func TestRegistryRejectsMalformedAndNonSyntheticCapabilities(t *testing.T) {
	cases := map[string]func(*SyntheticCatalog){
		"duplicate scope":   func(c *SyntheticCatalog) { c.Registrations[1] = c.Registrations[0] },
		"unknown scope":     func(c *SyntheticCatalog) { c.Registrations[0].Scope = "unknown" },
		"unknown action":    func(c *SyntheticCatalog) { c.Registrations[0].Actions[0] = "destroy" },
		"duplicate action":  func(c *SyntheticCatalog) { c.Registrations[0].Actions[1] = c.Registrations[0].Actions[0] },
		"nil adapter":       func(c *SyntheticCatalog) { c.Registrations[0].Effects = nil },
		"typed nil adapter": func(c *SyntheticCatalog) { c.Registrations[0].Effects = (*nilEffects)(nil) },
		"wrong channel":     func(c *SyntheticCatalog) { c.Registrations[0].Target.ChannelID = "UCsynthetic0000000000002" },
		"wrong project":     func(c *SyntheticCatalog) { c.Registrations[0].Target.ProjectID = "demo-other" },
		"wrong environment": func(c *SyntheticCatalog) { c.Registrations[0].Target.Environment = "production" },
		"wrong mode":        func(c *SyntheticCatalog) { c.Registrations[0].Mode = "emulator" },
		"production":        func(c *SyntheticCatalog) { c.Selector.Target.Environment = "production" },
		"non demo":          func(c *SyntheticCatalog) { c.Selector.Target.ProjectID = "synthetic-not-demo" },
		"live": func(c *SyntheticCatalog) {
			c.Manifest.Mode = "live"
			for i := range c.Registrations {
				c.Registrations[i].Mode = "live"
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			effect := newScriptedEffects()
			catalog := testCatalog(effect)
			mutate(&catalog)
			registry, err := NewRegistry(catalog.Selector, catalog.Manifest, catalog.Registrations)
			require.ErrorIs(t, err, ErrUnavailable)
			require.Nil(t, registry)
			require.Empty(t, effect.operations())
			_, err = PrepareDeletion(context.Background(), catalog.Selector, syntheticRef("owner"), catalog, func(context.Context, Target) (Store, error) {
				t.Fatal("malformed capability constructed a store")
				return nil, nil
			}, syntheticTime)
			require.ErrorIs(t, err, ErrUnavailable)
		})
	}
}

type effectFunc func(context.Context, Operation) (Evidence, error)

func (f effectFunc) Apply(ctx context.Context, op Operation) (Evidence, error) { return f(ctx, op) }

func TestRegistryPreservesDependencyErrorsAndRejectsCancellation(t *testing.T) {
	sentinel := errors.New("synthetic private provider error")
	calls := 0
	catalog := testCatalog(effectFunc(func(context.Context, Operation) (Evidence, error) { calls++; return Evidence{}, sentinel }))
	registry, err := NewRegistry(catalog.Selector, catalog.Manifest, catalog.Registrations)
	require.NoError(t, err)
	e := syntheticExecution()
	e.Selector, e.Manifest = catalog.Selector, catalog.Manifest
	op := NewOperation(e, syntheticTime())
	_, err = registry.Apply(context.Background(), op)
	require.ErrorIs(t, err, sentinel)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = registry.Apply(ctx, op)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
	var empty Registry
	_, err = empty.Apply(context.Background(), op)
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestRegistryRejectsLateSuccessfulEvidenceAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	catalog := testCatalog(effectFunc(func(_ context.Context, op Operation) (Evidence, error) { cancel(); return completeEvidence(op), nil }))
	registry, err := NewRegistry(catalog.Selector, catalog.Manifest, catalog.Registrations)
	require.NoError(t, err)
	e := syntheticExecution()
	e.Selector, e.Manifest = catalog.Selector, catalog.Manifest
	evidence, err := registry.Apply(ctx, NewOperation(e, syntheticTime()))
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, Evidence{}, evidence, "canceled effect must not publish checkpoint evidence")
}
