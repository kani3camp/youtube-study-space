package mypage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
)

// SupportRecoverySelector is the original human receipt, never authority or
// user proof. Recovery callers must independently authorize this exact binding.
type SupportRecoverySelector struct {
	Environment, ProjectID, RequestID, Channel string
	Purpose                                    SupportPurpose
	AcceptedAt                                 time.Time
}

// SupportReceiptInspection excludes body, contact, proof and challenge/hash.
// Revision is Firestore UpdateTime, not the reply-only OperatorRevision.
type SupportReceiptInspection struct {
	RequestRef string
	Revision   time.Time
	DeleteBy   *time.Time
}

func (s *FirestoreSupportStore) recoveryReady(ctx context.Context, selector SupportRecoverySelector, now time.Time) bool {
	return ctx != nil && ctx.Err() == nil && s != nil && s.Client != nil && validSupportEnvironment(s.Environment) && selector.Environment == s.Environment &&
		firebaseProjectID.MatchString(selector.ProjectID) && validRequestID(selector.RequestID) && youtubeChannelID.MatchString(selector.Channel) && validSupportPurpose(selector.Purpose) &&
		!selector.AcceptedAt.IsZero() && !now.IsZero() && !selector.AcceptedAt.After(now) &&
		strings.HasPrefix(s.Client.Collection("support-requests").Path, "projects/"+selector.ProjectID+"/databases/(default)/documents/")
}

func (s *FirestoreSupportStore) readRecovery(tx *firestore.Transaction, selector SupportRecoverySelector, now time.Time) (*firestore.DocumentSnapshot, SupportRequest, error) {
	deny := func() (*firestore.DocumentSnapshot, SupportRequest, error) {
		return nil, SupportRequest{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	index, err := tx.Get(s.Client.Collection("support-request-ids").Doc(digest(s.Environment + ":" + selector.RequestID)))
	if err != nil {
		return nil, SupportRequest{}, fmt.Errorf("read receipt recovery index: %w", err)
	}
	var binding supportChallengeIndex
	if len(index.Data()) != 2 || index.DataTo(&binding) != nil || binding.Environment != s.Environment || !validOpaque(binding.RequestRef) {
		return deny()
	}
	ref := s.Client.Collection("support-requests").Doc(binding.RequestRef)
	doc, err := tx.Get(ref)
	if err != nil {
		return nil, SupportRequest{}, fmt.Errorf("read receipt for recovery: %w", err)
	}
	var value SupportRequest
	if doc.DataTo(&value) != nil || doc.UpdateTime.IsZero() || value.Environment != selector.Environment || value.RequestID != selector.RequestID || value.TargetChannel != selector.Channel || value.Purpose != selector.Purpose || !value.AcceptedAt.Equal(selector.AcceptedAt) || value.Status != "awaiting_proof" || value.VerifiedAt != nil || value.CompletedAt != nil || value.ProofRef != "" || value.OAuthTransactionID != "" || !validOpaque(value.ChallengeHash) || value.ChallengeExpiresAt.IsZero() {
		return deny()
	}
	if value.Purpose == SupportDelete {
		if value.DeleteBy == nil || !value.DeleteBy.Equal(value.AcceptedAt.Add(7*24*time.Hour)) {
			return deny()
		}
	} else if value.DeleteBy != nil {
		return deny()
	}
	// Old challenge ownership must agree in the same read set as the receipt.
	old, err := tx.Get(s.Client.Collection("support-challenges").Doc(value.ChallengeHash))
	if err != nil {
		return nil, SupportRequest{}, fmt.Errorf("read recovery challenge index: %w", err)
	}
	var challenge supportChallengeIndex
	if len(old.Data()) != 2 || old.DataTo(&challenge) != nil || challenge.Environment != s.Environment || challenge.RequestRef != ref.ID {
		return deny()
	}
	if _, err := reissueSupportRecord(value, s.Environment, value.ChallengeHash, now); err != nil {
		return nil, SupportRequest{}, err
	}
	return doc, value, nil
}

// InspectRequestID is read-only. A missing/mismatched index remains unknown;
// callers must never interpret a failure as permission to call Create again.
func (s *FirestoreSupportStore) InspectRequestID(ctx context.Context, selector SupportRecoverySelector, now time.Time) (SupportReceiptInspection, error) {
	if !s.recoveryReady(ctx, selector, now) {
		return SupportReceiptInspection{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	var result SupportReceiptInspection
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		doc, value, err := s.readRecovery(tx, selector, now)
		if err != nil {
			return err
		}
		result = SupportReceiptInspection{RequestRef: doc.Ref.ID, Revision: doc.UpdateTime, DeleteBy: value.DeleteBy}
		return nil
	}, firestore.ReadOnly)
	if err != nil || ctx.Err() != nil {
		return SupportReceiptInspection{}, fmt.Errorf("inspect receipt recovery: %w", errors.Join(err, ctx.Err()))
	}
	return result, nil
}

// ReissueRequestID requires the explicitly reviewed inspection version. Reusing
// that version after a commit/ACK loss or a concurrent proof/reissue must fail.
// No operation record, retry loop, delivery or Create fallback is added.
func (s *FirestoreSupportStore) ReissueRequestID(ctx context.Context, selector SupportRecoverySelector, expected time.Time, now time.Time) (string, string, error) {
	if !s.recoveryReady(ctx, selector, now) || expected.IsZero() {
		return "", "", apiError("SUPPORT_CHALLENGE_INVALID")
	}
	challenge, err := opaque()
	if err != nil {
		return "", "", err
	}
	var ref string
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		doc, value, err := s.readRecovery(tx, selector, now)
		if err != nil {
			return err
		}
		if !doc.UpdateTime.Equal(expected) {
			return apiError("SUPPORT_CHALLENGE_INVALID")
		}
		if err := stageSupportReissue(tx, doc.Ref, value, s.Environment, challenge, now, s.Client); err != nil {
			return err
		}
		ref = doc.Ref.ID
		return nil
	})
	if err != nil || ctx.Err() != nil {
		return "", "", fmt.Errorf("recover support receipt: %w", errors.Join(err, ctx.Err()))
	}
	return ref, challenge, nil
}
