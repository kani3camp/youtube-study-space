package mypage

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"app.modules/core/supportdelete"
)

type derivedMemoryStore struct {
	snapshot DerivedInventorySnapshot
	captured bool
}

func (s *derivedMemoryStore) LoadDerivedInventory(_ context.Context, e supportdelete.Execution) (DerivedInventorySnapshot, error) {
	if !s.captured {
		return DerivedInventorySnapshot{}, ErrDerivedInventoryMissing
	}
	if s.snapshot.Execution != e {
		return DerivedInventorySnapshot{}, supportdelete.ErrConflict
	}
	return s.snapshot, nil
}

func (s *derivedMemoryStore) CaptureDerivedInventory(_ context.Context, v DerivedInventorySnapshot) error {
	if s.captured {
		return supportdelete.ErrConflict
	}
	s.snapshot, s.captured = v, true
	return nil
}

func (s *derivedMemoryStore) CheckDerivedInventory(_ context.Context, v DerivedInventorySnapshot) error {
	if !s.captured || !reflect.DeepEqual(s.snapshot, v) {
		return supportdelete.ErrConflict
	}
	return nil
}

func (s *derivedMemoryStore) ConfirmDerivedInventory(_ context.Context, v DerivedInventorySnapshot, inspect bool) error {
	if err := s.CheckDerivedInventory(context.Background(), v); err != nil {
		return err
	}
	if inspect && !s.snapshot.Deleted {
		return supportdelete.ErrEvidence
	}
	s.snapshot.Deleted = true
	s.snapshot.Inspected = s.snapshot.Inspected || inspect
	return nil
}

type derivedObject struct {
	item   DerivedItem
	exists bool
	owners []string
}
type derivedFake struct {
	objects      map[string]*derivedObject
	catalogKnown bool
	unknownRead  int
	unknownAt    int
	readCount    int
	applyCount   map[string]int
	identities   map[string]string
	lostAck      bool
}

func (f *derivedFake) InspectCatalog(_ context.Context, s DerivedInventorySnapshot) (DerivedCatalogObservation, error) {
	return DerivedCatalogObservation{Known: f.catalogKnown, Complete: f.catalogKnown, InventoryRef: s.InventoryRef}, nil
}

func (f *derivedFake) Observe(_ context.Context, item DerivedItem) (DerivedObservation, error) {
	f.readCount++
	if f.readCount == f.unknownAt {
		return DerivedObservation{}, nil
	}
	if f.unknownRead > 0 {
		f.unknownRead--
		return DerivedObservation{}, nil
	}
	o := f.objects[item.ObjectID]
	if o == nil {
		return DerivedObservation{}, nil
	}
	return DerivedObservation{Known: true, Exists: o.exists, Source: item.Source, ObjectID: item.ObjectID, IncarnationRef: o.item.IncarnationRef, OwnerChannels: append([]string(nil), o.owners...)}, nil
}

func (f *derivedFake) ApplyTarget(_ context.Context, m DerivedMutation) error {
	f.applyCount[m.Item.ObjectID]++
	if prior := f.identities[m.Item.ObjectID]; prior != "" && prior != m.OperationID {
		return supportdelete.ErrConflict
	}
	f.identities[m.Item.ObjectID] = m.OperationID
	o := f.objects[m.Item.ObjectID]
	if o == nil || o.item.IncarnationRef != m.Item.IncarnationRef || !supportdelete.ValidRef(m.OperationID) {
		return supportdelete.ErrConflict
	}
	if o.exists {
		if m.Item.Disposition == "delete-exclusive" {
			o.exists = false
			o.owners = nil
		} else {
			var kept []string
			for _, owner := range o.owners {
				if owner != m.TargetChannel {
					kept = append(kept, owner)
				}
			}
			o.owners = kept
		}
	}
	if f.lostAck {
		return supportdelete.ErrUnavailable
	}
	return nil
}

type derivedFixtureInventory struct{ snapshot DerivedInventorySnapshot }

func (i derivedFixtureInventory) Capture(_ context.Context, _ supportdelete.Operation) (DerivedInventorySnapshot, error) {
	return i.snapshot, nil
}

type derivedFixtureGuard struct{ now time.Time }

func (g derivedFixtureGuard) WithGuard(ctx context.Context, op supportdelete.Operation, _ DerivedInventorySnapshot, run func(context.Context, DerivedDeletionPermit) error) error {
	e := op.Execution
	v := supportdelete.Evidence{Selector: e.Selector, ManifestRef: e.Manifest.Ref, OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope, Mode: e.Manifest.Mode, TraceRef: digest("derived-trace"), Generation: e.Generation, Cutoff: e.Cutoff, ObservedAt: g.now, Known: true, Complete: true, RestoreExcluded: true}
	return run(ctx, DerivedDeletionPermit{Evidence: v, CatalogLocked: true, ReplayExcluded: true, OtherOwnersPreserved: true})
}

func derivedFixture() (*DerivedDeletionEffects, *derivedMemoryStore, *derivedFake, supportdelete.Operation) {
	now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	target := supportdelete.Target{Environment: "development", ProjectID: "demo-youtube-study-space-ci", ChannelID: "UCsynthetic0000000000001"}
	e := supportdelete.Execution{Selector: supportdelete.Selector{Target: target, RequestRef: digest("request"), ExecutionRef: digest("execution"), ProofRef: digest("proof")}, Manifest: supportdelete.Manifest{Ref: digest("manifest"), Mode: "mock"}, OwnerRef: digest("owner"), Revision: 1, Generation: 1, GuardSince: now, Cutoff: now, AcceptedAt: now.Add(-time.Hour), DeleteBy: now.Add(-time.Hour).Add(7 * 24 * time.Hour), UpdatedAt: now, Cursor: derivedDeleteCursor(), EvidenceDigest: digest("prior")}
	op := supportdelete.NewOperation(e, now)
	other := "UCsynthetic0000000000002"
	items := []DerivedItem{
		{Source: "openai-response", ObjectID: "synthetic-response", IncarnationRef: digest("openai-incarnation"), Disposition: "delete-exclusive", OwnerChannels: []string{target.ChannelID}},
		{Source: "discord-message", ObjectID: "synthetic-message", IncarnationRef: digest("discord-incarnation"), Disposition: "remove-target", OwnerChannels: []string{target.ChannelID, other}},
	}
	sources := make([]DerivedSource, len(derivedSources))
	for j, name := range derivedSources {
		sources[j] = DerivedSource{Name: name, Complete: true, Supported: true, Items: []DerivedItem{}}
	}
	sources[2].Items = []DerivedItem{items[0]}
	sources[3].Items = []DerivedItem{items[1]}
	s := DerivedInventorySnapshot{SchemaVersion: 1, Execution: e, CaptureOperationID: op.ID, TraceRef: digest("inventory-trace"), CapturedAt: now, Sources: sources}
	s.InventoryRef = derivedInventoryRef(s)
	store := &derivedMemoryStore{}
	fake := &derivedFake{objects: map[string]*derivedObject{}, catalogKnown: true, applyCount: map[string]int{}, identities: map[string]string{}}
	for _, item := range items {
		fake.objects[item.ObjectID] = &derivedObject{item: item, exists: true, owners: append([]string(nil), item.OwnerChannels...)}
	}
	a := &DerivedDeletionEffects{Store: store, Inventory: derivedFixtureInventory{s}, Executor: fake, Guard: derivedFixtureGuard{now}, Clock: func() time.Time { return now }}
	return a, store, fake, op
}

func TestDerivedInventoryRequiresKnownOwnershipAndDisposal(t *testing.T) {
	for name, change := range map[string]func(*DerivedInventorySnapshot){
		"missing source":         func(s *DerivedInventorySnapshot) { s.Sources = s.Sources[:3] },
		"unknown source":         func(s *DerivedInventorySnapshot) { s.Sources[2].Complete = false },
		"unsupported disposal":   func(s *DerivedInventorySnapshot) { s.Sources[3].Supported = false },
		"missing owner":          func(s *DerivedInventorySnapshot) { s.Sources[2].Items[0].OwnerChannels = nil },
		"shared object deletion": func(s *DerivedInventorySnapshot) { s.Sources[3].Items[0].Disposition = "delete-exclusive" },
		"unknown incarnation":    func(s *DerivedInventorySnapshot) { s.Sources[2].Items[0].IncarnationRef = "" },
	} {
		t.Run(name, func(t *testing.T) {
			a, store, fake, op := derivedFixture()
			fixture, ok := a.Inventory.(derivedFixtureInventory)
			if !ok {
				t.Fatal("synthetic inventory missing")
			}
			s := fixture.snapshot
			change(&s)
			s.InventoryRef = derivedInventoryRef(s)
			a.Inventory = derivedFixtureInventory{s}
			if _, err := a.Apply(context.Background(), op); !errors.Is(err, supportdelete.ErrEvidence) || store.captured || len(fake.applyCount) != 0 {
				t.Fatalf("unsafe inventory caused effect: %v", err)
			}
		})
	}
}

func TestDerivedDeleteLostAckRecoveryAndSharedPreservation(t *testing.T) {
	a, store, fake, op := derivedFixture()
	fake.lostAck = true
	v, err := a.Apply(context.Background(), op)
	if err != nil || v.Validate(op, op.ObservedAfter) != nil || !store.snapshot.Deleted {
		t.Fatalf("lost ack was not reconciled: %v", err)
	}
	if fake.objects["synthetic-response"].exists || !reflect.DeepEqual(fake.objects["synthetic-message"].owners, []string{"UCsynthetic0000000000002"}) {
		t.Fatal("target or other owner was not preserved correctly")
	}
	// The checkpoint advances independently of the effect acknowledgement.
	for i, step := range supportdelete.Steps() {
		if step == (supportdelete.Step{Action: "inspect", Scope: "derived-vendor"}) {
			store.snapshot.Execution.Cursor = i
		}
	}
	store.snapshot.Execution.Revision++
	op = supportdelete.NewOperation(store.snapshot.Execution, op.ObservedAfter)
	if _, err := a.Apply(context.Background(), op); err != nil || !store.snapshot.Inspected {
		t.Fatalf("final recheck failed: %v", err)
	}
}

func TestDerivedUnknownOutcomeAndStaleFenceStop(t *testing.T) {
	a, store, fake, op := derivedFixture()
	fake.unknownAt = 4 // second object's post-mutation read, after the first completed
	if _, err := a.Apply(context.Background(), op); !errors.Is(err, supportdelete.ErrEvidence) || store.snapshot.Deleted {
		t.Fatalf("unknown read committed: %v", err)
	}
	if fake.applyCount["synthetic-response"] != 1 || fake.applyCount["synthetic-message"] != 1 {
		t.Fatal("fixture did not reach partial completion")
	}
	if _, err := a.Apply(context.Background(), op); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if fake.applyCount["synthetic-response"] < 2 || fake.identities["synthetic-response"] == "" {
		t.Fatal("retry did not reconcile the same stable operation")
	}
	store.snapshot.Execution.OwnerRef = digest("recovered-owner")
	store.snapshot.Execution.Revision++
	store.snapshot.Execution.Generation++
	store.snapshot.Inspected = false
	if _, err := a.Apply(context.Background(), op); !errors.Is(err, supportdelete.ErrConflict) {
		t.Fatalf("stale owner/revision/generation survived: %v", err)
	}
	if !reflect.DeepEqual(fake.objects["synthetic-message"].owners, []string{"UCsynthetic0000000000002"}) {
		t.Fatal("other owner removed")
	}
}

func TestDerivedUnknownCatalogAndChangedIncarnationStop(t *testing.T) {
	a, store, fake, op := derivedFixture()
	fake.catalogKnown = false
	if _, err := a.Apply(context.Background(), op); !errors.Is(err, supportdelete.ErrEvidence) || store.snapshot.Deleted {
		t.Fatalf("unknown catalog committed: %v", err)
	}
	fake.catalogKnown = true
	fake.objects["synthetic-response"].item.IncarnationRef = digest("replacement")
	if _, err := a.Apply(context.Background(), op); !errors.Is(err, supportdelete.ErrConflict) || store.snapshot.Deleted {
		t.Fatalf("replaced object deleted: %v", err)
	}
}
