package mypage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func supportFixture(t *testing.T) (SupportRequest, SupportBinding, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	value, err := newSupportRecord("SYNTHETIC-001", "development", "UCsynthetic-support", SupportDelete, digest("synthetic-challenge"), now.Add(-time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	binding := SupportBinding{RequestRef: strings.Repeat("a", 64), RequestID: value.RequestID, Environment: value.Environment, Purpose: value.Purpose, ChallengeHash: value.ChallengeHash}
	return value, binding, now
}

func TestSupportReissuePreservesReceiptAndInvalidatesPriorProof(t *testing.T) {
	value, binding, now := supportFixture(t)
	next, err := reissueSupportRecord(value, "development", digest("new-synthetic-challenge"), now.Add(25*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !value.AcceptedAt.Equal(next.AcceptedAt) || !value.DeleteBy.Equal(*next.DeleteBy) || !next.ChallengeExpiresAt.Equal(now.Add(49*time.Hour)) {
		t.Fatal("reissue reset receipt/deletion deadline or lost 24-hour expiry")
	}
	if _, err := verifySupportRecord(next, binding, next.TargetChannel, strings.Repeat("b", 64), strings.Repeat("c", 64), now.Add(25*time.Hour)); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("OAuth bound before challenge reissue remained usable")
	}
}

func TestSupportProofRejectsMismatchExpiryAndReplay(t *testing.T) {
	value, binding, now := supportFixture(t)
	for _, scenario := range []string{"environment", "purpose", "request", "channel", "expiry"} {
		t.Run(scenario, func(t *testing.T) {
			candidate := binding
			channel, at := value.TargetChannel, now
			switch scenario {
			case "environment":
				candidate.Environment = "production"
			case "purpose":
				candidate.Purpose = SupportRevoke
			case "request":
				candidate.RequestID = "SYNTHETIC-OTHER"
			case "channel":
				channel = "UCsynthetic-other"
			case "expiry":
				at = value.ChallengeExpiresAt
			}
			if _, err := verifySupportRecord(value, candidate, channel, strings.Repeat("b", 64), strings.Repeat("c", 64), at); err == nil {
				t.Fatal("invalid support proof accepted")
			}
		})
	}
	verified, err := verifySupportRecord(value, binding, value.TargetChannel, strings.Repeat("b", 64), strings.Repeat("c", 64), now)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ChallengeHash != "" || verified.Status != "verified" || verified.ProofRef == "" {
		t.Fatal("proof did not consume challenge")
	}
	if _, err := verifySupportRecord(verified, binding, value.TargetChannel, strings.Repeat("b", 64), strings.Repeat("c", 64), now); err == nil {
		t.Fatal("proof replay accepted")
	}
	if _, err := reissueSupportRecord(verified, "development", digest("replacement"), now); err == nil {
		t.Fatal("verified request reopened by challenge reissue")
	}
}

func TestSupportCompletedRecordContainsOnlyAnonymousAudit(t *testing.T) {
	value, binding, now := supportFixture(t)
	if _, err := completeSupportRecord(value, "development", now); err == nil {
		t.Fatal("unverified request completed")
	}
	verified, err := verifySupportRecord(value, binding, value.TargetChannel, strings.Repeat("b", 64), strings.Repeat("c", 64), now)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := completeSupportRecord(verified, "development", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(completed)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range []string{value.TargetChannel, value.ChallengeHash, verified.ProofRef, verified.OAuthTransactionID} {
		if strings.Contains(string(encoded), privateValue) {
			t.Fatal("completed audit retained linkable proof/channel data")
		}
	}
	if completed.RequestID != value.RequestID || completed.AcceptedAt != value.AcceptedAt || completed.VerifiedAt == nil || completed.CompletedAt == nil || completed.Status != "completed" {
		t.Fatal("minimum receipt/proof/completion audit was lost")
	}
}

func TestSupportRecordRejectsUntrustedConfiguration(t *testing.T) {
	_, _, now := supportFixture(t)
	for _, scenario := range []string{"environment", "purpose", "id", "channel", "future"} {
		t.Run(scenario, func(t *testing.T) {
			env, id, channel, purpose, accepted := "development", "SYNTHETIC", "UCsynthetic", SupportDelete, now
			switch scenario {
			case "environment":
				env = "unknown"
			case "purpose":
				purpose = "login"
			case "id":
				id = "request/other"
			case "channel":
				channel = "channel/other"
			case "future":
				accepted = now.Add(time.Second)
			}
			if _, err := newSupportRecord(id, env, channel, purpose, digest("synthetic"), accepted, now); err == nil {
				t.Fatal("unsafe support record accepted")
			}
		})
	}
}
