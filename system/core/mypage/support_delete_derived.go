package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"app.modules/core/serviceaccess"
	"app.modules/core/supportdelete"
)

var ErrDerivedInventoryMissing = errors.New("derived deletion inventory missing")

// Every source must be inventoried, including sources with no objects. The
// production writers currently do not persist OpenAI response or Discord
// message IDs; a synthetic empty list is not evidence about those services.
var derivedSources = []string{"bff-cache", "work-name-trend", "openai-response", "discord-message", "youtube-copy"}

type DerivedItem struct {
	Source         string   `firestore:"source"`
	ObjectID       string   `firestore:"objectID"`
	IncarnationRef string   `firestore:"incarnationRef"`
	Disposition    string   `firestore:"disposition"`
	OwnerChannels  []string `firestore:"ownerChannels"`
}

// Disposition is either delete-exclusive or remove-target. A shared object
// may only use remove-target when the inventoried provider can actually edit
// the target's portion without deleting another owner's portion.
type DerivedSource struct {
	Name      string        `firestore:"name"`
	Complete  bool          `firestore:"complete"`
	Supported bool          `firestore:"supported"`
	Items     []DerivedItem `firestore:"items"`
}

type DerivedInventorySnapshot struct {
	SchemaVersion      int64                   `firestore:"schemaVersion"`
	Execution          supportdelete.Execution `firestore:"execution"`
	CaptureOperationID string                  `firestore:"captureOperationID"`
	TraceRef           string                  `firestore:"traceRef"`
	InventoryRef       string                  `firestore:"inventoryRef"`
	CapturedAt         time.Time               `firestore:"capturedAt"`
	Sources            []DerivedSource         `firestore:"sources"`
	Deleted            bool                    `firestore:"deleted"`
	Inspected          bool                    `firestore:"inspected"`
}

func derivedDeleteCursor() int {
	for i, step := range supportdelete.Steps() {
		if step == (supportdelete.Step{Action: "delete", Scope: "derived-vendor"}) {
			return i
		}
	}
	panic("required derived-vendor step missing")
}

func derivedInventoryRef(s DerivedInventorySnapshot) string {
	sources := append([]DerivedSource(nil), s.Sources...)
	for i := range sources {
		if len(sources[i].Items) == 0 {
			sources[i].Items = nil
		}
	}
	b, err := json.Marshal(struct {
		Selector                     supportdelete.Selector
		Manifest                     supportdelete.Manifest
		Cutoff                       time.Time
		CaptureOperationID, TraceRef string
		CapturedAt                   time.Time
		Sources                      []DerivedSource
	}{s.Execution.Selector, s.Execution.Manifest, s.Execution.Cutoff, s.CaptureOperationID, s.TraceRef, s.CapturedAt, sources})
	if err != nil {
		return ""
	}
	return digest(string(b))
}

func (s DerivedInventorySnapshot) validate() error {
	e := s.Execution
	initial := e
	initial.Cursor = derivedDeleteCursor()
	if s.SchemaVersion != 1 || e.Validate() != nil || e.Manifest.Mode == "live" || !supportdelete.ValidRef(s.TraceRef) || s.CapturedAt.Before(e.Cutoff) || s.CaptureOperationID != supportdelete.NewOperation(initial, e.Cutoff).ID || s.InventoryRef != derivedInventoryRef(s) || (s.Inspected && !s.Deleted) || len(s.Sources) != len(derivedSources) {
		return supportdelete.ErrEvidence
	}
	seen := map[string]bool{}
	for i, source := range s.Sources {
		if source.Name != derivedSources[i] || !source.Complete || !source.Supported || len(source.Items) > 1000 {
			return supportdelete.ErrEvidence
		}
		for _, item := range source.Items {
			key := item.Source + ":" + item.ObjectID
			if item.Source != source.Name || item.ObjectID == "" || len(item.ObjectID) > 512 || !supportdelete.ValidRef(item.IncarnationRef) || seen[key] || len(item.OwnerChannels) == 0 {
				return supportdelete.ErrEvidence
			}
			seen[key] = true
			owners := map[string]bool{}
			target := false
			for _, owner := range item.OwnerChannels {
				if !serviceaccess.ValidChannel(owner) || owners[owner] {
					return supportdelete.ErrEvidence
				}
				owners[owner] = true
				target = target || owner == e.Selector.Target.ChannelID
			}
			if !target || (len(owners) == 1 && item.Disposition != "delete-exclusive") || (len(owners) > 1 && item.Disposition != "remove-target") {
				return supportdelete.ErrEvidence
			}
		}
	}
	return nil
}

type DerivedDeletionInventory interface {
	Capture(context.Context, supportdelete.Operation) (DerivedInventorySnapshot, error)
}
type DerivedDeletionPersistence interface {
	LoadDerivedInventory(context.Context, supportdelete.Execution) (DerivedInventorySnapshot, error)
	CaptureDerivedInventory(context.Context, DerivedInventorySnapshot) error
	CheckDerivedInventory(context.Context, DerivedInventorySnapshot) error
	ConfirmDerivedInventory(context.Context, DerivedInventorySnapshot, bool) error
}

// Observation is a definitive read of the same object incarnation. A missing
// object is valid only if Known is true. Owners of a shared object are checked
// both before and after a target-scoped mutation.
type DerivedObservation struct {
	Known, Exists                    bool
	Source, ObjectID, IncarnationRef string
	OwnerChannels                    []string
}
type DerivedMutation struct {
	OperationID, InventoryRef, TargetChannel string
	Item                                     DerivedItem
}
type DerivedCatalogObservation struct {
	Known, Complete  bool
	InventoryRef     string
	UnexpectedTarget int64
}
type SyntheticDerivedExecutor interface {
	InspectCatalog(context.Context, DerivedInventorySnapshot) (DerivedCatalogObservation, error)
	Observe(context.Context, DerivedItem) (DerivedObservation, error)
	ApplyTarget(context.Context, DerivedMutation) error
}
type DerivedDeletionPermit struct {
	Evidence                                            supportdelete.Evidence
	CatalogLocked, ReplayExcluded, OtherOwnersPreserved bool
}
type DerivedDeletionGuard interface {
	WithGuard(context.Context, supportdelete.Operation, DerivedInventorySnapshot, func(context.Context, DerivedDeletionPermit) error) error
}
type DerivedDeletionEffects struct {
	Store     DerivedDeletionPersistence
	Inventory DerivedDeletionInventory
	Executor  SyntheticDerivedExecutor
	Guard     DerivedDeletionGuard
	Clock     func() time.Time
}

func (a *DerivedDeletionEffects) snapshot(ctx context.Context, op supportdelete.Operation) (DerivedInventorySnapshot, error) {
	s, err := a.Store.LoadDerivedInventory(ctx, op.Execution)
	if errors.Is(err, ErrDerivedInventoryMissing) {
		if op.Execution.Cursor != derivedDeleteCursor() || a.Inventory == nil {
			return s, supportdelete.ErrEvidence
		}
		s, err = a.Inventory.Capture(ctx, op)
		if err != nil || s.Execution != op.Execution || s.Deleted || s.Inspected || s.CapturedAt.Before(op.ObservedAfter) || s.CapturedAt.After(a.Clock()) {
			return s, supportdelete.ErrEvidence
		}
		s.CapturedAt = s.CapturedAt.UTC().Truncate(time.Microsecond)
		s.InventoryRef = derivedInventoryRef(s)
		if s.validate() != nil {
			return s, supportdelete.ErrEvidence
		}
		if err = a.Store.CaptureDerivedInventory(ctx, s); err != nil {
			return s, fmt.Errorf("capture derived inventory: %w", err)
		}
		s, err = a.Store.LoadDerivedInventory(ctx, op.Execution)
	}
	if err != nil {
		return s, fmt.Errorf("load derived inventory: %w", err)
	}
	if s.Execution != op.Execution || s.validate() != nil || s.CapturedAt.After(a.Clock()) {
		return s, supportdelete.ErrConflict
	}
	return s, nil
}

func remainingOwners(item DerivedItem, target string) []string {
	var result []string
	for _, owner := range item.OwnerChannels {
		if owner != target {
			result = append(result, owner)
		}
	}
	return result
}

func checkDerivedObservation(item DerivedItem, target string, o DerivedObservation, before bool) error {
	if !o.Known || o.Source != item.Source || o.ObjectID != item.ObjectID {
		return supportdelete.ErrEvidence
	}
	if !o.Exists {
		if item.Disposition == "remove-target" {
			return supportdelete.ErrEvidence
		}
		return nil
	}
	if o.IncarnationRef != item.IncarnationRef {
		return supportdelete.ErrConflict
	}
	want := remainingOwners(item, target)
	if before && reflect.DeepEqual(o.OwnerChannels, item.OwnerChannels) {
		return nil
	}
	if before && item.Disposition == "delete-exclusive" {
		return supportdelete.ErrEvidence
	}
	if !reflect.DeepEqual(o.OwnerChannels, want) {
		return supportdelete.ErrEvidence
	}
	if !before && item.Disposition == "delete-exclusive" {
		return supportdelete.ErrEvidence
	}
	return nil
}

func (a *DerivedDeletionEffects) verify(ctx context.Context, item DerivedItem, target string, before bool) error {
	o, err := a.Executor.Observe(ctx, item)
	if err != nil {
		return supportdelete.ErrUnavailable
	}
	return checkDerivedObservation(item, target, o, before)
}

func (a *DerivedDeletionEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	if a == nil || a.Store == nil || a.Executor == nil || a.Guard == nil || a.Clock == nil || op.Execution.Validate() != nil || op.Execution.Manifest.Mode == "live" || op.Execution.Cursor >= len(supportdelete.Steps()) || op != supportdelete.NewOperation(op.Execution, op.ObservedAfter) || op.ObservedAfter.Before(op.Execution.UpdatedAt) || op.ObservedAfter.After(a.Clock()) || op.Step.Scope != "derived-vendor" || (op.Step.Action != "delete" && op.Step.Action != "inspect") {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	s, err := a.snapshot(ctx, op)
	if err != nil {
		return supportdelete.Evidence{}, err
	}
	if op.Step.Action == "inspect" && !s.Deleted {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	var evidence supportdelete.Evidence
	called := false
	err = a.Guard.WithGuard(ctx, op, s, func(guardCtx context.Context, permit DerivedDeletionPermit) error {
		if called || !permit.CatalogLocked || !permit.ReplayExcluded || !permit.OtherOwnersPreserved || permit.Evidence.Validate(op, a.Clock()) != nil {
			return supportdelete.ErrEvidence
		}
		called = true
		catalog, err := a.Executor.InspectCatalog(guardCtx, s)
		if err != nil || !catalog.Known || !catalog.Complete || catalog.InventoryRef != s.InventoryRef || catalog.UnexpectedTarget != 0 {
			return supportdelete.ErrEvidence
		}
		for _, source := range s.Sources {
			for _, item := range source.Items {
				if guardCtx.Err() != nil {
					return supportdelete.ErrUnavailable
				}
				if err := a.Store.CheckDerivedInventory(guardCtx, s); err != nil {
					return fmt.Errorf("check derived inventory fence: %w", err)
				}
				if op.Step.Action == "delete" {
					if err := a.verify(guardCtx, item, op.Execution.Selector.Target.ChannelID, true); err != nil {
						return err
					}
					mutation := DerivedMutation{OperationID: digest(s.CaptureOperationID + ":" + s.InventoryRef + ":" + item.Source + ":" + item.ObjectID + ":" + item.IncarnationRef), InventoryRef: s.InventoryRef, TargetChannel: op.Execution.Selector.Target.ChannelID, Item: item}
					mutationErr := a.Executor.ApplyTarget(guardCtx, mutation)
					// A lost acknowledgement is reconciled by the definitive read below.
					if err := a.verify(guardCtx, item, op.Execution.Selector.Target.ChannelID, false); err != nil {
						if mutationErr != nil {
							return fmt.Errorf("mutation outcome unknown: %w", supportdelete.ErrUnavailable)
						}
						return err
					}
					continue
				}
				if err := a.verify(guardCtx, item, op.Execution.Selector.Target.ChannelID, false); err != nil {
					return err
				}
			}
		}
		catalog, err = a.Executor.InspectCatalog(guardCtx, s)
		if err != nil || !catalog.Known || !catalog.Complete || catalog.InventoryRef != s.InventoryRef || catalog.UnexpectedTarget != 0 {
			return supportdelete.ErrEvidence
		}
		for _, source := range s.Sources {
			for _, item := range source.Items {
				if err := a.verify(guardCtx, item, op.Execution.Selector.Target.ChannelID, false); err != nil {
					return err
				}
			}
		}
		if err := a.Store.ConfirmDerivedInventory(guardCtx, s, op.Step.Action == "inspect"); err != nil {
			return fmt.Errorf("confirm derived inventory: %w", err)
		}
		evidence = permit.Evidence
		evidence.ObservedAt = a.Clock()
		return nil
	})
	if err != nil {
		return supportdelete.Evidence{}, fmt.Errorf("guard derived deletion: %w", err)
	}
	if !called || evidence.Validate(op, a.Clock()) != nil {
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	return evidence, nil
}
