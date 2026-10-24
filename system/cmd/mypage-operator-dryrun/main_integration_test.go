//go:build integration

package main

import (
	"app.modules/core/mypage"
	"app.modules/internal/integrationtest"
	"bytes"
	"cloud.google.com/go/firestore"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCLIEmulatorReportIsSanitizedAndLeavesReceiptUnchanged(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	now := time.Now().UTC().Add(-3 * time.Minute).Truncate(time.Microsecond)
	uid := "UCoperator" + strings.Repeat("0", 13) + "1"
	requestID := fmt.Sprintf("synthetic-cli-%d", time.Now().UnixNano())
	hash := func(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
	support := &mypage.FirestoreSupportStore{Client: client, Environment: "development"}
	ref, challenge, err := support.Create(ctx, requestID, uid, mypage.SupportDelete, now.Add(-time.Hour), now)
	require.NoError(t, err)
	binding, err := support.Resolve(ctx, challenge, now)
	require.NoError(t, err)
	txid := hash(requestID + "-oauth")
	proof := hash(requestID + "-proof")
	confirmation := hash(requestID + "-confirmation")
	policy := mypage.Policy{Privacy: "synthetic-p", Terms: "synthetic-t"}
	auth := &mypage.FirestoreAuthStore{Client: client}
	value := mypage.OAuthTransaction{Purpose: "support", Support: &binding, Status: "channel_verified", Channel: mypage.Channel{ID: uid}, CreatedAt: now, VerifiedAt: now.Add(time.Minute), ExpiresAt: now.Add(10 * time.Minute), ConfirmationRef: confirmation, PrivacyPolicyVersion: policy.Privacy, TermsVersion: policy.Terms}
	require.NoError(t, auth.CreateSupport(ctx, txid, "", value, "development", now))
	require.NoError(t, auth.ConsumeSupport(ctx, txid, confirmation, policy, "development", proof, now.Add(2*time.Minute)))
	receipt := client.Collection("support-requests").Doc(ref)
	before, err := receipt.Get(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, r := range []*firestore.DocumentRef{receipt, client.Collection("oauth-transactions").Doc(txid), client.Collection("support-request-ids").Doc(hash("development:" + requestID))} {
			_, err := r.Delete(ctx)
			require.NoError(t, err)
		}
	})
	get := func(k string) string {
		switch k {
		case "GOOGLE_CLOUD_PROJECT":
			return "demo-youtube-study-space-ci"
		case "MYPAGE_ENVIRONMENT":
			return "development"
		case "MYPAGE_SUPPORT_PURPOSE":
			return "delete"
		default:
			return os.Getenv(k)
		}
	}
	var output bytes.Buffer
	require.NoError(t, run(ctx, get, nil, strings.NewReader(ref+"\n"), &output))
	require.Contains(t, output.String(), `"readOnly":true`)
	require.Contains(t, output.String(), `"executionAuthorized":false`)
	for _, private := range []string{uid, requestID, ref, challenge, txid, proof} {
		require.NotContains(t, output.String(), private)
	}
	after, err := receipt.Get(ctx)
	require.NoError(t, err)
	require.True(t, before.UpdateTime.Equal(after.UpdateTime))
	output.Reset()
	err = run(ctx, get, nil, strings.NewReader(strings.Repeat("f", 64)), &output)
	require.Error(t, err)
	require.Empty(t, output.String())
	require.Equal(t, "dryrun unavailable", err.Error())
}
