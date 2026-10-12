package mypage

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/core/supportdelete"
)

const (
	supportDeleteExecutions = "support-delete-executions"
	supportDeleteClaims     = "support-delete-proof-claims"
)

// FirestoreDeletionStore is an injected adapter. No constructor discovers ADC
// or starts a live workflow. Claim and guard commit together; finalization
// clears only the same request at the freshly fenced generation.
type FirestoreDeletionStore struct{ Client *firestore.Client }

func (s *FirestoreDeletionStore) check(selector supportdelete.Selector) error {
	if s == nil || s.Client == nil || selector.Validate() != nil || !strings.HasPrefix(s.Client.Collection(supportDeleteExecutions).Doc(selector.ExecutionRef).Path, "projects/"+selector.Target.ProjectID+"/databases/(default)/documents/") {
		return supportdelete.ErrUnavailable
	}
	return nil
}

func readDeletionExecution(doc *firestore.DocumentSnapshot) (supportdelete.Execution, error) {
	var e supportdelete.Execution
	if doc == nil || (len(doc.Data()) < 12 || len(doc.Data()) > 14) || doc.DataTo(&e) != nil || e.Validate() != nil {
		return e, supportdelete.ErrUnavailable
	}
	data := doc.Data()
	extras := 0
	if ref, ok := data["bigQueryInventoryRef"]; ok {
		extras++
		value, typeOK := ref.(string)
		if !typeOK || !supportdelete.ValidRef(value) {
			return e, supportdelete.ErrUnavailable
		}
	}
	if ref, ok := data["derivedInventoryRef"]; ok {
		extras++
		value, typeOK := ref.(string)
		if !typeOK || !supportdelete.ValidRef(value) {
			return e, supportdelete.ErrUnavailable
		}
	}
	if len(data) != 12+extras {
		return e, supportdelete.ErrUnavailable
	}
	for _, key := range []string{"selector", "manifest", "ownerRef", "revision", "generation", "guardSince", "cutoff", "acceptedAt", "deleteBy", "updatedAt", "cursor", "evidenceDigest"} {
		if _, ok := data[key]; !ok {
			return e, supportdelete.ErrUnavailable
		}
	}
	selector, ok := data["selector"].(map[string]interface{})
	if !ok || len(selector) != 4 {
		return e, supportdelete.ErrUnavailable
	}
	for _, key := range []string{"target", "requestRef", "executionRef", "proofRef"} {
		if _, ok := selector[key]; !ok {
			return e, supportdelete.ErrUnavailable
		}
	}
	target, ok := selector["target"].(map[string]interface{})
	if !ok || len(target) != 3 {
		return e, supportdelete.ErrUnavailable
	}
	for _, key := range []string{"environment", "projectID", "channelID"} {
		if _, ok := target[key]; !ok {
			return e, supportdelete.ErrUnavailable
		}
	}
	manifest, ok := data["manifest"].(map[string]interface{})
	if !ok || len(manifest) != 2 {
		return e, supportdelete.ErrUnavailable
	}
	for _, key := range []string{"ref", "mode"} {
		if _, ok := manifest[key]; !ok {
			return e, supportdelete.ErrUnavailable
		}
	}
	return e, nil
}

func deletionComplete(doc *firestore.DocumentSnapshot) bool {
	if doc == nil || (len(doc.Data()) != 4 && len(doc.Data()) != 5) {
		return false
	}
	d := doc.Data()
	version, ok := d["schemaVersion"].(int64)
	when, timeOK := d["completedAt"].(time.Time)
	accepted, acceptedOK := d["acceptedAt"].(time.Time)
	deadline, deadlineOK := d["deleteBy"].(time.Time)
	return ok && version == 1 && timeOK && !when.IsZero() && acceptedOK && !accepted.IsZero() && deadlineOK && deadline.Equal(accepted.Add(7*24*time.Hour)) && !when.Before(accepted)
}

func (s *FirestoreDeletionStore) Acquire(ctx context.Context, start supportdelete.Start) (supportdelete.Execution, error) {
	if s.check(start.Selector) != nil || start.Manifest.Validate() != nil || !supportdelete.ValidRef(start.OwnerRef) || start.Now.IsZero() {
		return supportdelete.Execution{}, supportdelete.ErrInvalid
	}
	var result supportdelete.Execution
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		execRef := s.Client.Collection(supportDeleteExecutions).Doc(start.Selector.ExecutionRef)
		doc, err := tx.Get(execRef)
		if err == nil {
			if deletionComplete(doc) {
				if doc.Data()["completionBinding"] != completionBinding(start.Selector, start.Manifest) {
					return supportdelete.ErrConflict
				}
				return supportdelete.ErrCompleted
			}
			e, readErr := readDeletionExecution(doc)
			if readErr != nil {
				return readErr
			}
			if e.Selector != start.Selector || e.Manifest != start.Manifest {
				return supportdelete.ErrConflict
			}
			if e.OwnerRef != start.OwnerRef {
				return supportdelete.ErrBusy
			}
			result = e
			return nil
		}
		if status.Code(err) != codes.NotFound {
			return supportdelete.ErrUnavailable
		}
		selector := start.Selector
		receiptDoc, err := tx.Get(s.Client.Collection("support-requests").Doc(selector.RequestRef))
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		var receipt SupportRequest
		dry := DryRunSelector{Environment: selector.Target.Environment, ExpectedProject: selector.Target.ProjectID, RequestRef: selector.RequestRef, Purpose: SupportDelete, Now: start.Now}
		if receiptDoc.DataTo(&receipt) != nil || validateDryRunReceipt(receipt, dry) != nil || receipt.TargetChannel != selector.Target.ChannelID || receipt.ProofRef != selector.ProofRef {
			return supportdelete.ErrConflict
		}
		oauthDoc, err := tx.Get(s.Client.Collection("oauth-transactions").Doc(receipt.OAuthTransactionID))
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		var oauth OAuthTransaction
		if oauthDoc.DataTo(&oauth) != nil || validateDryRunProof(receipt, oauth, dry) != nil {
			return supportdelete.ErrConflict
		}
		index, err := tx.Get(s.Client.Collection("support-request-ids").Doc(digest(receipt.Environment + ":" + receipt.RequestID)))
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		var binding supportChallengeIndex
		if index.DataTo(&binding) != nil || binding.RequestRef != selector.RequestRef || binding.Environment != selector.Target.Environment {
			return supportdelete.ErrConflict
		}
		claimRef := s.Client.Collection(supportDeleteClaims).Doc(selector.ProofRef)
		_, err = tx.Get(claimRef)
		if status.Code(err) != codes.NotFound {
			if err == nil {
				return supportdelete.ErrConflict
			}
			return supportdelete.ErrUnavailable
		}
		controls := &serviceaccess.FirestoreStore{Client: s.Client}
		old, err := controls.ReadTransaction(tx, selector.Target.ChannelID)
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		if old.Control.PrivacyDeletion.Active {
			return supportdelete.ErrConflict
		}
		next, _, err := serviceaccess.Transition(old, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: true, ReasonCode: "PRIVACY_DELETION", Reference: selector.RequestRef}, start.Now)
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		result = supportdelete.Execution{Selector: selector, Manifest: start.Manifest, OwnerRef: start.OwnerRef, Revision: 1, Generation: next.Generation, GuardSince: next.PrivacyDeletion.Since, Cutoff: next.PrivacyDeletion.Since, AcceptedAt: receipt.AcceptedAt, DeleteBy: *receipt.DeleteBy, UpdatedAt: start.Now.UTC().Truncate(time.Microsecond), EvidenceDigest: digest(selector.ExecutionRef + ":" + selector.ProofRef)}
		if result.Validate() != nil {
			return supportdelete.ErrInvalid
		}
		if tx.Create(claimRef, map[string]interface{}{"executionRef": selector.ExecutionRef, "requestRef": selector.RequestRef}) != nil || tx.Create(execRef, result) != nil || tx.Set(s.Client.Collection(serviceaccess.Collection).Doc(selector.Target.ChannelID), next) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return result, deletionStoreError(err)
}

func deletionStoreError(err error) error {
	if err == nil {
		return nil
	}
	for _, v := range []error{supportdelete.ErrCompleted, supportdelete.ErrConflict, supportdelete.ErrBusy, supportdelete.ErrInvalid} {
		if errors.Is(err, v) {
			return v
		}
	}
	return supportdelete.ErrUnavailable
}

func (s *FirestoreDeletionStore) readFence(tx *firestore.Transaction, expected supportdelete.Execution, exactGeneration bool) (supportdelete.Execution, serviceaccess.Snapshot, error) {
	doc, err := tx.Get(s.Client.Collection(supportDeleteExecutions).Doc(expected.Selector.ExecutionRef))
	if err != nil {
		return supportdelete.Execution{}, serviceaccess.Snapshot{}, supportdelete.ErrUnavailable
	}
	e, err := readDeletionExecution(doc)
	if err != nil {
		return e, serviceaccess.Snapshot{}, err
	}
	if e != expected {
		return e, serviceaccess.Snapshot{}, supportdelete.ErrConflict
	}
	control, err := (&serviceaccess.FirestoreStore{Client: s.Client}).ReadTransaction(tx, e.Selector.Target.ChannelID)
	if err != nil || !control.Exists || !control.Control.PrivacyDeletion.Active || control.Control.PrivacyDeletion.RequestRef != e.Selector.RequestRef || !control.Control.PrivacyDeletion.Since.Equal(e.GuardSince) || control.Control.Generation < e.Generation || (exactGeneration && control.Control.Generation != e.Generation) {
		return e, control, supportdelete.ErrConflict
	}
	return e, control, nil
}

func (s *FirestoreDeletionStore) Refresh(ctx context.Context, expected supportdelete.Execution, now time.Time) (supportdelete.Execution, error) {
	if s.check(expected.Selector) != nil || expected.Validate() != nil || now.Before(expected.UpdatedAt) {
		return supportdelete.Execution{}, supportdelete.ErrInvalid
	}
	var result supportdelete.Execution
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		e, control, err := s.readFence(tx, expected, false)
		if err != nil {
			return err
		}
		result = e
		if control.Control.Generation == e.Generation {
			return nil
		}
		if e.Revision == math.MaxInt64 {
			return supportdelete.ErrConflict
		}
		result.Generation = control.Control.Generation
		result.Revision++
		result.UpdatedAt = now.UTC().Truncate(time.Microsecond)
		// An independent moderation change invalidates partial absence evidence.
		// Durable resume intent is irreversible: new legitimate use must survive.
		if result.Cursor > supportdelete.InspectionStart() && result.Cursor < supportdelete.ResumeStart() {
			result.Cursor = supportdelete.InspectionStart()
		}
		if err := s.moveDeletionSnapshots(tx, e, result, false); err != nil {
			return err
		}
		if tx.Set(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef), result, mergeDeletionExecution()) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return result, deletionStoreError(err)
}

func (s *FirestoreDeletionStore) Commit(ctx context.Context, expected supportdelete.Execution, evidence supportdelete.Evidence, now time.Time) (supportdelete.Execution, error) {
	if s.check(expected.Selector) != nil || expected.Validate() != nil || now.Before(expected.UpdatedAt) || expected.Cursor >= len(supportdelete.Steps()) || expected.Revision == math.MaxInt64 {
		return supportdelete.Execution{}, supportdelete.ErrInvalid
	}
	if evidence.Validate(supportdelete.NewOperation(expected, expected.UpdatedAt), now) != nil {
		return supportdelete.Execution{}, supportdelete.ErrEvidence
	}
	var result supportdelete.Execution
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		e, _, err := s.readFence(tx, expected, true)
		if err != nil {
			return err
		}
		result = e
		result.Cursor++
		result.Revision++
		result.EvidenceDigest = supportdelete.EvidenceDigest(e.EvidenceDigest, evidence)
		result.UpdatedAt = now.UTC().Truncate(time.Microsecond)
		if err := s.moveDeletionSnapshots(tx, e, result, false); err != nil {
			return err
		}
		if tx.Set(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef), result, mergeDeletionExecution()) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return result, deletionStoreError(err)
}

func (s *FirestoreDeletionStore) Recover(ctx context.Context, proof supportdelete.Recovery, now time.Time) error {
	// This method accepts only evidence from a trusted runtime adapter. The CLI
	// has no recovery JSON flag that can manufacture Stopped/TraceRef evidence.
	if s.check(proof.Selector) != nil || !proof.Stopped || !supportdelete.ValidRef(proof.TraceRef) || !supportdelete.ValidRef(proof.PreviousOwnerRef) || !supportdelete.ValidRef(proof.NewOwnerRef) || proof.PreviousOwnerRef == proof.NewOwnerRef || proof.ObservedAt.IsZero() || proof.ObservedAt.After(now) || proof.ObservedAt.Before(now.Add(-time.Minute)) {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(s.Client.Collection(supportDeleteExecutions).Doc(proof.Selector.ExecutionRef))
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		e, err := readDeletionExecution(doc)
		if err != nil {
			return err
		}
		if e.Selector != proof.Selector || e.OwnerRef != proof.PreviousOwnerRef || e.Manifest.Ref != proof.ManifestRef || e.Revision != proof.Revision || e.Generation != proof.Generation || e.Revision == math.MaxInt64 || now.Before(e.UpdatedAt) || proof.ObservedAt.Before(e.UpdatedAt) {
			return supportdelete.ErrConflict
		}
		if _, _, err = s.readFence(tx, e, true); err != nil {
			return err
		}
		previous := e
		e.OwnerRef = proof.NewOwnerRef
		e.Revision++
		e.UpdatedAt = now.UTC().Truncate(time.Microsecond)
		if err := s.moveDeletionSnapshots(tx, previous, e, false); err != nil {
			return err
		}
		if tx.Set(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef), e, mergeDeletionExecution()) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) Finalize(ctx context.Context, expected supportdelete.Execution, now time.Time) error {
	if s.check(expected.Selector) != nil || expected.Validate() != nil || expected.Cursor != len(supportdelete.Steps()) || now.Before(expected.UpdatedAt) {
		return supportdelete.ErrInvalid
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		e, control, err := s.readFence(tx, expected, true)
		if err != nil {
			return err
		}
		receiptRef := s.Client.Collection("support-requests").Doc(e.Selector.RequestRef)
		doc, err := tx.Get(receiptRef)
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		var receipt SupportRequest
		dry := DryRunSelector{Environment: e.Selector.Target.Environment, ExpectedProject: e.Selector.Target.ProjectID, RequestRef: e.Selector.RequestRef, Purpose: SupportDelete, Now: now}
		if doc.DataTo(&receipt) != nil || validateDryRunReceipt(receipt, dry) != nil || receipt.TargetChannel != e.Selector.Target.ChannelID || receipt.ProofRef != e.Selector.ProofRef || !receipt.AcceptedAt.Equal(e.AcceptedAt) || !receipt.DeleteBy.Equal(e.DeleteBy) {
			return supportdelete.ErrConflict
		}
		claim, err := tx.Get(s.Client.Collection(supportDeleteClaims).Doc(e.Selector.ProofRef))
		if err != nil || len(claim.Data()) != 2 || claim.Data()["executionRef"] != e.Selector.ExecutionRef || claim.Data()["requestRef"] != e.Selector.RequestRef {
			return supportdelete.ErrConflict
		}
		next, _, err := serviceaccess.Transition(control, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: false}, now)
		if err != nil {
			return supportdelete.ErrConflict
		}
		// Minimal opaque terminal receipts carry no channel/proof/request links.
		// Inactive control checkpoints are retained without TTL/reset/deletion.
		terminal := map[string]interface{}{"schemaVersion": int64(1), "acceptedAt": e.AcceptedAt, "deleteBy": e.DeleteBy, "completedAt": now.UTC().Truncate(time.Microsecond)}
		if err := s.moveDeletionSnapshots(tx, e, e, true); err != nil {
			return err
		}
		for _, ref := range []*firestore.DocumentRef{s.Client.Collection("oauth-transactions").Doc(receipt.OAuthTransactionID), s.Client.Collection("support-request-ids").Doc(digest(receipt.Environment + ":" + receipt.RequestID)), s.Client.Collection(supportDeleteClaims).Doc(e.Selector.ProofRef)} {
			if tx.Delete(ref) != nil {
				return supportdelete.ErrUnavailable
			}
		}
		if tx.Set(receiptRef, terminal) != nil || tx.Set(s.Client.Collection(supportDeleteExecutions).Doc(e.Selector.ExecutionRef), map[string]interface{}{"schemaVersion": int64(1), "acceptedAt": e.AcceptedAt, "deleteBy": e.DeleteBy, "completedAt": now.UTC().Truncate(time.Microsecond), "completionBinding": completionBinding(e.Selector, e.Manifest)}) != nil || tx.Set(s.Client.Collection(serviceaccess.Collection).Doc(e.Selector.Target.ChannelID), next) != nil {
			return supportdelete.ErrUnavailable
		}
		// The existing deletion engine alone owns this terminal event. Keep it
		// body-free and unlinkable by raw channel, proof or receipt reference.
		if tx.Create(s.Client.Collection(supportOperatorAudit).Doc(digest("delete-finalized:"+e.Selector.ExecutionRef)), operatorAuditEvent{
			SchemaVersion: 1, Environment: e.Selector.Target.Environment, Purpose: SupportDelete,
			Action: "delete-finalized", Actor: "deletion-workflow", Binding: digest(e.Selector.RequestRef),
			Fingerprint: completionBinding(e.Selector, e.Manifest), At: now.UTC().Truncate(time.Microsecond),
		}) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return deletionStoreError(err)
}

func completionBinding(selector supportdelete.Selector, manifest supportdelete.Manifest) string {
	return digest(selector.Target.Environment + ":" + selector.Target.ProjectID + ":" + selector.Target.ChannelID + ":" + selector.RequestRef + ":" + selector.ExecutionRef + ":" + selector.ProofRef + ":" + manifest.Ref + ":" + manifest.Mode)
}

// Preserve the optional immutable BQ capture marker across ordinary checkpoint
// updates. Terminal replacement deliberately removes it with all raw links.
func mergeDeletionExecution() firestore.SetOption {
	var paths []firestore.FieldPath
	for _, key := range []string{"selector", "manifest", "ownerRef", "revision", "generation", "guardSince", "cutoff", "acceptedAt", "deleteBy", "updatedAt", "cursor", "evidenceDigest"} {
		paths = append(paths, firestore.FieldPath{key})
	}
	return firestore.Merge(paths...)
}
