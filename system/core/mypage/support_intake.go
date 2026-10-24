package mypage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IntakeSecret is a server-only, per-environment secret. It must be stable
// across instances and restarts while idempotency retries are supported.
// A missing secret leaves intake disabled; it must never enter a Vite bundle.
type FirestorePrivacyIntake struct {
	Client       *firestore.Client
	Environment  string
	IntakeSecret []byte
}

type PrivacyIntakeReceipt struct {
	RequestRef string         `json:"requestRef"`
	Challenge  string         `json:"supportChallenge,omitempty"`
	Purpose    SupportPurpose `json:"purpose"`
	Status     string         `json:"status"`
	AcceptedAt time.Time      `json:"acceptedAt"`
	DeleteBy   *time.Time     `json:"deleteBy"`
}

type PrivacyRequestStatus struct {
	RequestRef string         `json:"requestRef"`
	Purpose    SupportPurpose `json:"purpose"`
	Status     string         `json:"status"`
	AcceptedAt time.Time      `json:"acceptedAt"`
	DeleteBy   *time.Time     `json:"deleteBy"`
	VerifiedAt *time.Time     `json:"verifiedAt"`
	Reply      string         `json:"reply,omitempty"`
	ReplyAt    *time.Time     `json:"replyAt,omitempty"`
}

func validPrivacyBody(body string) bool {
	if body != strings.TrimSpace(body) || len(body) > 2000 || !utf8.ValidString(body) {
		return false
	}
	for _, r := range body {
		if unicode.IsControl(r) && r != '\n' {
			return false
		}
	}
	return true
}

func validSubmissionKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func (s *FirestorePrivacyIntake) ready() bool {
	return s != nil && s.Client != nil && validSupportEnvironment(s.Environment) && len(s.IntakeSecret) >= 32
}

func (s *FirestorePrivacyIntake) keyed(label, uid, key string) string {
	mac := hmac.New(sha256.New, s.IntakeSecret)
	_, _ = mac.Write([]byte(label + "\x00" + s.Environment + "\x00" + uid + "\x00" + key))
	return hex.EncodeToString(mac.Sum(nil))
}

// Create records the receipt and challenge atomically. The same identity/key
// yields the same opaque challenge on an exact retry, including after ACK loss.
// The signed-in uid is the target; no browser-supplied channel is accepted.
func (s *FirestorePrivacyIntake) Create(ctx context.Context, uid, key string, purpose SupportPurpose, body string, now time.Time) (PrivacyIntakeReceipt, error) {
	if !s.ready() {
		return PrivacyIntakeReceipt{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	if uid == "" || strings.Contains(uid, "/") || !validSubmissionKey(key) || !validSupportPurpose(purpose) || !validPrivacyBody(body) {
		return PrivacyIntakeReceipt{}, apiError("INVALID_REQUEST")
	}
	ref := s.keyed("ref", uid, key)
	challenge := s.keyed("challenge", uid, key)
	requestID := s.keyed("request-id", uid, key)
	fingerprint := digest(string(purpose) + "\x00" + body)
	doc := s.Client.Collection("support-requests").Doc(ref)
	var value SupportRequest
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snapshot, err := tx.Get(doc)
		if err == nil {
			if err := snapshot.DataTo(&value); err != nil {
				return fmt.Errorf("decode intake receipt: %w", err)
			}
			if value.Environment != s.Environment || value.TargetChannel != uid || value.RequestID != requestID || value.SubmissionHash != fingerprint || value.Purpose != purpose {
				return apiError("INTAKE_KEY_CONFLICT")
			}
			return nil
		}
		if status.Code(err) != codes.NotFound {
			return fmt.Errorf("read intake receipt: %w", err)
		}
		value, err = newSupportRecord(requestID, s.Environment, uid, purpose, digest(challenge), now, now)
		if err != nil {
			return err
		}
		value.SubmissionHash = fingerprint
		value.Body = body
		if err := tx.Create(doc, value); err != nil {
			return fmt.Errorf("create intake receipt: %w", err)
		}
		if err := tx.Create(s.Client.Collection("support-challenges").Doc(value.ChallengeHash), supportChallengeIndex{RequestRef: ref, Environment: s.Environment}); err != nil {
			return fmt.Errorf("create intake challenge: %w", err)
		}
		if err := tx.Create(s.Client.Collection("support-request-ids").Doc(digest(s.Environment+":"+requestID)), supportChallengeIndex{RequestRef: ref, Environment: s.Environment}); err != nil {
			return fmt.Errorf("create intake ID: %w", err)
		}
		return nil
	})
	if err != nil {
		return PrivacyIntakeReceipt{}, fmt.Errorf("privacy intake: %w", err)
	}
	receipt := PrivacyIntakeReceipt{RequestRef: ref, Purpose: value.Purpose, Status: value.Status, AcceptedAt: value.AcceptedAt, DeleteBy: value.DeleteBy}
	if value.Status == "awaiting_proof" && value.ChallengeHash == digest(challenge) && now.Before(value.ChallengeExpiresAt) {
		receipt.Challenge = challenge
	}
	return receipt, nil
}

// Status requires both the current Firebase identity and completed fresh
// support OAuth proof. Request refs alone are never bearer credentials.
func (s *FirestorePrivacyIntake) Status(ctx context.Context, uid, ref string) (PrivacyRequestStatus, error) {
	if !s.ready() {
		return PrivacyRequestStatus{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	if uid == "" || strings.Contains(uid, "/") || !validOpaque(ref) {
		return PrivacyRequestStatus{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	snap, err := s.Client.Collection("support-requests").Doc(ref).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return PrivacyRequestStatus{}, apiError("SUPPORT_CHALLENGE_INVALID")
		}
		return PrivacyRequestStatus{}, fmt.Errorf("read privacy status: %w", err)
	}
	var value SupportRequest
	if err := snap.DataTo(&value); err != nil {
		return PrivacyRequestStatus{}, fmt.Errorf("decode privacy status: %w", err)
	}
	if value.Environment != s.Environment || value.TargetChannel != uid || value.VerifiedAt == nil || value.Status != "verified" || value.ProofRef == "" {
		return PrivacyRequestStatus{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	return PrivacyRequestStatus{RequestRef: ref, Purpose: value.Purpose, Status: value.Status, AcceptedAt: value.AcceptedAt, DeleteBy: value.DeleteBy, VerifiedAt: value.VerifiedAt, Reply: value.OperatorReply, ReplyAt: value.OperatorReplyAt}, nil
}
