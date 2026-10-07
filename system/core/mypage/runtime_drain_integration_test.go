//go:build integration

package mypage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"app.modules/core/supportdelete"
	"app.modules/internal/integrationtest"

	"github.com/stretchr/testify/require"
)

func runtimeEmulatorExecution(now time.Time) supportdelete.Execution {
	e := runtimeExecution(now)
	e.Selector.Target.ProjectID = "demo-youtube-study-space-ci"
	return e
}

func TestRuntimeEmulatorRejectsOldCallbackAndConfirmReplayAfterResumeAndAcceptsFreshOAuth(t *testing.T) {
	s, store, provider, minter, now := authTestService(t)
	r := NewRuntimeRegistry("development", "demo-youtube-study-space-ci")
	s.Runtime = r
	a, e := runtimeAdapter(r, now), runtimeEmulatorExecution(now)
	for _, action := range []string{"pause", "drain", "resume"} {
		runtimeApply(t, a, e, action)
	}
	oldCallback, _ := seedTransaction(t, s, "pending", now.Add(-time.Minute))
	oldConfirm, confirmation := seedTransaction(t, s, "channel_verified", now.Add(-time.Minute))
	t.Cleanup(func() {
		for _, id := range []string{oldCallback, oldConfirm} {
			_, err := store.Client.Collection("oauth-transactions").Doc(id).Delete(context.Background())
			require.NoError(t, err)
		}
		_, err := store.Client.Collection("web-accounts").Doc(runtimeChannel).Delete(context.Background())
		require.NoError(t, err)
	})
	err := s.Callback(context.Background(), oldCallback, "synthetic-state", "synthetic-code", false)
	require.ErrorIs(t, err, ErrRuntimeFenced)
	doc, err := store.Client.Collection("oauth-transactions").Doc(oldCallback).Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "processing", doc.Data()["status"], "old callback cannot publish verification")
	_, err = s.ConfirmResult(context.Background(), oldConfirm, confirmation)
	require.ErrorIs(t, err, ErrRuntimeFenced)
	require.Zero(t, minter.calls.Load())
	doc, err = store.Client.Collection("oauth-transactions").Doc(oldConfirm).Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "channel_verified", doc.Data()["status"], "old confirm must not consume/upsert")
	freshNow := now.Add(time.Second)
	s.Now = func() time.Time { return freshNow }
	started, id, err := s.Start(context.Background(), "", StartRequest{PrivacyAccepted: true, TermsAccepted: true, PrivacyPolicyVersion: s.Policy.Privacy, TermsVersion: s.Policy.Terms})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := store.Client.Collection("oauth-transactions").Doc(id).Delete(context.Background())
		require.NoError(t, err)
	})
	authorize, err := url.Parse(started.AuthorizationURL)
	require.NoError(t, err)
	require.NoError(t, s.Callback(context.Background(), id, authorize.Query().Get("state"), "synthetic-code", false))
	channel, err := s.Channel(context.Background(), id)
	require.NoError(t, err)
	response, err := s.ConfirmResult(context.Background(), id, channel.ConfirmationRef)
	require.NoError(t, err)
	require.Equal(t, "synthetic-custom-token", response.CustomToken)
	require.EqualValues(t, 1, minter.calls.Load())
	require.EqualValues(t, 2, provider.calls.Load())
}

func TestRuntimeEmulatorCanceledHTTPMintIsStillPendingUntilSDKActuallyReturns(t *testing.T) {
	s, store, _, _, now := authTestService(t)
	r := NewRuntimeRegistry("development", "demo-youtube-study-space-ci")
	s.Runtime = r
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	t.Cleanup(func() {
		_, err := store.Client.Collection("oauth-transactions").Doc(id).Delete(context.Background())
		require.NoError(t, err)
		_, err = store.Client.Collection("web-accounts").Doc(runtimeChannel).Delete(context.Background())
		require.NoError(t, err)
	})
	entered, release := make(chan struct{}), make(chan struct{})
	s.Minter = runtimeMintFunc(func(context.Context, string) (string, error) {
		close(entered)
		<-release
		return "synthetic-late-token", nil
	})
	h := &HTTPHandler{Runtime: r, Auth: s, Verifier: &boundaryVerifier{}, PublicOrigin: "https://mypage.example.test"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodPost, "https://mypage.example.test/api/auth/youtube/confirm", strings.NewReader(`{"confirmationRef":"`+confirmation+`"}`)).WithContext(ctx)
	request.Header.Set("Origin", h.PublicOrigin)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Firebase-AppCheck", "synthetic")
	request.AddCookie(&http.Cookie{Name: "__session", Value: id})
	w, done := httptest.NewRecorder(), make(chan struct{})
	go func() { defer close(done); h.ServeHTTP(w, request) }()
	<-entered
	cancel()
	a, e := runtimeAdapter(r, now), runtimeEmulatorExecution(now)
	runtimeApply(t, a, e, "pause")
	_, err := a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
	doc, err := store.Client.Collection("oauth-transactions").Doc(id).Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "consumed", doc.Data()["status"], "consume commits before external mint")
	require.Positive(t, r.Observe(runtimeChannel).Active)
	close(release)
	<-done
	require.Equal(t, 503, w.Code)
	require.NotContains(t, w.Body.String(), "synthetic-late-token")
	runtimeApply(t, a, e, "drain")
	runtimeApply(t, a, e, "resume")
}

func TestRuntimeEmulatorLateMetadataResultCannotWriteAndFreshRefreshCan(t *testing.T) {
	s, store, _, _, now := authTestService(t)
	r := NewRuntimeRegistry("development", "demo-youtube-study-space-ci")
	initial := healthyAccount(now.Add(-25 * time.Hour))
	initial.PrivacyPolicyVersion, initial.TermsVersion = s.Policy.Privacy, s.Policy.Terms
	ref := store.Client.Collection("web-accounts").Doc(runtimeChannel)
	_, err := ref.Set(context.Background(), initial)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := ref.Delete(context.Background()); require.NoError(t, err) })
	expected, err := store.ReadAccount(context.Background(), runtimeChannel)
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	refresh := &AccountMetadataRefresh{Runtime: r, Access: s.Access, Store: store, Policy: s.Policy, Now: s.Now, Provider: metadataReaderFunc(func(context.Context, string) (Channel, error) {
		close(entered)
		<-release
		return Channel{ID: runtimeChannel, DisplayName: "Late synthetic metadata"}, nil
	})}
	done := make(chan error, 1)
	go func() { _, err := refresh.Refresh(context.Background(), runtimeChannel, expected); done <- err }()
	<-entered
	a, e := runtimeAdapter(r, now), runtimeEmulatorExecution(now)
	runtimeApply(t, a, e, "pause")
	close(release)
	require.ErrorIs(t, <-done, ErrRuntimeFenced)
	doc, err := ref.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Sample", doc.Data()["displayName"])
	runtimeApply(t, a, e, "drain")
	runtimeApply(t, a, e, "resume")
	refresh.Provider = metadataReaderFunc(func(context.Context, string) (Channel, error) {
		return Channel{ID: runtimeChannel, DisplayName: "Fresh synthetic metadata"}, nil
	})
	updated, err := refresh.Refresh(context.Background(), runtimeChannel, expected)
	require.NoError(t, err)
	require.Equal(t, "Fresh synthetic metadata", updated.DisplayName)
}

func TestRuntimeEmulatorCleanupLostAcknowledgmentCannotProveDrainFromMissingMetadata(t *testing.T) {
	integrationtest.ResetFirestore(t)
	s, store, _, _, now := authTestService(t)
	r := NewRuntimeRegistry("development", "demo-youtube-study-space-ci")
	ref := store.Client.Collection("web-accounts").Doc(runtimeChannel)
	initial := healthyAccount(now.Add(-MetadataCleanupAge))
	initial.PrivacyPolicyVersion, initial.TermsVersion = s.Policy.Privacy, s.Policy.Terms
	_, err := ref.Set(context.Background(), initial)
	require.NoError(t, err)
	t.Cleanup(func() { _, err := ref.Delete(context.Background()); require.NoError(t, err) })
	actual := &FirestoreMetadataCleanupStore{Client: store.Client, ProjectID: "demo-youtube-study-space-ci"}
	wrapper := cleanupStoreFake{scan: actual.Scan, clear: func(ctx context.Context, candidate MetadataCleanupCandidate, cutoff, at time.Time) (string, error) {
		outcome, err := actual.Clear(ctx, candidate, cutoff, at)
		if err != nil {
			return outcome, err
		}
		return "", ErrRuntimeOutcomeUnknown
	}}
	job := &MetadataCleanupJob{Runtime: r, Store: wrapper, Recorder: cleanupRecorderFunc(func(context.Context, MetadataCleanupObservation) error { return nil }), Now: s.Now, BatchSize: 100, MaxPages: 10}
	observation, err := job.Run(context.Background())
	require.Error(t, err)
	require.False(t, observation.Succeeded)
	doc, err := ref.Get(context.Background())
	require.NoError(t, err)
	require.NotContains(t, doc.Data(), "displayName", "actual commit is insufficient to infer complete worker drain")
	require.EqualValues(t, 1, r.Observe(runtimeChannel).Pending)
	require.Zero(t, r.Observe(runtimeChannel).Active)
	a, e := runtimeAdapter(r, now), runtimeEmulatorExecution(now)
	runtimeApply(t, a, e, "pause")
	_, err = a.Apply(context.Background(), runtimeOperation(e, "drain"))
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
	encoded, err := json.Marshal(observation)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), runtimeChannel)
}
