//go:build integration

package mypage

import (
	"app.modules/internal/integrationtest"
	"cloud.google.com/go/firestore"
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"strings"
	"testing"
	"time"
)

func TestOperatorDryRunUsesCurrentProofSchemaWithoutMutations(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	now := time.Now().UTC().Add(-3 * time.Minute).Truncate(time.Microsecond)
	uid := "UCdryrun" + strings.Repeat("0", 15) + "1"
	store := &FirestoreSupportStore{Client: client, Environment: "development"}
	requestID := "synthetic-dryrun-" + fmt.Sprint(time.Now().UnixNano())
	ref, challenge, err := store.Create(ctx, requestID, uid, SupportDelete, now.Add(-8*24*time.Hour), now)
	require.NoError(t, err)
	binding, err := store.Resolve(ctx, challenge, now)
	require.NoError(t, err)
	txid := strings.Repeat("e", 64)
	proof := strings.Repeat("f", 64)
	policy := Policy{Privacy: "synthetic-privacy", Terms: "synthetic-terms"}
	transaction := OAuthTransaction{Purpose: "support", Support: &binding, Status: "channel_verified", Channel: Channel{ID: uid, DisplayName: "Synthetic private name"}, PrivacyPolicyVersion: policy.Privacy, TermsVersion: policy.Terms, CreatedAt: now, VerifiedAt: now.Add(time.Minute), ExpiresAt: now.Add(10 * time.Minute), ConfirmationRef: strings.Repeat("c", 64)}
	auth := &FirestoreAuthStore{Client: client}
	require.NoError(t, auth.CreateSupport(ctx, txid, "", transaction, "development", now))
	require.NoError(t, auth.ConsumeSupport(ctx, txid, transaction.ConfirmationRef, policy, "development", proof, now.Add(2*time.Minute)))
	refs := []*firestore.DocumentRef{client.Collection("support-requests").Doc(ref), client.Collection("support-request-ids").Doc(digest("development:" + requestID)), client.Collection("oauth-transactions").Doc(txid), client.Collection("web-accounts").Doc(uid), client.Collection("users").Doc(uid), client.Collection("seats").Doc("synthetic-dryrun-seat")}
	for _, r := range refs[3:] {
		_, err := r.Set(ctx, map[string]any{"user-id": uid, "displayName": "Synthetic private name", "work-name": "Synthetic private work"})
		require.NoError(t, err)
	}
	before := map[string]time.Time{}
	for _, r := range refs {
		doc, err := r.Get(ctx)
		require.NoError(t, err)
		before[r.Path] = doc.UpdateTime
	}
	t.Cleanup(func() {
		for _, r := range refs {
			_, err := r.Delete(ctx)
			require.NoError(t, err)
		}
	})
	selector := DryRunSelector{Environment: "development", ExpectedProject: "demo-youtube-study-space-ci", RequestRef: ref, Purpose: SupportDelete, Now: time.Now().UTC()}
	report, err := BuildDryRun(ctx, selector, &FirestoreDryRunReader{Client: client})
	require.NoError(t, err)
	require.True(t, report.Overdue)
	require.False(t, report.ExecutionAuthorized)
	require.False(t, report.InventoryComplete)
	for _, name := range []string{"web-accounts", "users", "seats", "channel-support-requests", "receipt-oauth-transactions", "receipt-request-indexes"} {
		require.Equal(t, "known", report.Counts[name].State)
		require.NotNil(t, report.Counts[name].Count)
		require.Equal(t, 1, *report.Counts[name].Count)
	}
	require.Equal(t, 0, *report.Counts["receipt-challenge-indexes"].Count)
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	for _, private := range []string{uid, challenge, proof, ref, txid, requestID, "Synthetic private"} {
		require.NotContains(t, string(raw), private)
	}
	for _, r := range refs {
		doc, err := r.Get(ctx)
		require.NoError(t, err)
		require.True(t, before[r.Path].Equal(doc.UpdateTime), "read-only report changed a synthetic document")
	}
	// The limit is a lower bound, never an exact count after truncation.
	workRefs := make([]*firestore.DocumentRef, 0, 1002)
	for i := 0; i < 1002; i++ {
		workRefs = append(workRefs, client.Collection("work-segments").Doc(fmt.Sprintf("synthetic-dryrun-work-%04d", i)))
	}
	for start := 0; start < len(workRefs); start += 400 {
		batch := client.Batch()
		for _, r := range workRefs[start:min(start+400, len(workRefs))] {
			batch.Set(r, map[string]any{"user-id": uid})
		}
		_, err := batch.Commit(ctx)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		for start := 0; start < len(workRefs); start += 400 {
			batch := client.Batch()
			for _, r := range workRefs[start:min(start+400, len(workRefs))] {
				batch.Delete(r)
			}
			_, err := batch.Commit(ctx)
			require.NoError(t, err)
		}
	})
	query := client.Collection("work-segments").Where("user-id", "==", uid).Select()
	beforeDocs, err := query.Documents(ctx).GetAll()
	require.NoError(t, err)
	require.Len(t, beforeDocs, 1002)
	for _, doc := range beforeDocs {
		before[doc.Ref.Path] = doc.UpdateTime
	}
	selector.Now = time.Now().UTC()
	report, err = BuildDryRun(ctx, selector, &FirestoreDryRunReader{Client: client})
	require.NoError(t, err)
	require.Equal(t, "at_least", report.Counts["work-segments"].State)
	require.Equal(t, 1001, *report.Counts["work-segments"].Count)
	afterDocs, err := query.Documents(ctx).GetAll()
	require.NoError(t, err)
	require.Len(t, afterDocs, 1002)
	for _, doc := range afterDocs {
		require.True(t, before[doc.Ref.Path].Equal(doc.UpdateTime))
	}

	wrong := selector
	wrong.ExpectedProject = "demo-other"
	_, err = BuildDryRun(ctx, wrong, &FirestoreDryRunReader{Client: client})
	require.ErrorIs(t, err, ErrDryRunUnavailable)
	wrong = selector
	wrong.Purpose = SupportRevoke
	_, err = BuildDryRun(ctx, wrong, &FirestoreDryRunReader{Client: client})
	require.ErrorIs(t, err, ErrDryRunUnavailable)
	// OAuth TTL removal cannot turn the diagnostic into execution approval.
	_, err = client.Collection("oauth-transactions").Doc(txid).Delete(ctx)
	require.NoError(t, err)
	selector.Now = time.Now().UTC()
	report, err = BuildDryRun(ctx, selector, &FirestoreDryRunReader{Client: client})
	require.NoError(t, err)
	require.Equal(t, "unknown", report.ProofTrace)
	require.False(t, report.ExecutionAuthorized)
	require.False(t, report.InventoryComplete)

}
