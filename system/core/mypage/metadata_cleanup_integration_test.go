//go:build integration

package mypage

import (
	"app.modules/internal/integrationtest"
	"bytes"
	"cloud.google.com/go/firestore"
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func TestCleanupPurgesInactiveBlockedAndOldPolicyMetadataAt29Days(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	integrationtest.ResetFirestore(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := now.Add(-50 * 24 * time.Hour)
	handle := "@synthetic-private"
	avatar := "https://example.invalid/synthetic"
	var refs []*firestore.DocumentRef
	before := map[string]map[string]any{}
	ages := []time.Duration{MetadataCleanupAge, MetadataRetentionLimit, MetadataCleanupAge + time.Hour, MetadataCleanupAge - time.Second}
	for i, age := range ages {
		uid := fmt.Sprintf("UCcleanup%015d", i)
		ref := client.Collection("web-accounts").Doc(uid)
		refs = append(refs, ref)
		account := WebAccount{DisplayName: "Synthetic private display", Handle: &handle, AvatarURL: &avatar, MetadataFetchedAt: now.Add(-age), CreatedAt: now.Add(-60 * 24 * time.Hour), UpdatedAt: now.Add(-age), PrivacyConsentedAt: now.Add(-55 * 24 * time.Hour), PrivacyPolicyVersion: "synthetic-old-policy", TermsVersion: "synthetic-old-terms", AccessBlocked: i == 1}
		if i != 2 {
			account.FirstWebLoginAt = &first
		}
		_, err := ref.Set(ctx, account)
		require.NoError(t, err)
		doc, err := ref.Get(ctx)
		require.NoError(t, err)
		before[uid] = doc.Data()
	}
	t.Cleanup(func() {
		for _, ref := range refs {
			_, err := ref.Delete(ctx)
			require.NoError(t, err)
		}
	})
	var output bytes.Buffer
	store := &FirestoreMetadataCleanupStore{Client: client, ProjectID: "demo-youtube-study-space-ci"}
	job := MetadataCleanupJob{Store: store, Recorder: &JSONMetadataCleanupRecorder{Writer: &output}, Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 10}
	o, err := job.Run(ctx)
	require.NoError(t, err)
	require.True(t, o.Succeeded)
	require.Equal(t, 3, o.Scanned)
	require.Equal(t, 3, o.Cleared)
	require.Equal(t, 1, o.RetentionViolations)
	require.Equal(t, "retention_deadline", EvaluateMetadataCleanupHealth(&o, now, now).Reason)
	for i, ref := range refs {
		doc, err := ref.Get(ctx)
		require.NoError(t, err)
		after := doc.Data()
		require.NotContains(t, output.String(), ref.ID)
		require.NotContains(t, output.String(), "Synthetic private")
		if i == 3 {
			require.Equal(t, before[ref.ID], after)
			continue
		}
		for _, field := range []string{"displayName", "handle", "avatarUrl", "metadataFetchedAt"} {
			require.NotContains(t, after, field)
		}
		for _, field := range []string{"privacyPolicyVersion", "termsVersion", "privacyConsentedAt", "createdAt", "firstWebLoginAt", "accessBlocked"} {
			require.Equal(t, before[ref.ID][field], after[field])
		}
		require.Equal(t, now, after["updatedAt"])
	}
	output.Reset()
	o, err = job.Run(ctx)
	require.NoError(t, err)
	require.Zero(t, o.Scanned)
	require.Zero(t, o.Cleared)
	require.False(t, EvaluateMetadataCleanupHealth(&o, now, now).Alert)
}
func TestCleanupConcurrentRefreshDeletionAndUnrelatedRevisionChange(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	integrationtest.ResetFirestore(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	now := time.Now().UTC().Truncate(time.Microsecond)
	uid := "UCcleanup000000000000001"
	ref := client.Collection("web-accounts").Doc(uid)
	store := &FirestoreMetadataCleanupStore{Client: client, ProjectID: "demo-youtube-study-space-ci"}
	cutoff := now.Add(-MetadataCleanupAge)
	seed := func() MetadataCleanupCandidate {
		_, err := ref.Set(ctx, WebAccount{DisplayName: "Old synthetic", MetadataFetchedAt: cutoff, PrivacyPolicyVersion: "synthetic-p", TermsVersion: "synthetic-t"})
		require.NoError(t, err)
		page, err := store.Scan(ctx, cutoff, nil, 1)
		require.NoError(t, err)
		require.Len(t, page.Candidates, 1)
		return page.Candidates[0]
	}
	t.Cleanup(func() { _, err := ref.Delete(ctx); require.NoError(t, err) })
	candidate := seed()
	_, err = ref.Update(ctx, []firestore.Update{{Path: "displayName", Value: "Fresh synthetic"}, {Path: "metadataFetchedAt", Value: now}})
	require.NoError(t, err)
	outcome, err := store.Clear(ctx, candidate, cutoff, now)
	require.NoError(t, err)
	require.Equal(t, "changed", outcome)
	doc, err := ref.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "Fresh synthetic", doc.Data()["displayName"])
	candidate = seed()
	_, err = ref.Delete(ctx)
	require.NoError(t, err)
	outcome, err = store.Clear(ctx, candidate, cutoff, now)
	require.NoError(t, err)
	require.Equal(t, "missing", outcome)
	_, err = ref.Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
	seed()
	changed := false
	var output bytes.Buffer
	raceStore := cleanupStoreFake{scan: store.Scan, clear: func(ctx context.Context, c MetadataCleanupCandidate, cutoff, now time.Time) (string, error) {
		if !changed {
			changed = true
			_, err := ref.Update(ctx, []firestore.Update{{Path: "privacyPolicyVersion", Value: "new synthetic consent"}})
			require.NoError(t, err)
		}
		return store.Clear(ctx, c, cutoff, now)
	}}
	job := MetadataCleanupJob{Store: raceStore, Recorder: &JSONMetadataCleanupRecorder{Writer: &output}, Now: func() time.Time { return now }, BatchSize: 1, MaxPages: 10}
	o, err := job.Run(ctx)
	require.ErrorIs(t, err, ErrMetadataCleanupIncomplete)
	require.False(t, o.Succeeded)
	require.Equal(t, "backlog", o.Stage)
	require.Equal(t, 1, o.Changed)
	doc, err = ref.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "new synthetic consent", doc.Data()["privacyPolicyVersion"])
	require.Equal(t, "Old synthetic", doc.Data()["displayName"])
	// Next pass reads the current revision; only metadata is removed.
	job.Store = store
	o, err = job.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, o.Cleared)
	doc, err = ref.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "new synthetic consent", doc.Data()["privacyPolicyVersion"])
	require.NotContains(t, doc.Data(), "displayName")
	wrong := &FirestoreMetadataCleanupStore{Client: client, ProjectID: "demo-other"}
	_, err = wrong.Scan(ctx, cutoff, nil, 1)
	require.ErrorIs(t, err, ErrMetadataCleanupIncomplete)
}
