package mypage

import (
	"context"
	"fmt"
	"reflect"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/supportdelete"
)

const (
	supportDeleteDerivedInventory = "support-delete-derived-inventory"
	supportDeleteDerivedBinding   = "support-delete-derived-bindings"
)

func derivedBinding(s DerivedInventorySnapshot) map[string]interface{} {
	return map[string]interface{}{"inventoryRef": s.InventoryRef, "captureOperationID": s.CaptureOperationID, "executionBinding": completionBinding(s.Execution.Selector, s.Execution.Manifest) + ":" + s.Execution.Cutoff.String()}
}

func readDerivedInventory(doc *firestore.DocumentSnapshot) (DerivedInventorySnapshot, error) {
	var s DerivedInventorySnapshot
	keys := []string{"schemaVersion", "execution", "captureOperationID", "traceRef", "inventoryRef", "capturedAt", "sources", "deleted", "inspected"}
	if doc == nil || len(doc.Data()) != len(keys) || doc.DataTo(&s) != nil || s.validate() != nil {
		return s, supportdelete.ErrEvidence
	}
	for _, key := range keys {
		if _, ok := doc.Data()[key]; !ok {
			return s, supportdelete.ErrEvidence
		}
	}
	items, ok := doc.Data()["sources"].([]interface{})
	if !ok || len(items) != len(derivedSources) {
		return s, supportdelete.ErrEvidence
	}
	for _, raw := range items {
		m, ok := raw.(map[string]interface{})
		if !ok || len(m) != 4 {
			return s, supportdelete.ErrEvidence
		}
		for _, key := range []string{"name", "complete", "supported", "items"} {
			if _, ok := m[key]; !ok {
				return s, supportdelete.ErrEvidence
			}
		}
		if m["items"] == nil {
			continue
		}
		children, ok := m["items"].([]interface{})
		if !ok {
			return s, supportdelete.ErrEvidence
		}
		for _, child := range children {
			c, ok := child.(map[string]interface{})
			if !ok || len(c) != 5 {
				return s, supportdelete.ErrEvidence
			}
			for _, key := range []string{"source", "objectID", "incarnationRef", "disposition", "ownerChannels"} {
				if _, ok := c[key]; !ok {
					return s, supportdelete.ErrEvidence
				}
			}
		}
	}
	return s, nil
}

func (s *FirestoreDeletionStore) readDerivedInventory(tx *firestore.Transaction, e supportdelete.Execution) (DerivedInventorySnapshot, error) {
	executionDoc, executionErr := tx.Get(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef))
	marker, markerErr := tx.Get(s.Client.Collection(supportDeleteDerivedBinding).Doc(e.Selector.ExecutionRef))
	doc, err := tx.Get(s.Client.Collection(supportDeleteDerivedInventory).Doc(e.Selector.ExecutionRef))
	if executionErr != nil {
		return DerivedInventorySnapshot{}, supportdelete.ErrUnavailable
	}
	ref, captured := executionDoc.Data()["derivedInventoryRef"]
	if !captured && status.Code(err) == codes.NotFound && status.Code(markerErr) == codes.NotFound {
		return DerivedInventorySnapshot{}, ErrDerivedInventoryMissing
	}
	if err != nil || markerErr != nil {
		return DerivedInventorySnapshot{}, supportdelete.ErrConflict
	}
	current, err := readDerivedInventory(doc)
	if err != nil || current.Execution != e || !captured || ref != current.InventoryRef || !reflect.DeepEqual(marker.Data(), derivedBinding(current)) {
		return current, supportdelete.ErrConflict
	}
	return current, nil
}

func (s *FirestoreDeletionStore) LoadDerivedInventory(ctx context.Context, e supportdelete.Execution) (DerivedInventorySnapshot, error) {
	if s.check(e.Selector) != nil || e.Validate() != nil {
		return DerivedInventorySnapshot{}, supportdelete.ErrUnavailable
	}
	var result DerivedInventorySnapshot
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if _, _, err := s.readFence(tx, e, true); err != nil {
			return err
		}
		var err error
		result, err = s.readDerivedInventory(tx, e)
		return err
	})
	if err == ErrDerivedInventoryMissing {
		return result, fmt.Errorf("derived inventory absent: %w", err)
	}
	return result, deletionStoreError(err)
}

func (s *FirestoreDeletionStore) CaptureDerivedInventory(ctx context.Context, snapshot DerivedInventorySnapshot) error {
	e := snapshot.Execution
	if s.check(e.Selector) != nil || snapshot.validate() != nil || e.Cursor != derivedDeleteCursor() || snapshot.Deleted || snapshot.Inspected {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if _, _, err := s.readFence(tx, e, true); err != nil {
			return err
		}
		old, err := s.readDerivedInventory(tx, e)
		if err == nil {
			if !reflect.DeepEqual(old, snapshot) {
				return supportdelete.ErrConflict
			}
			return nil
		}
		if err != ErrDerivedInventoryMissing {
			return err
		}
		if err := s.checkBigQueryRequest(tx, e); err != nil {
			return err
		}
		if tx.Create(s.Client.Collection(supportDeleteDerivedInventory).Doc(e.Selector.ExecutionRef), snapshot) != nil || tx.Create(s.Client.Collection(supportDeleteDerivedBinding).Doc(e.Selector.ExecutionRef), derivedBinding(snapshot)) != nil || tx.Update(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef), []firestore.Update{{Path: "derivedInventoryRef", Value: snapshot.InventoryRef}}) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) currentDerivedInventory(tx *firestore.Transaction, expected DerivedInventorySnapshot) (DerivedInventorySnapshot, error) {
	if _, _, err := s.readFence(tx, expected.Execution, true); err != nil {
		return DerivedInventorySnapshot{}, err
	}
	current, err := s.readDerivedInventory(tx, expected.Execution)
	if err != nil {
		return current, err
	}
	if err := s.checkBigQueryRequest(tx, expected.Execution); err != nil {
		return current, err
	}
	comparison := current
	comparison.Deleted, comparison.Inspected = expected.Deleted, expected.Inspected
	if !reflect.DeepEqual(comparison, expected) || (expected.Deleted && !current.Deleted) || (expected.Inspected && !current.Inspected) {
		return current, supportdelete.ErrConflict
	}
	return current, nil
}

func (s *FirestoreDeletionStore) CheckDerivedInventory(ctx context.Context, expected DerivedInventorySnapshot) error {
	if s.check(expected.Execution.Selector) != nil || expected.validate() != nil {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		_, err := s.currentDerivedInventory(tx, expected)
		return err
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) ConfirmDerivedInventory(ctx context.Context, expected DerivedInventorySnapshot, inspect bool) error {
	if s.check(expected.Execution.Selector) != nil || expected.validate() != nil || expected.Execution.Cursor >= len(supportdelete.Steps()) || supportdelete.Steps()[expected.Execution.Cursor] != (supportdelete.Step{Action: map[bool]string{true: "inspect", false: "delete"}[inspect], Scope: "derived-vendor"}) {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		current, err := s.currentDerivedInventory(tx, expected)
		if err != nil {
			return err
		}
		if inspect && !current.Deleted {
			return supportdelete.ErrEvidence
		}
		current.Deleted = true
		current.Inspected = current.Inspected || inspect
		if tx.Set(s.Client.Collection(supportDeleteDerivedInventory).Doc(expected.Execution.Selector.ExecutionRef), current) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) prepareDerivedInventory(tx *firestore.Transaction, old, next supportdelete.Execution, terminal bool) (func() error, error) {
	snapshot, err := s.readDerivedInventory(tx, old)
	if err == ErrDerivedInventoryMissing {
		// Before the first derived effect there is legitimately no snapshot.
		// Once its delete step advances, every later checkpoint (including
		// recovery and finalization) must retain that captured inventory.
		if terminal || old.Cursor > derivedDeleteCursor() || (old.Cursor == derivedDeleteCursor() && next.Cursor > old.Cursor) {
			return nil, supportdelete.ErrEvidence
		}
		return func() error { return nil }, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.checkBigQueryRequest(tx, old); err != nil {
		return nil, err
	}
	if terminal && (!snapshot.Deleted || !snapshot.Inspected) {
		return nil, supportdelete.ErrEvidence
	}
	if next.Cursor > old.Cursor && old.Cursor < len(supportdelete.Steps()) && supportdelete.Steps()[old.Cursor].Scope == "derived-vendor" && (!snapshot.Deleted || (supportdelete.Steps()[old.Cursor].Action == "inspect" && !snapshot.Inspected)) {
		return nil, supportdelete.ErrEvidence
	}
	ref := s.Client.Collection(supportDeleteDerivedInventory).Doc(old.Selector.ExecutionRef)
	if terminal {
		return func() error {
			if tx.Delete(ref) != nil || tx.Delete(s.Client.Collection(supportDeleteDerivedBinding).Doc(old.Selector.ExecutionRef)) != nil {
				return supportdelete.ErrUnavailable
			}
			return nil
		}, nil
	}
	snapshot.Execution = next
	if next.Generation != old.Generation && next.Cursor < supportdelete.ResumeStart() {
		snapshot.Inspected = false
	}
	if snapshot.validate() != nil {
		return nil, supportdelete.ErrEvidence
	}
	return func() error {
		if tx.Set(ref, snapshot) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	}, nil
}
