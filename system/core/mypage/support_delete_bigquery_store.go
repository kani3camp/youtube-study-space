package mypage

import (
	"context"
	"reflect"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/supportdelete"
)

const (
	supportDeleteBigQueryInventory = "support-delete-bigquery-inventory"
	supportDeleteBigQueryBinding   = "support-delete-bigquery-bindings"
)

func readBigQueryInventory(doc *firestore.DocumentSnapshot) (BigQueryInventorySnapshot, error) {
	var s BigQueryInventorySnapshot
	keys := []string{"schemaVersion", "execution", "captureOperationID", "traceRef", "capturedAt", "complete", "location", "tables", "sourceJobs", "externalCopyRefs", "inventoryRef", "deleted", "inspected"}
	if doc == nil || len(doc.Data()) != len(keys) || doc.DataTo(&s) != nil || s.validate() != nil {
		return s, supportdelete.ErrEvidence
	}
	for _, key := range keys {
		if _, ok := doc.Data()[key]; !ok {
			return s, supportdelete.ErrEvidence
		}
	}
	for _, field := range []struct {
		key  string
		keys []string
	}{
		{"tables", []string{"projectID", "location", "datasetID", "tableID", "role", "targetField", "exists", "schemaRef", "incarnationRef"}},
		{"sourceJobs", []string{"projectID", "location", "jobID", "configurationRef"}},
	} {
		if doc.Data()[field.key] == nil {
			continue
		}
		items, ok := doc.Data()[field.key].([]interface{})
		if !ok {
			return s, supportdelete.ErrEvidence
		}
		for _, item := range items {
			data, ok := item.(map[string]interface{})
			if !ok || len(data) != len(field.keys) {
				return s, supportdelete.ErrEvidence
			}
			for _, key := range field.keys {
				if _, ok := data[key]; !ok {
					return s, supportdelete.ErrEvidence
				}
			}
		}
	}
	return s, nil
}

func bigQueryBinding(s BigQueryInventorySnapshot) map[string]interface{} {
	return map[string]interface{}{"inventoryRef": s.InventoryRef, "captureOperationID": s.CaptureOperationID, "executionBinding": completionBinding(s.Execution.Selector, s.Execution.Manifest) + ":" + s.Execution.Cutoff.String()}
}

// The original OAuth may expire via TTL, but its durable claim, request and
// canonical index must still belong to this exact deletion until finalization.
func (s *FirestoreDeletionStore) checkBigQueryRequest(tx *firestore.Transaction, e supportdelete.Execution) error {
	claim, err := tx.Get(s.Client.Collection(supportDeleteClaims).Doc(e.Selector.ProofRef))
	if err != nil || len(claim.Data()) != 2 || claim.Data()["executionRef"] != e.Selector.ExecutionRef || claim.Data()["requestRef"] != e.Selector.RequestRef {
		return supportdelete.ErrConflict
	}
	doc, err := tx.Get(s.Client.Collection("support-requests").Doc(e.Selector.RequestRef))
	var receipt SupportRequest
	dry := DryRunSelector{Environment: e.Selector.Target.Environment, ExpectedProject: e.Selector.Target.ProjectID, RequestRef: e.Selector.RequestRef, Purpose: SupportDelete, Now: e.UpdatedAt}
	if err != nil || doc.DataTo(&receipt) != nil || validateDryRunReceipt(receipt, dry) != nil || receipt.TargetChannel != e.Selector.Target.ChannelID || receipt.ProofRef != e.Selector.ProofRef || !receipt.AcceptedAt.Equal(e.AcceptedAt) || !receipt.DeleteBy.Equal(e.DeleteBy) {
		return supportdelete.ErrConflict
	}
	index, err := tx.Get(s.Client.Collection("support-request-ids").Doc(digest(receipt.Environment + ":" + receipt.RequestID)))
	var binding supportChallengeIndex
	if err != nil || index.DataTo(&binding) != nil || binding.RequestRef != e.Selector.RequestRef || binding.Environment != e.Selector.Target.Environment {
		return supportdelete.ErrConflict
	}
	return nil
}

func (s *FirestoreDeletionStore) readBigQueryInventory(tx *firestore.Transaction, e supportdelete.Execution) (BigQueryInventorySnapshot, error) {
	executionDoc, executionErr := tx.Get(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef))
	marker, markerErr := tx.Get(s.Client.Collection(supportDeleteBigQueryBinding).Doc(e.Selector.ExecutionRef))
	doc, err := tx.Get(s.Client.Collection(supportDeleteBigQueryInventory).Doc(e.Selector.ExecutionRef))
	if executionErr != nil {
		return BigQueryInventorySnapshot{}, supportdelete.ErrUnavailable
	}
	ref, captured := executionDoc.Data()["bigQueryInventoryRef"]
	if !captured && status.Code(err) == codes.NotFound && status.Code(markerErr) == codes.NotFound {
		return BigQueryInventorySnapshot{}, ErrBigQueryInventoryMissing
	}
	// A missing/different half never permits recapture after partial effects.
	if err != nil || markerErr != nil {
		return BigQueryInventorySnapshot{}, supportdelete.ErrConflict
	}
	current, err := readBigQueryInventory(doc)
	if err != nil || current.Execution != e || !captured || ref != current.InventoryRef || !reflect.DeepEqual(marker.Data(), bigQueryBinding(current)) {
		return current, supportdelete.ErrConflict
	}
	return current, nil
}

func (s *FirestoreDeletionStore) LoadBigQueryInventory(ctx context.Context, e supportdelete.Execution) (BigQueryInventorySnapshot, error) {
	if s.check(e.Selector) != nil || e.Validate() != nil {
		return BigQueryInventorySnapshot{}, supportdelete.ErrUnavailable
	}
	var result BigQueryInventorySnapshot
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if _, _, err := s.readFence(tx, e, true); err != nil {
			return err
		}
		var err error
		result, err = s.readBigQueryInventory(tx, e)
		return err
	})
	if err == ErrBigQueryInventoryMissing {
		return result, ErrBigQueryInventoryMissing
	}
	return result, deletionStoreError(err)
}

func (s *FirestoreDeletionStore) CaptureBigQueryInventory(ctx context.Context, snapshot BigQueryInventorySnapshot) error {
	e := snapshot.Execution
	if s.check(e.Selector) != nil || snapshot.validate() != nil || e.Cursor != bigQueryDeleteCursor() || snapshot.Deleted || snapshot.Inspected {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if _, _, err := s.readFence(tx, e, true); err != nil {
			return err
		}
		old, err := s.readBigQueryInventory(tx, e)
		if err == nil {
			if !reflect.DeepEqual(old, snapshot) {
				return supportdelete.ErrConflict
			}
			return nil
		}
		if err != ErrBigQueryInventoryMissing {
			return err
		}
		if err := s.checkBigQueryRequest(tx, e); err != nil {
			return err
		}
		if tx.Create(s.Client.Collection(supportDeleteBigQueryInventory).Doc(e.Selector.ExecutionRef), snapshot) != nil || tx.Create(s.Client.Collection(supportDeleteBigQueryBinding).Doc(e.Selector.ExecutionRef), bigQueryBinding(snapshot)) != nil || tx.Update(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef), []firestore.Update{{Path: "bigQueryInventoryRef", Value: snapshot.InventoryRef}}) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) currentBigQueryInventory(tx *firestore.Transaction, expected BigQueryInventorySnapshot) (BigQueryInventorySnapshot, error) {
	if _, _, err := s.readFence(tx, expected.Execution, true); err != nil {
		return BigQueryInventorySnapshot{}, err
	}
	current, err := s.readBigQueryInventory(tx, expected.Execution)
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

func (s *FirestoreDeletionStore) CheckBigQueryInventory(ctx context.Context, expected BigQueryInventorySnapshot) error {
	if s.check(expected.Execution.Selector) != nil || expected.validate() != nil {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		_, err := s.currentBigQueryInventory(tx, expected)
		return err
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) ConfirmBigQueryInventory(ctx context.Context, expected BigQueryInventorySnapshot, inspect bool) error {
	if s.check(expected.Execution.Selector) != nil || expected.validate() != nil || expected.Execution.Cursor >= len(supportdelete.Steps()) || supportdelete.Steps()[expected.Execution.Cursor] != (supportdelete.Step{Action: map[bool]string{true: "inspect", false: "delete"}[inspect], Scope: "bigquery"}) {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		current, err := s.currentBigQueryInventory(tx, expected)
		if err != nil {
			return err
		}
		if inspect && !current.Deleted {
			return supportdelete.ErrEvidence
		}
		current.Deleted = true
		current.Inspected = current.Inspected || inspect
		if tx.Set(s.Client.Collection(supportDeleteBigQueryInventory).Doc(expected.Execution.Selector.ExecutionRef), current) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) prepareBigQueryInventory(tx *firestore.Transaction, old, next supportdelete.Execution, terminal bool) (func() error, error) {
	snapshot, err := s.readBigQueryInventory(tx, old)
	if err == ErrBigQueryInventoryMissing {
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
	if next.Cursor > old.Cursor && old.Cursor < len(supportdelete.Steps()) && supportdelete.Steps()[old.Cursor].Scope == "bigquery" {
		if !snapshot.Deleted || (supportdelete.Steps()[old.Cursor].Action == "inspect" && !snapshot.Inspected) {
			return nil, supportdelete.ErrEvidence
		}
	}
	ref := s.Client.Collection(supportDeleteBigQueryInventory).Doc(old.Selector.ExecutionRef)
	if terminal {
		return func() error {
			if tx.Delete(ref) != nil || tx.Delete(s.Client.Collection(supportDeleteBigQueryBinding).Doc(old.Selector.ExecutionRef)) != nil {
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

// Read every temporary snapshot before scheduling any transaction writes.
// Firestore forbids reads after writes, including another adapter's snapshot.
func (s *FirestoreDeletionStore) moveDeletionSnapshots(tx *firestore.Transaction, old, next supportdelete.Execution, terminal bool) error {
	authMove, err := s.prepareAuthOwnership(tx, old, next, terminal)
	if err != nil {
		return err
	}
	bqMove, err := s.prepareBigQueryInventory(tx, old, next, terminal)
	if err != nil {
		return err
	}
	derivedMove, err := s.prepareDerivedInventory(tx, old, next, terminal)
	if err != nil {
		return err
	}
	if err = authMove(); err != nil {
		return err
	}
	if err = bqMove(); err != nil {
		return err
	}
	return derivedMove()
}
