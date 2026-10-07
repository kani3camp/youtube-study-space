//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"app.modules/core/serviceaccess"
	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCLIEmulatorCommitsGuardBeforeRevokeAndPreservesIndependentReasons(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	const project = "demo-youtube-study-space-ci"
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, project, option.WithEndpoint(os.Getenv("FIRESTORE_EMULATOR_HOST")), option.WithoutAuthentication(), option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := &serviceaccess.FirestoreStore{Client: client}
	ref := sha256.Sum256([]byte(t.Name()))
	channel := "UC" + hex.EncodeToString(ref[:])[:22]
	document := client.Collection(serviceaccess.Collection).Doc(channel)
	t.Cleanup(func() {
		_, err := document.Delete(ctx)
		require.NoError(t, err)
	})
	now := testNow
	privacyRef := sha256.Sum256([]byte("synthetic privacy request"))
	seed, err := store.Change(ctx, channel, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: true, ReasonCode: "PRIVACY_DELETION", Reference: hex.EncodeToString(privacyRef[:])}, now.Add(-time.Minute))
	require.NoError(t, err)
	var output, stderr bytes.Buffer
	revokes := 0
	failedRevoke := true
	start := func(_ context.Context, selected target) (ports, error) {
		require.Equal(t, project, selected.ProjectID)
		require.Equal(t, channel, selected.ChannelID)
		return ports{Store: store, Revoke: func(ctx context.Context, selectedChannel string) error {
			revokes++
			actual, err := store.Read(ctx, selectedChannel)
			require.NoError(t, err)
			require.True(t, actual.Control.Moderation.Active, "revocation must see the durable guard")
			require.Equal(t, seed.Control.PrivacyDeletion, actual.Control.PrivacyDeletion)
			if failedRevoke {
				return errors.New(privateError)
			}
			return nil
		}}, nil
	}
	run := func(operation string) (int, report) {
		output.Reset()
		stderr.Reset()
		data := requestJSON(t, operation, func(v map[string]any) {
			for _, key := range []string{"target", "confirmation"} {
				objectField(t, v, key)["projectID"] = project
				objectField(t, v, key)["channelID"] = channel
			}
		})
		// The injected constructor is emulator-only. Production bootstrap is
		// never invoked; production configuration still rejects emulator env.
		get := func(k string) string {
			switch k {
			case "MYPAGE_ENVIRONMENT":
				return "development"
			case "GOOGLE_CLOUD_PROJECT":
				return project
			default:
				return ""
			}
		}
		code := command(ctx, []string{"--stdin", "--execute"}, get, bytes.NewBufferString(data), &output, &stderr, start, func() time.Time { return now })
		var out report
		require.NoError(t, json.Unmarshal(output.Bytes(), &out))
		for _, private := range []string{channel, project, testRef, privateError, hex.EncodeToString(privacyRef[:])} {
			require.NotContains(t, output.String()+stderr.String(), private)
		}
		return code, out
	}
	code, out := run("block")
	require.Equal(t, 3, code)
	require.Equal(t, "partial-failure", out.Status)
	require.Equal(t, "active", out.Moderation)
	require.Equal(t, "active", out.PrivacyDeletion)
	blocked, err := store.Read(ctx, channel)
	require.NoError(t, err)
	require.Equal(t, int64(2), blocked.Control.Generation)
	failedRevoke = false
	now = now.Add(time.Second)
	code, out = run("block")
	require.Zero(t, code)
	require.Equal(t, "succeeded", out.Revoke)
	repeated, err := store.Read(ctx, channel)
	require.NoError(t, err)
	require.Equal(t, blocked.Checkpoint(), repeated.Checkpoint(), "idempotent block must not advance generation/revision")
	require.Equal(t, 2, revokes, "retry must recover a previous partial revocation")
	code, out = run("unblock")
	require.Zero(t, code)
	require.Equal(t, "inactive", out.Moderation)
	require.Equal(t, "active", out.PrivacyDeletion)
	require.Equal(t, "deletion", out.Control)
	unblocked, err := store.Read(ctx, channel)
	require.NoError(t, err)
	require.Equal(t, int64(3), unblocked.Control.Generation)
	require.Equal(t, seed.Control.PrivacyDeletion, unblocked.Control.PrivacyDeletion)
	code, _ = run("unblock")
	require.Zero(t, code)
	repeated, err = store.Read(ctx, channel)
	require.NoError(t, err)
	require.Equal(t, unblocked.Checkpoint(), repeated.Checkpoint())
	code, out = run("inspect")
	require.Zero(t, code)
	require.Equal(t, "deletion", out.Control)
	require.Equal(t, 2, revokes, "unblock and inspect must not revoke")

	// This setup-only change models an independently reviewed privacy operator.
	// The CLI has no privacy mutation operation or record deletion capability.
	_, err = store.Change(ctx, channel, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion}, now.Add(time.Second))
	require.NoError(t, err)
	code, out = run("unblock")
	require.Zero(t, code)
	require.Equal(t, "allowed", out.Control)
	retained, err := document.Get(ctx)
	require.NoError(t, err, "an all-inactive control must remain durable")
	require.Equal(t, int64(4), retained.Data()["generation"])
}
