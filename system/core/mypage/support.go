package mypage

import (
	"crypto/subtle"
	"strings"
	"time"
)

// SupportPurpose is fixed in a server-owned record. Intake may accept a
// validated category, but browser input never authorizes proof or an action.
type SupportPurpose string

const (
	SupportDelete     SupportPurpose = "delete"
	SupportRevoke     SupportPurpose = "revoke"
	SupportDisclosure SupportPurpose = "disclosure"
)

type SupportBinding struct {
	RequestRef    string         `firestore:"requestRef"`
	RequestID     string         `firestore:"requestId"`
	Environment   string         `firestore:"environment"`
	Purpose       SupportPurpose `firestore:"purpose"`
	ChallengeHash string         `firestore:"challengeHash"`
}

// SupportRequest is server-only. The challenge is random and stored as a hash.
// A successful proof is only identity verification, never execution approval.
type SupportRequest struct {
	RequestID          string         `firestore:"requestId"`
	Environment        string         `firestore:"environment"`
	Purpose            SupportPurpose `firestore:"purpose"`
	Status             string         `firestore:"status"`
	AcceptedAt         time.Time      `firestore:"acceptedAt"`
	DeleteBy           *time.Time     `firestore:"deleteBy"`
	TargetChannel      string         `firestore:"targetChannel"`
	ChallengeHash      string         `firestore:"challengeHash"`
	ChallengeExpiresAt time.Time      `firestore:"challengeExpiresAt"`
	VerifiedAt         *time.Time     `firestore:"verifiedAt"`
	ProofRef           string         `firestore:"proofRef"`
	OAuthTransactionID string         `firestore:"oauthTransactionId"`
	CompletedAt        *time.Time     `firestore:"completedAt"`
	// Intake text is server-only and removed on completion. SubmissionHash
	// binds retries to the exact original request without collapsing claims.
	SubmissionHash  string     `firestore:"submissionHash,omitempty"`
	Body            string     `firestore:"body,omitempty"`
	OperatorReply   string     `firestore:"operatorReply,omitempty"`
	OperatorReplyAt *time.Time `firestore:"operatorReplyAt,omitempty"`
}

func validSupportEnvironment(environment string) bool {
	return environment == "development" || environment == "production"
}

func validSupportPurpose(purpose SupportPurpose) bool {
	return purpose == SupportDelete || purpose == SupportRevoke || purpose == SupportDisclosure
}

func validRequestID(id string) bool {
	if len(id) < 1 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func newSupportRecord(requestID, environment, channel string, purpose SupportPurpose, challengeHash string, acceptedAt, now time.Time) (SupportRequest, error) {
	if !validRequestID(requestID) || !validSupportEnvironment(environment) || !validSupportPurpose(purpose) || channel == "" || strings.Contains(channel, "/") || !validOpaque(challengeHash) || acceptedAt.IsZero() || acceptedAt.After(now) {
		return SupportRequest{}, apiError("INVALID_REQUEST")
	}
	value := SupportRequest{RequestID: requestID, Environment: environment, Purpose: purpose, Status: "awaiting_proof", AcceptedAt: acceptedAt.UTC(), TargetChannel: channel, ChallengeHash: challengeHash, ChallengeExpiresAt: now.UTC().Add(24 * time.Hour)}
	if purpose == SupportDelete {
		deadline := value.AcceptedAt.Add(7 * 24 * time.Hour)
		value.DeleteBy = &deadline
	}
	return value, nil
}

func checkSupport(value SupportRequest, environment, hash string, now time.Time) error {
	if !validSupportEnvironment(environment) || value.Environment != environment || !validRequestID(value.RequestID) || !validSupportPurpose(value.Purpose) || value.TargetChannel == "" || strings.Contains(value.TargetChannel, "/") || value.Status != "awaiting_proof" || value.AcceptedAt.IsZero() || value.AcceptedAt.After(now) || !now.Before(value.ChallengeExpiresAt) || !validOpaque(hash) || subtle.ConstantTimeCompare([]byte(value.ChallengeHash), []byte(hash)) != 1 {
		return apiError("SUPPORT_CHALLENGE_INVALID")
	}
	return nil
}

func verifySupportRecord(value SupportRequest, binding SupportBinding, channel, transactionID, proofRef string, now time.Time) (SupportRequest, error) {
	if !validOpaque(binding.RequestRef) || binding.RequestID != value.RequestID || binding.Purpose != value.Purpose || binding.Environment != value.Environment || subtle.ConstantTimeCompare([]byte(binding.ChallengeHash), []byte(value.ChallengeHash)) != 1 || !validOpaque(transactionID) || !validOpaque(proofRef) {
		return value, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	if err := checkSupport(value, binding.Environment, value.ChallengeHash, now); err != nil {
		return value, err
	}
	if channel != value.TargetChannel {
		return value, apiError("SUPPORT_CHANNEL_MISMATCH")
	}
	at := now.UTC()
	value.Status = "verified"
	value.VerifiedAt = &at
	value.ProofRef = proofRef
	value.OAuthTransactionID = transactionID
	// Consume the challenge in the same transition as successful proof.
	value.ChallengeHash = ""
	value.ChallengeExpiresAt = time.Time{}
	return value, nil
}

func reissueSupportRecord(value SupportRequest, environment, hash string, now time.Time) (SupportRequest, error) {
	if !validSupportEnvironment(environment) || value.Environment != environment || value.Status != "awaiting_proof" || value.AcceptedAt.IsZero() || value.AcceptedAt.After(now) || !validSupportPurpose(value.Purpose) || !validOpaque(hash) {
		return value, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	value.ChallengeHash = hash
	value.ChallengeExpiresAt = now.UTC().Add(24 * time.Hour)
	return value, nil
}

func completeSupportRecord(value SupportRequest, environment string, now time.Time) (SupportRequest, error) {
	if !validSupportEnvironment(environment) || value.Environment != environment || value.Status != "verified" || value.VerifiedAt == nil || now.Before(*value.VerifiedAt) || value.OAuthTransactionID == "" {
		return value, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	at := now.UTC()
	value.Status = "completed"
	value.CompletedAt = &at
	value.TargetChannel = ""
	value.ChallengeHash = ""
	value.ChallengeExpiresAt = time.Time{}
	value.ProofRef = ""
	value.OAuthTransactionID = ""
	value.Body = ""
	value.OperatorReply = ""
	value.OperatorReplyAt = nil
	value.SubmissionHash = ""
	return value, nil
}
