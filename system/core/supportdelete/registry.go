package supportdelete

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// AdapterRegistration is supplied by trusted fixture assembly, never CLI JSON.
// Capabilities describe wiring, not evidence that a fleet/store is stopped or
// absent. Each adapter still owes the independently validated effect evidence.
type AdapterRegistration struct {
	Scope   string
	Target  Target
	Mode    string
	Actions []string
	Effects Effects
}

// Registry freezes the selected adapters and their selector/manifest binding.
// Adapter internals are not cloned; their own guards and fences remain required.
type Registry struct {
	selector Selector
	manifest Manifest
	adapters map[Step]Effects
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// NewRegistry performs no effects, store reads, SDK construction or discovery.
// It intentionally supports only development demo fixtures. A live registry
// and issuance of a trusted live manifest require separate engineering work.
func NewRegistry(selector Selector, manifest Manifest, registrations []AdapterRegistration) (*Registry, error) {
	if selector.Validate() != nil || manifest.Validate() != nil ||
		(manifest.Mode != "mock" && manifest.Mode != "emulator") ||
		selector.Target.Environment != "development" || !strings.HasPrefix(selector.Target.ProjectID, "demo-") {
		return nil, ErrUnavailable
	}
	required := make(map[string]map[string]bool)
	for _, step := range Steps() {
		if required[step.Scope] == nil {
			required[step.Scope] = make(map[string]bool)
		}
		required[step.Scope][step.Action] = true
	}
	if len(registrations) != len(required) {
		return nil, ErrUnavailable
	}
	registry := &Registry{selector: selector, manifest: manifest, adapters: make(map[Step]Effects)}
	seenScopes := make(map[string]bool)
	for _, registration := range registrations {
		actions, known := required[registration.Scope]
		if !known || seenScopes[registration.Scope] || registration.Target != selector.Target ||
			registration.Mode != manifest.Mode || nilDependency(registration.Effects) || len(registration.Actions) != len(actions) {
			return nil, ErrUnavailable
		}
		seenScopes[registration.Scope] = true
		seenActions := make(map[string]bool)
		for _, action := range registration.Actions {
			if !actions[action] || seenActions[action] {
				return nil, ErrUnavailable
			}
			seenActions[action] = true
			registry.adapters[Step{Action: action, Scope: registration.Scope}] = registration.Effects
		}
	}
	return registry, nil
}

func (r *Registry) Apply(ctx context.Context, op Operation) (Evidence, error) {
	if r == nil || ctx == nil || op.Execution.Validate() != nil || op.Execution.Cursor >= len(Steps()) ||
		op.Execution.Selector != r.selector || op.Execution.Manifest != r.manifest ||
		op != NewOperation(op.Execution, op.ObservedAfter) || op.ObservedAfter.Before(op.Execution.UpdatedAt) {
		return Evidence{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Evidence{}, fmt.Errorf("deletion adapter interrupted: %w", err)
	}
	adapter := r.adapters[op.Step]
	if nilDependency(adapter) {
		return Evidence{}, ErrUnavailable
	}
	v, err := adapter.Apply(ctx, op)
	if err != nil {
		return Evidence{}, fmt.Errorf("apply required deletion adapter: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Evidence{}, fmt.Errorf("deletion adapter interrupted: %w", err)
	}
	return v, nil
}
