package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dryRunReaderFunc func(context.Context, DryRunSelector) (DryRunSnapshot, error)

func (f dryRunReaderFunc) Inspect(ctx context.Context, s DryRunSelector) (DryRunSnapshot, error) {
	return f(ctx, s)
}

func dryRunFixture() (DryRunSelector, DryRunSnapshot) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	verified := now.Add(-time.Minute)
	accepted := now.Add(-8 * 24 * time.Hour)
	deadline := accepted.Add(7 * 24 * time.Hour)
	s := DryRunSelector{Environment: "development", ExpectedProject: "demo-synthetic", RequestRef: strings.Repeat("a", 64), Purpose: SupportDelete, Now: now}
	receipt := SupportRequest{RequestID: "synthetic-private-receipt", Environment: s.Environment, Purpose: s.Purpose, Status: "verified", AcceptedAt: accepted, DeleteBy: &deadline, TargetChannel: "UCsynthetic0000000000001", VerifiedAt: &verified, ProofRef: strings.Repeat("b", 64), OAuthTransactionID: strings.Repeat("c", 64)}
	oauth := OAuthTransaction{Purpose: "support", Status: "consumed", Support: &SupportBinding{RequestRef: s.RequestRef, RequestID: receipt.RequestID, Environment: s.Environment, Purpose: s.Purpose, ChallengeHash: strings.Repeat("d", 64)}, CreatedAt: verified.Add(-2 * time.Minute), VerifiedAt: verified.Add(-time.Minute), ConsumedAt: verified, ExpiresAt: verified.Add(8 * time.Minute)}
	return s, DryRunSnapshot{Project: s.ExpectedProject, AsOf: now, Receipt: receipt, OAuth: oauth, Counts: map[string]InventoryCount{}}
}

func TestDryRunSanitizedIncompleteInventoryAndImmutableDeadline(t *testing.T) {
	s, snapshot := dryRunFixture()
	zero, over := 0, 1001
	snapshot.Counts["web-accounts"] = InventoryCount{State: "known", Count: &zero, Scope: "channel"}
	snapshot.Counts["work-segments"] = InventoryCount{State: "at_least", Count: &over, Scope: "channel"}
	snapshot.Counts["private-extra"] = InventoryCount{State: "known", Count: &zero, Scope: "channel"}
	report, err := BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) { return snapshot, nil }))
	require.NoError(t, err)
	require.True(t, report.ReadOnly)
	require.False(t, report.ExecutionAuthorized)
	require.False(t, report.InventoryComplete)
	require.True(t, report.Overdue)
	require.Equal(t, snapshot.Receipt.DeleteBy, report.DeleteBy)
	require.Equal(t, "known", report.Counts["web-accounts"].State)
	require.Equal(t, "at_least", report.Counts["work-segments"].State)
	require.Equal(t, "unknown", report.Counts["seats"].State)
	require.Nil(t, report.Counts["seats"].Count)
	require.NotContains(t, report.Counts, "private-extra")
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	for _, private := range []string{s.RequestRef, snapshot.Receipt.TargetChannel, snapshot.Receipt.ProofRef, snapshot.Receipt.RequestID, snapshot.Receipt.OAuthTransactionID, snapshot.OAuth.Support.ChallengeHash, "private-extra"} {
		require.NotContains(t, string(raw), private)
	}
}

func TestDryRunRejectsMismatchedUnverifiedOrReusedProof(t *testing.T) {
	changes := map[string]func(*DryRunSelector, *DryRunSnapshot){
		"environment":              func(s *DryRunSelector, _ *DryRunSnapshot) { s.Environment = "production" },
		"purpose":                  func(s *DryRunSelector, _ *DryRunSnapshot) { s.Purpose = SupportRevoke },
		"project":                  func(_ *DryRunSelector, v *DryRunSnapshot) { v.Project = "demo-other" },
		"future_snapshot":          func(s *DryRunSelector, v *DryRunSnapshot) { v.AsOf = s.Now.Add(2 * time.Minute) },
		"stale_snapshot":           func(s *DryRunSelector, v *DryRunSnapshot) { v.AsOf = s.Now.Add(-2 * time.Minute) },
		"unverified":               func(_ *DryRunSelector, v *DryRunSnapshot) { v.Receipt.Status = "awaiting_proof" },
		"extended_deadline":        func(s *DryRunSelector, v *DryRunSnapshot) { v.Receipt.DeleteBy = &s.Now },
		"ordinary_login":           func(_ *DryRunSelector, v *DryRunSnapshot) { v.OAuth.Purpose = "login" },
		"binding":                  func(_ *DryRunSelector, v *DryRunSnapshot) { v.OAuth.Support.RequestRef = strings.Repeat("e", 64) },
		"replay":                   func(_ *DryRunSelector, v *DryRunSnapshot) { v.OAuth.Status = "channel_verified" },
		"expired_proof_at_capture": func(_ *DryRunSelector, v *DryRunSnapshot) { v.OAuth.ExpiresAt = v.OAuth.ConsumedAt },
		"unconsumed_challenge":     func(_ *DryRunSelector, v *DryRunSnapshot) { v.Receipt.ChallengeHash = strings.Repeat("d", 64) },
		"completed":                func(s *DryRunSelector, v *DryRunSnapshot) { v.Receipt.CompletedAt = &s.Now },
		"missing_proof":            func(_ *DryRunSelector, v *DryRunSnapshot) { v.Receipt.ProofRef = "" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			s, v := dryRunFixture()
			change(&s, &v)
			_, err := BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) { return v, nil }))
			require.ErrorIs(t, err, ErrDryRunUnavailable)
		})
	}
	calls := 0
	s, _ := dryRunFixture()
	s.RequestRef = "invalid"
	_, err := BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) { calls++; return DryRunSnapshot{}, nil }))
	require.ErrorIs(t, err, ErrDryRunUnavailable)
	require.Zero(t, calls)
}

func TestDryRunDependencyErrorIdentityAndInvalidCountsStayUnknown(t *testing.T) {
	s, v := dryRunFixture()
	_, err := BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) {
		return DryRunSnapshot{}, context.Canceled
	}))
	require.True(t, errors.Is(err, context.Canceled))
	negative := -1
	v.Counts["users"] = InventoryCount{State: "known", Count: &negative, Scope: "channel"}
	report, err := BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) { return v, nil }))
	require.NoError(t, err)
	require.Equal(t, "unknown", report.Counts["users"].State)
	require.Nil(t, report.Counts["users"].Count)
	for _, purpose := range []SupportPurpose{SupportRevoke, SupportDisclosure} {
		s, v := dryRunFixture()
		s.Purpose = purpose
		v.Receipt.Purpose = purpose
		v.Receipt.DeleteBy = nil
		v.OAuth.Support.Purpose = purpose
		report, err := BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) { return v, nil }))
		require.NoError(t, err)
		require.Nil(t, report.DeleteBy)
	}
}

func TestDryRunMissingTTLTraceRemainsUnknownAndNeverAuthorizesExecution(t *testing.T) {
	s, v := dryRunFixture()
	v.OAuth = OAuthTransaction{}
	v.OAuthMissing = true
	report, err := BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) { return v, nil }))
	require.NoError(t, err)
	require.Equal(t, "unknown", report.ProofTrace)
	require.False(t, report.ExecutionAuthorized)
	require.False(t, report.InventoryComplete)
	v.Receipt.ProofRef = ""
	_, err = BuildDryRun(context.Background(), s, dryRunReaderFunc(func(context.Context, DryRunSelector) (DryRunSnapshot, error) { return v, nil }))
	require.ErrorIs(t, err, ErrDryRunUnavailable)
}
