//go:build integration

package mypage

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/internal/integrationtest"
)

func TestMetadataRefreshDoesNotResurrectOrOverwriteChangedAccounts(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := &FirestoreAuthStore{Client: client}
	uid := "UCmetadata" + strings.Repeat("0", 13) + "1"
	ref := client.Collection("web-accounts").Doc(uid)
	controlRef := client.Collection(serviceaccess.Collection).Doc(uid)
	_, err = controlRef.Delete(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := controlRef.Delete(ctx); require.NoError(t, err) })
	now := time.Now().UTC().Truncate(time.Second)
	policy := Policy{Privacy: "synthetic-p", Terms: "synthetic-t"}
	initial := WebAccount{PrivacyPolicyVersion: policy.Privacy, TermsVersion: policy.Terms, PrivacyConsentedAt: now.Add(-time.Hour), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), FirstWebLoginAt: &now, DisplayName: "Old synthetic metadata", MetadataFetchedAt: now.Add(-25 * time.Hour)}
	t.Cleanup(func() { _, err := ref.Delete(ctx); require.NoError(t, err) })
	for _, change := range []string{"unchanged", "delete", "block", "policy", "same-time-login"} {
		t.Run(change, func(t *testing.T) {
			_, err := controlRef.Delete(ctx)
			require.NoError(t, err)
			_, err = ref.Set(ctx, initial)
			require.NoError(t, err)
			expected, err := store.ReadAccount(ctx, uid)
			require.NoError(t, err)
			require.False(t, expected.Revision.IsZero())
			calls := 0
			refresh := AccountMetadataRefresh{Access: &serviceaccess.FirestoreStore{Client: client}, Store: store, Policy: policy, Now: func() time.Time { return now }, Provider: metadataReaderFunc(func(context.Context, string) (Channel, error) {
				calls++
				switch change {
				case "delete":
					_, err := ref.Delete(ctx)
					require.NoError(t, err)
				case "block":
					_, err := (&serviceaccess.FirestoreStore{Client: client}).Change(ctx, uid, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "MODERATION", Reference: strings.Repeat("a", 64)}, now)
					require.NoError(t, err)
				case "policy":
					_, err := ref.Update(ctx, []firestore.Update{{Path: "termsVersion", Value: "new-synthetic"}})
					require.NoError(t, err)
				case "same-time-login":
					_, err := ref.Update(ctx, []firestore.Update{{Path: "displayName", Value: "New login metadata"}, {Path: "updatedAt", Value: initial.UpdatedAt}})
					require.NoError(t, err)
				}
				return Channel{ID: uid, DisplayName: "Fresh synthetic metadata"}, nil
			})}
			result, err := refresh.Refresh(ctx, uid, expected)
			require.Equal(t, 1, calls)
			if change == "unchanged" {
				require.NoError(t, err)
				require.Equal(t, "Fresh synthetic metadata", result.DisplayName)
				require.True(t, result.CreatedAt.Equal(initial.CreatedAt))
				require.True(t, result.FirstWebLoginAt.Equal(*initial.FirstWebLoginAt))
				require.Equal(t, policy.Terms, result.TermsVersion)
				doc, err := ref.Get(ctx)
				require.NoError(t, err)
				require.NotContains(t, doc.Data(), "Revision")
				require.NotContains(t, doc.Data(), "revision")
				return
			}
			require.Error(t, err)
			doc, readErr := ref.Get(ctx)
			if change == "delete" {
				require.Equal(t, codes.NotFound, status.Code(readErr))
				require.Equal(t, "WEB_ACCOUNT_REQUIRED", errorCode(err))
				return
			}
			require.NoError(t, readErr)
			require.NotEqual(t, "Fresh synthetic metadata", doc.Data()["displayName"])
			if change == "block" {
				require.Equal(t, "SERVICE_ACCESS_RESTRICTED", errorCode(err))
			}
			if change == "policy" {
				require.Equal(t, "PRIVACY_RECONSENT_REQUIRED", errorCode(err))
			}
		})
	}
}

func TestExpiredMetadataStripPreservesAccountAndRejectsEarlyOrChangedRevision(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := &FirestoreAuthStore{Client: client}
	uid := "UCexpire" + strings.Repeat("0", 15) + "1"
	ref := client.Collection("web-accounts").Doc(uid)
	controlRef := client.Collection(serviceaccess.Collection).Doc(uid)
	_, err = controlRef.Delete(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := controlRef.Delete(ctx); require.NoError(t, err) })
	now := time.Now().UTC().Truncate(time.Second)
	policy := Policy{Privacy: "synthetic-p", Terms: "synthetic-t"}
	handle := "@synthetic"
	avatar := "https://example.invalid/avatar"
	initial := WebAccount{PrivacyPolicyVersion: policy.Privacy, TermsVersion: policy.Terms, PrivacyConsentedAt: now.Add(-time.Hour), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), FirstWebLoginAt: &now, DisplayName: "Expired synthetic metadata", Handle: &handle, AvatarURL: &avatar, MetadataFetchedAt: now.Add(-30 * 24 * time.Hour)}
	_, err = ref.Set(ctx, initial)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := ref.Delete(ctx); require.NoError(t, err) })
	expected, err := store.ReadAccount(ctx, uid)
	require.NoError(t, err)
	expected.AccessCheckpoint = "absent"
	_, err = store.ExpireMetadata(ctx, uid, expected, policy, now.Add(-time.Second))
	require.Error(t, err)
	stripped, err := store.ExpireMetadata(ctx, uid, expected, policy, now)
	require.NoError(t, err)
	require.Empty(t, stripped.DisplayName)
	require.Nil(t, stripped.Handle)
	require.Nil(t, stripped.AvatarURL)
	require.True(t, stripped.MetadataFetchedAt.IsZero())
	require.True(t, stripped.CreatedAt.Equal(initial.CreatedAt))
	require.True(t, stripped.FirstWebLoginAt.Equal(*initial.FirstWebLoginAt))
	require.Equal(t, policy.Privacy, stripped.PrivacyPolicyVersion)
	require.Equal(t, MetadataTooOld, *accountSection(stripped, now).ReasonCode)
	doc, err := ref.Get(ctx)
	require.NoError(t, err)
	require.NotContains(t, doc.Data(), "metadataFetchedAt")
	_, err = store.SaveMetadata(ctx, uid, expected, Channel{ID: uid, DisplayName: "Late old response"}, policy, now)
	require.Error(t, err)
}

func TestTwoBFFProcessesDropPurgedMetadataBeforeCacheTTL(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	uid := "UCterminal" + strings.Repeat("0", 13) + "1"
	ref := client.Collection("web-accounts").Doc(uid)
	controlRef := client.Collection(serviceaccess.Collection).Doc(uid)
	_, err = controlRef.Delete(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := controlRef.Delete(ctx); require.NoError(t, err) })
	now := time.Now().UTC().Truncate(time.Microsecond)
	policy := Policy{Privacy: "p1", Terms: "t1"}
	account := healthyAccount(now.Add(-25 * time.Hour))
	account.CreatedAt = now.Add(-time.Hour)
	_, err = ref.Set(ctx, account)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := ref.Delete(ctx); require.NoError(t, err) })
	store := &FirestoreAuthStore{Client: client}
	current, err := store.ReadAccount(ctx, uid)
	require.NoError(t, err)
	reads := 0
	reader := readerFunc(func(context.Context, string) (WorkSnapshot, error) {
		reads++
		s := workFixture()
		s.AsOf = now
		return s, nil
	})
	processes := []*BFF{{Access: &serviceaccess.FirestoreStore{Client: client}, Environment: "demo", Now: func() time.Time { return now }, Reader: reader}, {Access: &serviceaccess.FirestoreStore{Client: client}, Environment: "demo", Now: func() time.Time { return now }, Reader: reader}}
	for _, b := range processes {
		response, err := b.Get(ctx, uid, current)
		require.NoError(t, err)
		require.NotNil(t, response.Account.Data)
	}
	// Public API absence is authoritative even for 25-day metadata.
	provider, err := NewPublicYouTubeMetadata("synthetic-key", &http.Client{Transport: providerTransport(func(*http.Request) (*http.Response, error) { return providerResponse(200, `{"items":[]}`), nil })})
	require.NoError(t, err)
	refresh := AccountMetadataRefresh{Access: &serviceaccess.FirestoreStore{Client: client}, Provider: provider, Store: store, Policy: policy, Now: func() time.Time { return now }}
	cleared, err := refresh.Refresh(ctx, uid, current)
	require.NoError(t, err)
	require.Empty(t, cleared.DisplayName)
	require.True(t, cleared.MetadataFetchedAt.IsZero())
	require.Equal(t, policy.Privacy, cleared.PrivacyPolicyVersion)
	doc, err := ref.Get(ctx)
	require.NoError(t, err)
	for _, field := range []string{"displayName", "handle", "avatarUrl", "metadataFetchedAt"} {
		require.NotContains(t, doc.Data(), field)
	}
	now = now.Add(time.Second)
	latest, err := store.ReadAccount(ctx, uid)
	require.NoError(t, err)
	for _, b := range processes {
		response, err := b.Get(ctx, uid, latest)
		require.NoError(t, err)
		require.Nil(t, response.Account.Data)
		require.Equal(t, MetadataTooOld, *response.Account.ReasonCode)
	}
	require.Equal(t, 4, reads, "both process caches must reject the old revision before their 30s TTL")
	current.AccessCheckpoint = "absent"
	_, err = store.SaveMetadata(ctx, uid, current, Channel{ID: uid, DisplayName: "Late old lookup"}, policy, now)
	require.Error(t, err)
}
