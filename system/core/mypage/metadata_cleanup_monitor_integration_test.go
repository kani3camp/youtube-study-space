//go:build integration

package mypage

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"app.modules/internal/integrationtest"
)

func cleanupMonitorEmulator(t *testing.T) (*firestore.Client, *FirestoreMetadataCleanupMonitor) {
	t.Helper()
	integrationtest.RequireFirestoreEmulator(t)
	integrationtest.ResetFirestore(t)
	client, err := firestore.NewClient(context.Background(), "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client, &FirestoreMetadataCleanupMonitor{Client: client, ProjectID: "demo-youtube-study-space-ci"}
}

func TestCleanupMonitorEmulatorTransactionsPreserveCoverageAcrossRestartReplayAndConcurrentFailure(t *testing.T) {
	client, monitor := cleanupMonitorEmulator(t)
	ctx := context.Background()
	state, err := monitor.Load(ctx)
	require.NoError(t, err)
	require.Nil(t, state.Latest)
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	older := cleanupCompleteObservation(now.Add(-time.Hour))
	newerFailure := cleanupCompleteObservation(now)
	newerFailure.Succeeded = false
	newerFailure.Stage = "backlog"
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for _, observation := range []MetadataCleanupObservation{older, newerFailure} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- monitor.Record(ctx, observation)
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	// A fresh adapter represents a watchdog process/restart with no memory state.
	restarted := &FirestoreMetadataCleanupMonitor{Client: client, ProjectID: "demo-youtube-study-space-ci"}
	state, err = restarted.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, newerFailure, *state.Latest)
	require.Equal(t, older.StartedAt, state.LastSuccessfulCoverage)
	require.NoError(t, restarted.Record(ctx, older))
	state, err = restarted.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, newerFailure, *state.Latest)
	require.Equal(t, older.StartedAt, state.LastSuccessfulCoverage)
	recovered := cleanupCompleteObservation(now.Add(time.Hour))
	require.NoError(t, restarted.Record(ctx, recovered))
	state, err = restarted.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, recovered.StartedAt, state.LastSuccessfulCoverage)
	require.False(t, EvaluateMetadataCleanupHealth(state.Latest, state.LastSuccessfulCoverage, recovered.CompletedAt).Alert)
	// This collection is a single overwritten snapshot, not an accumulating log.
	docs, err := client.Collection("mypage-maintenance").Documents(ctx).GetAll()
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, "metadata-cleanup", docs[0].Ref.ID)
}

func TestCleanupMonitorEmulatorRejectsMalformedStateAndWrongProjectWithoutReset(t *testing.T) {
	client, monitor := cleanupMonitorEmulator(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	o := cleanupCompleteObservation(now)
	require.NoError(t, monitor.Record(ctx, o))
	before, err := monitor.ref().Get(ctx)
	require.NoError(t, err)
	bad := o
	bad.Stage = "synthetic-private-channel-id"
	require.ErrorIs(t, monitor.Record(ctx, bad), ErrMetadataCleanupIncomplete)
	wrong := &FirestoreMetadataCleanupMonitor{Client: client, ProjectID: "demo-other"}
	require.ErrorIs(t, wrong.Record(ctx, o), ErrMetadataCleanupIncomplete)
	_, err = wrong.Load(ctx)
	require.ErrorIs(t, err, ErrMetadataCleanupIncomplete)
	_, err = NewFirestoreMetadataCleanupRunner(client, "demo-other", &JSONMetadataCleanupHealthRecorder{Writer: &cleanupBufferWriter{}}, time.Now, 10, 10)
	require.ErrorIs(t, err, ErrMetadataCleanupIncomplete)
	after, err := monitor.ref().Get(ctx)
	require.NoError(t, err)
	require.Equal(t, before.Data(), after.Data(), "rejected record must not change coverage")
	_, err = monitor.ref().Set(ctx, map[string]any{"latest": map[string]any{"StartedAt": "invalid-type"}})
	require.NoError(t, err)
	malformed, err := monitor.ref().Get(ctx)
	require.NoError(t, err)
	_, err = monitor.Load(ctx)
	require.Error(t, err)
	require.Error(t, monitor.Record(ctx, o), "corrupt monitoring must fail closed, not reset silently")
	after, err = monitor.ref().Get(ctx)
	require.NoError(t, err)
	require.Equal(t, malformed.Data(), after.Data())
}

func TestCleanupRunnerEmulatorClearsOnlyMetadataAndWatchdogObservesRetentionAndStaleCoverage(t *testing.T) {
	client, monitor := cleanupMonitorEmulator(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	ref := client.Collection("web-accounts").Doc("UCcleanup000000000000001")
	_, err := ref.Set(ctx, WebAccount{DisplayName: "Synthetic private display", MetadataFetchedAt: now.Add(-MetadataRetentionLimit), PrivacyPolicyVersion: "synthetic-p", TermsVersion: "synthetic-t", AccessBlocked: true})
	require.NoError(t, err)
	var signals cleanupBufferWriter
	runner, err := NewFirestoreMetadataCleanupRunner(client, "demo-youtube-study-space-ci", &JSONMetadataCleanupHealthRecorder{Writer: &signals}, func() time.Time { return now }, 10, 10)
	require.NoError(t, err)
	o, err := runner.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, o.Cleared)
	require.Contains(t, signals.String(), "retention_deadline")
	doc, err := ref.Get(ctx)
	require.NoError(t, err)
	require.NotContains(t, doc.Data(), "displayName")
	require.NotContains(t, doc.Data(), "metadataFetchedAt")
	require.Equal(t, "synthetic-p", doc.Data()["privacyPolicyVersion"])
	require.Equal(t, true, doc.Data()["accessBlocked"])
	_, err = runner.Run(ctx)
	require.NoError(t, err)
	// Separately composed watchdog has no account store and cannot delete data.
	now = now.Add(MetadataCleanupHeartbeat)
	watchdog := MetadataCleanupRunner{Monitor: &FirestoreMetadataCleanupMonitor{Client: client, ProjectID: "demo-youtube-study-space-ci"}, Signals: &JSONMetadataCleanupHealthRecorder{Writer: &signals}, Now: func() time.Time { return now }}
	signal, err := watchdog.CheckHealth(ctx)
	require.NoError(t, err)
	require.True(t, signal.Alert)
	require.Equal(t, "stale_heartbeat", signal.Reason)
	for _, private := range []string{ref.ID, "Synthetic private display", "synthetic-p"} {
		require.NotContains(t, signals.String(), private)
	}
	maintenance, err := monitor.ref().Get(ctx)
	require.NoError(t, err)
	data := maintenance.Data()
	require.Len(t, data, 3)
	require.Contains(t, data, "latest")
	require.Contains(t, data, "lastSuccessfulCoverage")
}

func TestCleanupMonitoringClientRulesDenyAnonymousAndAuthenticatedReadsAndWrites(t *testing.T) {
	_, monitor := cleanupMonitorEmulator(t)
	require.NoError(t, monitor.Record(context.Background(), cleanupCompleteObservation(time.Now().UTC().Add(-time.Hour))))
	encode := base64.RawURLEncoding.EncodeToString
	token := encode([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + encode([]byte(`{"iss":"https://securetoken.google.com/demo-youtube-study-space-ci","aud":"demo-youtube-study-space-ci","sub":"synthetic-rules-user","user_id":"synthetic-rules-user","iat":1780000000,"exp":2090000000,"firebase":{"sign_in_provider":"custom"}}`)) + "."
	endpoint := "http://" + os.Getenv("FIRESTORE_EMULATOR_HOST") + "/v1/projects/demo-youtube-study-space-ci/databases/(default)/documents/mypage-maintenance/metadata-cleanup"
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, auth := range []string{"", "Bearer " + token} {
		for _, method := range []string{http.MethodGet, http.MethodPatch} {
			request, err := http.NewRequestWithContext(context.Background(), method, endpoint, strings.NewReader(`{"fields":{"lastSuccessfulCoverage":{"timestampValue":"2026-10-14T00:00:00Z"}}}`))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")
			if auth != "" {
				request.Header.Set("Authorization", auth)
			}
			response, err := client.Do(request)
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, response.StatusCode)
			require.NoError(t, response.Body.Close())
		}
	}
}

func TestCleanupMonitorEmulatorTimestampReplayAndRetentionAcknowledgement(t *testing.T) {
	_, monitor := cleanupMonitorEmulator(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 14, 0, 0, 0, 123456789, time.UTC)
	success := cleanupCompleteObservation(now)
	failure := success
	failure.Succeeded, failure.Stage = false, "monitoring"
	require.NoError(t, monitor.Record(ctx, success))
	require.NoError(t, monitor.Record(ctx, failure))
	require.NoError(t, monitor.Record(ctx, success))
	state, err := monitor.Load(ctx)
	require.NoError(t, err)
	require.False(t, state.Latest.Succeeded)
	require.Equal(t, "monitoring", state.Latest.Stage)
	require.Equal(t, now.Truncate(time.Microsecond), state.LastSuccessfulCoverage)

	first := cleanupCompleteObservation(now.Add(time.Hour))
	first.Scanned, first.Cleared, first.RetentionViolations = 1, 1, 1
	require.NoError(t, monitor.Record(ctx, first))
	firstState, err := monitor.Load(ctx)
	require.NoError(t, err)
	newer := first
	newer.StartedAt, newer.CompletedAt = first.StartedAt.Add(time.Hour), first.CompletedAt.Add(time.Hour)
	require.NoError(t, monitor.Record(ctx, newer))
	healthy := cleanupCompleteObservation(now.Add(3 * time.Hour))
	require.NoError(t, monitor.Record(ctx, healthy))
	// A delayed acknowledgement must not erase the newer undelivered violation.
	require.NoError(t, monitor.AcknowledgeRetention(ctx, firstState.PendingRetentionStartedAt))
	state, err = monitor.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, newer.StartedAt.Truncate(time.Microsecond), state.PendingRetentionStartedAt)
	var output cleanupBufferWriter
	watchdog := MetadataCleanupRunner{Monitor: monitor, Now: func() time.Time { return healthy.CompletedAt }, Signals: &JSONMetadataCleanupHealthRecorder{Writer: &output}}
	signal, err := watchdog.CheckHealth(ctx)
	require.NoError(t, err)
	require.Equal(t, "retention_deadline", signal.Reason)
	state, err = monitor.Load(ctx)
	require.NoError(t, err)
	require.True(t, state.PendingRetentionStartedAt.IsZero())
	require.Equal(t, healthy.StartedAt.Truncate(time.Microsecond), state.LastSuccessfulCoverage)
	signal, err = watchdog.CheckHealth(ctx)
	require.NoError(t, err)
	require.Equal(t, "healthy", signal.Reason)
}
