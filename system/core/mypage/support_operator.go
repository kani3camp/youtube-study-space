package mypage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const supportOperatorAudit = "support-operator-audit"

var (
	ErrOperatorDenied   = errors.New("trusted operator unavailable or denied")
	ErrOperatorConflict = errors.New("support operation conflict")
)

// OperatorAuthority must obtain identity and permissions from a trusted
// operator environment. A CLI-supplied name, request ref or support challenge
// is never an authority. There is deliberately no production implementation.
type OperatorAuthority interface {
	Authorize(context.Context, OperatorIntent) (OperatorIdentity, error)
	VerifyEvidence(context.Context, OperatorIdentity, CompletionOperation) (VerifiedCompletionEvidence, error)
}

type OperatorIntent struct {
	Environment string
	ProjectID   string
	RequestRef  string
	Purpose     SupportPurpose
	OperationID string
	Action      string
}

type OperatorIdentity struct {
	Subject     string
	Environment string
	ProjectID   string
}

type ReplyOperation struct {
	Intent           OperatorIntent
	ProofRef         string
	ExpectedRevision int64
	Body             string
	At               time.Time
}

type CompletionOperation struct {
	Intent                  OperatorIntent
	ProofRef                string
	ReplyOperationID        string
	ExpectedRevision        int64
	ActionEvidence          string
	DeliveryAcknowledgement string
	At                      time.Time
}

// VerifiedCompletionEvidence is returned only after the trusted authority has
// verified both evidence references and their binding to the reviewed reply.
// The operator proposal cannot attest to these fields on its own.
type VerifiedCompletionEvidence struct {
	Actor                   string
	Environment             string
	ProjectID               string
	RequestRef              string
	Purpose                 SupportPurpose
	OperationID             string
	ProofRef                string
	ReplyOperationID        string
	ExpectedRevision        int64
	ReplyDigest             string
	ActionEvidence          string
	DeliveryAcknowledgement string
}

func (e VerifiedCompletionEvidence) matches(identity OperatorIdentity, op CompletionOperation) bool {
	return e.Actor == identity.Subject && e.Environment == op.Intent.Environment && e.ProjectID == op.Intent.ProjectID &&
		e.RequestRef == op.Intent.RequestRef && e.Purpose == op.Intent.Purpose && e.OperationID == op.Intent.OperationID &&
		e.ProofRef == op.ProofRef && e.ReplyOperationID == op.ReplyOperationID && e.ExpectedRevision == op.ExpectedRevision &&
		validOpaque(e.ReplyDigest) && e.ActionEvidence == op.ActionEvidence && e.DeliveryAcknowledgement == op.DeliveryAcknowledgement
}

// FirestoreSupportOperator is never constructed by a public HTTP handler.
// Without an injected trusted authority and a server-only audit key it denies
// every mutation. The project is checked against the Firestore client path.
type FirestoreSupportOperator struct {
	Client      *firestore.Client
	Environment string
	ProjectID   string
	AuditKey    []byte
	Authority   OperatorAuthority
}

type operatorAuditEvent struct {
	SchemaVersion int64          `firestore:"schemaVersion"`
	Environment   string         `firestore:"environment"`
	Purpose       SupportPurpose `firestore:"purpose"`
	Action        string         `firestore:"action"`
	Actor         string         `firestore:"actor"`
	Binding       string         `firestore:"binding"`
	Fingerprint   string         `firestore:"fingerprint"`
	At            time.Time      `firestore:"at"`
}

func (s *FirestoreSupportOperator) mac(parts ...string) string {
	h := hmac.New(sha256.New, s.AuditKey)
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (s *FirestoreSupportOperator) authorize(ctx context.Context, intent OperatorIntent, action string) (OperatorIdentity, error) {
	if s == nil || s.Client == nil || s.Authority == nil || len(s.AuditKey) < 32 || !validSupportEnvironment(s.Environment) || s.ProjectID == "" ||
		!strings.HasPrefix(s.Client.Collection("support-requests").Path, "projects/"+s.ProjectID+"/databases/(default)/documents/") ||
		intent.Environment != s.Environment || intent.ProjectID != s.ProjectID || intent.Action != action || !validOpaque(intent.RequestRef) || !validOpaque(intent.OperationID) || !validSupportPurpose(intent.Purpose) {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	identity, err := s.Authority.Authorize(ctx, intent)
	if err != nil || identity.Environment != s.Environment || identity.ProjectID != s.ProjectID || len(identity.Subject) < 3 || len(identity.Subject) > 128 || strings.ContainsAny(identity.Subject, "/\\\n\r\t") {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	return identity, nil
}

func (s *FirestoreSupportOperator) audit(intent OperatorIntent, actor, fingerprint string, at time.Time) operatorAuditEvent {
	return operatorAuditEvent{
		SchemaVersion: 1, Environment: s.Environment, Purpose: intent.Purpose, Action: intent.Action, Actor: actor,
		Binding: s.mac("request", intent.RequestRef), Fingerprint: fingerprint, At: at.UTC().Truncate(time.Microsecond),
	}
}

func (s *FirestoreSupportOperator) auditRef(intent OperatorIntent) *firestore.DocumentRef {
	return s.Client.Collection(supportOperatorAudit).Doc(s.mac("operation", intent.OperationID))
}

func (s *FirestoreSupportOperator) validRecord(v SupportRequest, op OperatorIntent, proof string, at time.Time) bool {
	return v.Environment == s.Environment && v.Purpose == op.Purpose && v.Status == "verified" && v.VerifiedAt != nil && !at.Before(*v.VerifiedAt) &&
		validOpaque(v.ProofRef) && v.ProofRef == proof && validOpaque(v.OAuthTransactionID) && v.TargetChannel != "" && v.CompletedAt == nil
}

func (s *FirestoreSupportOperator) deletionClaimed(tx *firestore.Transaction, proof string) (bool, error) {
	_, err := tx.Get(s.Client.Collection(supportDeleteClaims).Doc(proof))
	if err == nil {
		return true, nil
	}
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	return false, fmt.Errorf("read deletion claim: %w", err)
}

// Reply writes one reviewed response and a bounded body-free audit event in
// the same transaction. It never marks the requested action completed.
func (s *FirestoreSupportOperator) Reply(ctx context.Context, op ReplyOperation) error {
	identity, err := s.authorize(ctx, op.Intent, "reply")
	if err != nil {
		return err
	}
	if !validOpaque(op.ProofRef) || op.ExpectedRevision < 0 || op.At.IsZero() || op.Body == "" || !validPrivacyBody(op.Body) {
		return ErrOperatorDenied
	}
	fingerprint := s.mac("reply", op.Intent.Environment, op.Intent.ProjectID, op.Intent.RequestRef, string(op.Intent.Purpose), op.ProofRef, op.Intent.OperationID, fmt.Sprint(op.ExpectedRevision), op.Body)
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		ref := s.Client.Collection("support-requests").Doc(op.Intent.RequestRef)
		v, err := readSupportRecord(tx, ref)
		if err != nil {
			if errorCode(err) == "SUPPORT_CHALLENGE_INVALID" {
				return ErrOperatorConflict
			}
			return err
		}
		if !s.validRecord(v, op.Intent, op.ProofRef, op.At) {
			return ErrOperatorConflict
		}
		claimed, err := s.deletionClaimed(tx, op.ProofRef)
		if err != nil {
			return err
		}
		if claimed {
			return ErrOperatorConflict
		}
		auditRef := s.auditRef(op.Intent)
		auditSnap, err := tx.Get(auditRef)
		if err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("read reply audit: %w", err)
		}
		if err == nil {
			var event operatorAuditEvent
			if auditSnap.DataTo(&event) != nil || event.Fingerprint != fingerprint || event.Actor != identity.Subject || v.ReplyOperationID != op.Intent.OperationID || v.ReplyDigest != fingerprint || v.OperatorRevision != op.ExpectedRevision+1 {
				return ErrOperatorConflict
			}
			return nil
		}
		if v.OperatorRevision != op.ExpectedRevision || v.ReplyOperationID != "" || v.OperatorReply != "" {
			return ErrOperatorConflict
		}
		if err := tx.Update(ref, []firestore.Update{{Path: "operatorReply", Value: op.Body}, {Path: "operatorReplyAt", Value: op.At.UTC().Truncate(time.Microsecond)}, {Path: "operatorRevision", Value: op.ExpectedRevision + 1}, {Path: "replyOperationId", Value: op.Intent.OperationID}, {Path: "replyDigest", Value: fingerprint}}); err != nil {
			return fmt.Errorf("write reply: %w", err)
		}
		if err := tx.Create(auditRef, s.audit(op.Intent, identity.Subject, fingerprint, op.At)); err != nil {
			return fmt.Errorf("write reply audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("operator reply: %w", err)
	}
	return nil
}

// Complete applies only to disclosure or Firebase-session revoke. Evidence
// verification is delegated to the trusted adapter; strings are references,
// not self-attested proof. Deletion has its own fenced Finalize workflow.
func (s *FirestoreSupportOperator) Complete(ctx context.Context, op CompletionOperation) error {
	identity, err := s.authorize(ctx, op.Intent, "complete")
	if err != nil {
		return err
	}
	if op.Intent.Purpose == SupportDelete || !validOpaque(op.ProofRef) || !validOpaque(op.ReplyOperationID) || !validOpaque(op.ActionEvidence) || !validOpaque(op.DeliveryAcknowledgement) || op.ExpectedRevision < 1 || op.At.IsZero() {
		return ErrOperatorDenied
	}
	fingerprint := s.mac("complete", op.Intent.Environment, op.Intent.ProjectID, op.Intent.RequestRef, string(op.Intent.Purpose), op.ProofRef, op.Intent.OperationID, op.ReplyOperationID, fmt.Sprint(op.ExpectedRevision), op.ActionEvidence, op.DeliveryAcknowledgement)
	auditRef := s.auditRef(op.Intent)
	// A committed audit and terminal receipt are enough to recover a lost ACK.
	// Evidence may have been scrubbed or its verifier may now be unavailable.
	auditSnap, err := auditRef.Get(ctx)
	if err == nil {
		var event operatorAuditEvent
		if auditSnap.DataTo(&event) != nil || !s.sameCompletionAudit(event, op, identity, fingerprint) {
			return ErrOperatorConflict
		}
		snap, err := s.Client.Collection("support-requests").Doc(op.Intent.RequestRef).Get(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrOperatorConflict
			}
			return fmt.Errorf("read completion receipt: %w", err)
		}
		if !simpleOperatorTerminal(snap) || snap.Data()["purpose"] != string(op.Intent.Purpose) {
			return ErrOperatorConflict
		}
		return nil
	}
	if status.Code(err) != codes.NotFound {
		return fmt.Errorf("read completion audit: %w", err)
	}
	verified, err := s.Authority.VerifyEvidence(ctx, identity, op)
	if err != nil || !verified.matches(identity, op) {
		return ErrOperatorDenied
	}
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		ref := s.Client.Collection("support-requests").Doc(op.Intent.RequestRef)
		snap, err := tx.Get(ref)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrOperatorConflict
			}
			return fmt.Errorf("read completion receipt: %w", err)
		}
		auditSnap, err := tx.Get(auditRef)
		if err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("read completion audit: %w", err)
		}
		if err == nil {
			var event operatorAuditEvent
			if auditSnap.DataTo(&event) != nil || !s.sameCompletionAudit(event, op, identity, fingerprint) || !simpleOperatorTerminal(snap) || snap.Data()["purpose"] != string(op.Intent.Purpose) {
				return ErrOperatorConflict
			}
			return nil
		}
		var v SupportRequest
		if snap.DataTo(&v) != nil || !s.validRecord(v, op.Intent, op.ProofRef, op.At) || v.OperatorRevision != op.ExpectedRevision || v.ReplyOperationID != op.ReplyOperationID || v.ReplyDigest != verified.ReplyDigest || v.OperatorReply == "" || v.OperatorReplyAt == nil {
			return ErrOperatorConflict
		}
		claimed, err := s.deletionClaimed(tx, op.ProofRef)
		if err != nil {
			return err
		}
		if claimed {
			return ErrOperatorConflict
		}
		// Replacement removes body, reply, channel, proof and all receipt fields.
		terminal := map[string]interface{}{"schemaVersion": int64(1), "completedAt": op.At.UTC().Truncate(time.Microsecond), "purpose": v.Purpose, "acceptedAt": v.AcceptedAt}
		if err := tx.Delete(s.Client.Collection("support-request-ids").Doc(digest(v.Environment + ":" + v.RequestID))); err != nil {
			return fmt.Errorf("delete request index: %w", err)
		}
		if err := tx.Delete(s.Client.Collection("oauth-transactions").Doc(v.OAuthTransactionID)); err != nil {
			return fmt.Errorf("delete support OAuth: %w", err)
		}
		if err := tx.Set(ref, terminal); err != nil {
			return fmt.Errorf("write terminal request: %w", err)
		}
		if err := tx.Create(auditRef, s.audit(op.Intent, identity.Subject, fingerprint, op.At)); err != nil {
			return fmt.Errorf("write completion audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("operator completion: %w", err)
	}
	return nil
}

func (s *FirestoreSupportOperator) sameCompletionAudit(event operatorAuditEvent, op CompletionOperation, identity OperatorIdentity, fingerprint string) bool {
	return event.SchemaVersion == 1 && event.Environment == op.Intent.Environment && event.Purpose == op.Intent.Purpose &&
		event.Action == "complete" && event.Actor == identity.Subject && event.Binding == s.mac("request", op.Intent.RequestRef) && event.Fingerprint == fingerprint
}

func simpleOperatorTerminal(snap *firestore.DocumentSnapshot) bool {
	if snap == nil || len(snap.Data()) != 4 {
		return false
	}
	v := snap.Data()
	_, version := v["schemaVersion"].(int64)
	_, completed := v["completedAt"].(time.Time)
	_, purpose := v["purpose"].(string)
	_, accepted := v["acceptedAt"].(time.Time)
	return version && completed && purpose && accepted
}
