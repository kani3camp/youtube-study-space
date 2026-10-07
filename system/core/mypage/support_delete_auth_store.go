package mypage

import (
	"context"
	"encoding/json"
	"reflect"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/supportdelete"
)

const supportDeleteAuthOwnership = "support-delete-auth-ownership"

// AuthOwnershipFingerprint is for synthetic fixtures/injected inventories. It
// does not infer attribution from document content or mint a trusted manifest.
func AuthOwnershipFingerprint(data map[string]interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", supportdelete.ErrEvidence
	}
	return digest(string(b)), nil
}

func readAuthOwnership(doc *firestore.DocumentSnapshot) (AuthOwnershipSnapshot, error) {
	var s AuthOwnershipSnapshot
	if doc == nil || len(doc.Data()) != 10 || doc.DataTo(&s) != nil || s.validate() != nil {
		return s, supportdelete.ErrEvidence
	}
	for _, key := range []string{"schemaVersion", "execution", "captureOperationID", "traceRef", "capturedAt", "complete", "identities", "mappings", "related", "mappingsRemoved"} {
		if _, ok := doc.Data()[key]; !ok {
			return s, supportdelete.ErrEvidence
		}
	}
	execution, ok := doc.Data()["execution"].(map[string]interface{})
	if !ok || len(execution) != 12 {
		return s, supportdelete.ErrEvidence
	}
	for _, key := range []string{"selector", "manifest", "ownerRef", "revision", "generation", "guardSince", "cutoff", "acceptedAt", "deleteBy", "updatedAt", "cursor", "evidenceDigest"} {
		if _, ok := execution[key]; !ok {
			return s, supportdelete.ErrEvidence
		}
	}
	return s, nil
}

func (s *FirestoreDeletionStore) LoadAuthOwnership(ctx context.Context, e supportdelete.Execution) (AuthOwnershipSnapshot, error) {
	if s.check(e.Selector) != nil || e.Validate() != nil {
		return AuthOwnershipSnapshot{}, supportdelete.ErrUnavailable
	}
	var result AuthOwnershipSnapshot
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if _, _, err := s.readFence(tx, e, true); err != nil {
			return err
		}
		doc, err := tx.Get(s.Client.Collection(supportDeleteAuthOwnership).Doc(e.Selector.ExecutionRef))
		if status.Code(err) == codes.NotFound {
			return ErrAuthOwnershipMissing
		}
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		result, err = readAuthOwnership(doc)
		if err != nil || result.Execution != e {
			return supportdelete.ErrConflict
		}
		return nil
	})
	if err == ErrAuthOwnershipMissing {
		return result, ErrAuthOwnershipMissing
	}
	return result, deletionStoreError(err)
}

func (s *FirestoreDeletionStore) CaptureAuthOwnership(ctx context.Context, snapshot AuthOwnershipSnapshot) error {
	e := snapshot.Execution
	if s.check(e.Selector) != nil || snapshot.validate() != nil || e.Cursor != 0 || snapshot.MappingsRemoved {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if _, _, err := s.readFence(tx, e, true); err != nil {
			return err
		}
		ref := s.Client.Collection(supportDeleteAuthOwnership).Doc(e.Selector.ExecutionRef)
		doc, err := tx.Get(ref)
		if err == nil {
			old, err := readAuthOwnership(doc)
			if err != nil || !reflect.DeepEqual(old, snapshot) {
				return supportdelete.ErrConflict
			}
			return nil
		}
		if status.Code(err) != codes.NotFound {
			return supportdelete.ErrUnavailable
		}
		if err := s.scanAuthOwnership(tx, snapshot, false); err != nil {
			return err
		}
		if tx.Create(ref, snapshot) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	})
	return deletionStoreError(err)
}

func (s *FirestoreDeletionStore) currentAuthOwnership(tx *firestore.Transaction, expected AuthOwnershipSnapshot) (AuthOwnershipSnapshot, error) {
	if _, _, err := s.readFence(tx, expected.Execution, true); err != nil {
		return AuthOwnershipSnapshot{}, err
	}
	doc, err := tx.Get(s.Client.Collection(supportDeleteAuthOwnership).Doc(expected.Execution.Selector.ExecutionRef))
	if err != nil {
		return AuthOwnershipSnapshot{}, supportdelete.ErrUnavailable
	}
	current, err := readAuthOwnership(doc)
	if err != nil {
		return current, err
	}
	// A committed mapping removal may have lost its acknowledgement. No other
	// snapshot change is allowed at this execution fence.
	comparison := current
	comparison.MappingsRemoved = expected.MappingsRemoved
	if !reflect.DeepEqual(comparison, expected) || (expected.MappingsRemoved && !current.MappingsRemoved) {
		return current, supportdelete.ErrConflict
	}
	return current, nil
}

func (s *FirestoreDeletionStore) CheckAuthOwnership(ctx context.Context, expected AuthOwnershipSnapshot) error {
	if s.check(expected.Execution.Selector) != nil || expected.validate() != nil {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		current, err := s.currentAuthOwnership(tx, expected)
		if err != nil {
			return err
		}
		return s.scanAuthOwnership(tx, current, true)
	})
	return deletionStoreError(err)
}

// scanAuthOwnership checks the injected complete relation set against bounded
// Firestore queries plus direct legacy UID documents. Query zero is never used
// as proof of Auth absence/completeness. An unknown/shared/new relation stops
// the whole transaction before any delete.
func (s *FirestoreDeletionStore) scanAuthOwnership(tx *firestore.Transaction, snapshot AuthOwnershipSnapshot, allowMissing bool) error {
	e := snapshot.Execution
	channel := e.Selector.Target.ChannelID
	// Acquire already verified the OAuth proof and atomically bound its claim
	// to this execution. TTL may remove that trace even before this snapshot;
	// the durable binding, not renewed OAuth presence, authorizes continuation.
	claim, err := tx.Get(s.Client.Collection(supportDeleteClaims).Doc(e.Selector.ProofRef))
	if err != nil || len(claim.Data()) != 2 || claim.Data()["executionRef"] != e.Selector.ExecutionRef || claim.Data()["requestRef"] != e.Selector.RequestRef {
		return supportdelete.ErrEvidence
	}
	currentDoc, err := tx.Get(s.Client.Collection("support-requests").Doc(e.Selector.RequestRef))
	var current SupportRequest
	dry := DryRunSelector{Environment: e.Selector.Target.Environment, Purpose: SupportDelete, Now: e.UpdatedAt}
	if err != nil || currentDoc.DataTo(&current) != nil || validateDryRunReceipt(current, dry) != nil || current.ProofRef != e.Selector.ProofRef || current.TargetChannel != channel || !current.AcceptedAt.Equal(e.AcceptedAt) || !current.DeleteBy.Equal(e.DeleteBy) {
		return supportdelete.ErrEvidence
	}
	currentOAuthKey := "oauth-transactions/" + current.OAuthTransactionID
	_, err = tx.Get(s.Client.Collection("oauth-transactions").Doc(current.OAuthTransactionID))
	currentOAuthMissing := status.Code(err) == codes.NotFound
	if err != nil && !currentOAuthMissing {
		return supportdelete.ErrUnavailable
	}
	allowed := map[string]AuthOwnedDocument{}
	for _, group := range [][]AuthOwnedDocument{snapshot.Mappings, snapshot.Related} {
		for _, d := range group {
			allowed[d.Collection+"/"+d.ID] = d
		}
	}
	seen := map[string]bool{}
	checkDoc := func(doc *firestore.DocumentSnapshot) error {
		key := doc.Ref.Parent.ID + "/" + doc.Ref.ID
		d, ok := allowed[key]
		fingerprint, err := AuthOwnershipFingerprint(doc.Data())
		if !ok || err != nil || d.Fingerprint != fingerprint {
			return supportdelete.ErrEvidence
		}
		if snapshot.MappingsRemoved && (d.Collection == "mypage-users" || d.Collection == "mypage-youtube-channel-owners") {
			return supportdelete.ErrConflict
		}
		if d.Collection == "mypage-youtube-channel-owners" {
			uid, ok := doc.Data()["firebase-uid"].(string)
			if !ok || !snapshot.hasUID(uid) || d.ID != channel {
				return supportdelete.ErrEvidence
			}
		}
		if d.Collection == "oauth-transactions" {
			if c, ok := doc.Data()["channel"].(map[string]interface{}); ok && c["channelId"] != nil && c["channelId"] != "" && c["channelId"] != channel {
				return supportdelete.ErrEvidence
			}
		}
		seen[key] = true
		return nil
	}
	query := func(collection, field, value string, visit func(*firestore.DocumentSnapshot) error) error {
		docs, err := tx.Documents(s.Client.Collection(collection).Where(field, "==", value).Limit(101)).GetAll()
		if err != nil {
			return supportdelete.ErrUnavailable
		}
		if len(docs) > 100 {
			return supportdelete.ErrEvidence
		}
		for _, doc := range docs {
			if err := visit(doc); err != nil {
				return err
			}
		}
		return nil
	}
	for _, d := range allowed {
		doc, err := tx.Get(s.Client.Collection(d.Collection).Doc(d.ID))
		if status.Code(err) == codes.NotFound && d.Collection+"/"+d.ID == currentOAuthKey && currentOAuthMissing {
			continue
		}
		if status.Code(err) == codes.NotFound && allowMissing && d.Collection != "mypage-users" && d.Collection != "mypage-youtube-channel-owners" {
			continue
		}
		if status.Code(err) == codes.NotFound && snapshot.MappingsRemoved && (d.Collection == "mypage-users" || d.Collection == "mypage-youtube-channel-owners") {
			continue
		}
		if err != nil {
			return supportdelete.ErrEvidence
		}
		if err := checkDoc(doc); err != nil {
			return err
		}
	}
	owner, err := tx.Get(s.Client.Collection("mypage-youtube-channel-owners").Doc(channel))
	if err == nil {
		if err := checkDoc(owner); err != nil {
			return err
		}
	} else if status.Code(err) != codes.NotFound {
		return supportdelete.ErrUnavailable
	}
	for _, i := range snapshot.Identities {
		if err := query("mypage-youtube-channel-owners", "firebase-uid", i.UID, checkDoc); err != nil {
			return err
		}
		// Legacy account ownership is its document ID, not an optional field.
		doc, err := tx.Get(s.Client.Collection("mypage-users").Doc(i.UID))
		if err == nil {
			if err := checkDoc(doc); err != nil {
				return err
			}
		} else if status.Code(err) != codes.NotFound {
			return supportdelete.ErrUnavailable
		}
	}
	if err := query(supportDeleteExecutions, "selector.target.channelID", channel, func(doc *firestore.DocumentSnapshot) error {
		if doc.Ref.ID != e.Selector.ExecutionRef {
			return supportdelete.ErrEvidence
		}
		return nil
	}); err != nil {
		return err
	}
	requests := map[string]bool{e.Selector.RequestRef: true}
	for _, d := range snapshot.Related {
		if d.Collection == "support-requests" {
			requests[d.ID] = true
		}
	}
	if err := query("support-requests", "targetChannel", channel, checkDoc); err != nil {
		return err
	}
	if err := query("oauth-transactions", "channel.channelId", channel, checkDoc); err != nil {
		return err
	}
	for request := range requests {
		for _, lookup := range []struct{ collection, field string }{{"oauth-transactions", "support.requestRef"}, {"support-challenges", "requestRef"}, {"support-request-ids", "requestRef"}} {
			if err := query(lookup.collection, lookup.field, request, checkDoc); err != nil {
				return err
			}
		}
		if err := query(supportDeleteClaims, "requestRef", request, func(doc *firestore.DocumentSnapshot) error {
			if request != e.Selector.RequestRef || doc.Ref.ID != e.Selector.ProofRef || len(doc.Data()) != 2 || doc.Data()["executionRef"] != e.Selector.ExecutionRef {
				return supportdelete.ErrEvidence
			}
			return nil
		}); err != nil {
			return err
		}
	}
	// Each receipt's current dependencies must be included explicitly. Shared
	// OAuth proof references are ambiguous even if targetChannel matches.
	oauthOwners := map[string]string{}
	for request := range requests {
		doc, err := tx.Get(s.Client.Collection("support-requests").Doc(request))
		if status.Code(err) == codes.NotFound && allowMissing && request != e.Selector.RequestRef {
			continue
		}
		var receipt SupportRequest
		if err != nil || doc.DataTo(&receipt) != nil || receipt.TargetChannel != channel || receipt.Environment != e.Selector.Target.Environment {
			return supportdelete.ErrEvidence
		}
		if receipt.OAuthTransactionID != "" {
			if oauthOwners[receipt.OAuthTransactionID] != "" || !validOpaque(receipt.OAuthTransactionID) {
				return supportdelete.ErrEvidence
			}
			oauthOwners[receipt.OAuthTransactionID] = request
			key := "oauth-transactions/" + receipt.OAuthTransactionID
			boundTraceMissing := request == e.Selector.RequestRef && currentOAuthMissing
			if _, ok := allowed[key]; (!ok || (request == e.Selector.RequestRef && !seen[key])) && !boundTraceMissing {
				return supportdelete.ErrEvidence
			}
			if err := query("support-requests", "oauthTransactionId", receipt.OAuthTransactionID, func(other *firestore.DocumentSnapshot) error {
				if other.Ref.ID != request {
					return supportdelete.ErrEvidence
				}
				return nil
			}); err != nil {
				return err
			}
		}
		if request == e.Selector.RequestRef && !seen["support-request-ids/"+digest(receipt.Environment+":"+receipt.RequestID)] {
			return supportdelete.ErrEvidence
		}
		if receipt.ProofRef != "" {
			if err := query("support-requests", "proofRef", receipt.ProofRef, func(other *firestore.DocumentSnapshot) error {
				if other.Ref.ID != request {
					return supportdelete.ErrEvidence
				}
				return nil
			}); err != nil {
				return err
			}
		}
		if request != e.Selector.RequestRef && receipt.ProofRef != "" {
			if !validOpaque(receipt.ProofRef) {
				return supportdelete.ErrEvidence
			}
			_, err := tx.Get(s.Client.Collection(supportDeleteClaims).Doc(receipt.ProofRef))
			if status.Code(err) != codes.NotFound {
				return supportdelete.ErrEvidence
			}
		}
	}
	return nil
}

func (s *FirestoreDeletionStore) ApplyAuthRecords(ctx context.Context, op supportdelete.Operation, expected AuthOwnershipSnapshot) error {
	if s.check(op.Execution.Selector) != nil || expected.Execution != op.Execution || expected.validate() != nil || (op.Step.Action != "delete" && op.Step.Action != "inspect") {
		return supportdelete.ErrEvidence
	}
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		current, err := s.currentAuthOwnership(tx, expected)
		if err != nil {
			return err
		}
		if err := s.scanAuthOwnership(tx, current, true); err != nil {
			return err
		}
		var candidates []AuthOwnedDocument
		switch op.Step.Scope {
		case "legacy-mappings":
			if current.MappingsRemoved {
				return nil
			}
			candidates = current.Mappings
		case "firestore-oauth", "firestore-support-relations":
			candidates = current.Related
		default:
			return supportdelete.ErrUnavailable
		}
		var receipt SupportRequest
		doc, err := tx.Get(s.Client.Collection("support-requests").Doc(op.Execution.Selector.RequestRef))
		if err != nil || doc.DataTo(&receipt) != nil || receipt.ProofRef != op.Execution.Selector.ProofRef {
			return supportdelete.ErrConflict
		}
		var deletes []*firestore.DocumentRef
		for _, d := range candidates {
			if op.Step.Scope == "firestore-oauth" && d.Collection != "oauth-transactions" {
				continue
			}
			if (d.Collection == "support-requests" && d.ID == op.Execution.Selector.RequestRef) || (d.Collection == "oauth-transactions" && d.ID == receipt.OAuthTransactionID) || (d.Collection == "support-request-ids" && d.ID == digest(receipt.Environment+":"+receipt.RequestID)) {
				continue
			}
			ref := s.Client.Collection(d.Collection).Doc(d.ID)
			_, err := tx.Get(ref)
			if status.Code(err) == codes.NotFound {
				continue
			}
			if err != nil {
				return supportdelete.ErrUnavailable
			}
			if op.Step.Action == "inspect" {
				return supportdelete.ErrEvidence
			}
			deletes = append(deletes, ref)
		}
		for _, ref := range deletes {
			if tx.Delete(ref) != nil {
				return supportdelete.ErrUnavailable
			}
		}
		if op.Step.Scope == "legacy-mappings" && op.Step.Action == "delete" {
			current.MappingsRemoved = true
			if tx.Set(s.Client.Collection(supportDeleteAuthOwnership).Doc(op.Execution.Selector.ExecutionRef), current) != nil {
				return supportdelete.ErrUnavailable
			}
		}
		return nil
	})
	return deletionStoreError(err)
}

// moveAuthOwnership must run before any writes in the enclosing lifecycle
// transaction. Missing state is valid for workflows using other adapters.
func (s *FirestoreDeletionStore) moveAuthOwnership(tx *firestore.Transaction, old, next supportdelete.Execution, terminal bool) error {
	ref := s.Client.Collection(supportDeleteAuthOwnership).Doc(old.Selector.ExecutionRef)
	doc, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		return supportdelete.ErrUnavailable
	}
	snapshot, err := readAuthOwnership(doc)
	if err != nil || snapshot.Execution != old {
		return supportdelete.ErrConflict
	}
	if terminal {
		if tx.Delete(ref) != nil {
			return supportdelete.ErrUnavailable
		}
		return nil
	}
	snapshot.Execution = next
	if snapshot.validate() != nil || tx.Set(ref, snapshot) != nil {
		return supportdelete.ErrUnavailable
	}
	return nil
}
