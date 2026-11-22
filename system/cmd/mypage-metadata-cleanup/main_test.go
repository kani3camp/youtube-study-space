package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/mypage"
)

func TestParseModeAcceptsExactlyOneMode(t *testing.T) {
	for args, want := range map[string]string{"-mode=run": modeRun, "--mode=health": modeHealth} {
		got, err := parseMode([]string{args})
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	for _, args := range [][]string{
		nil,
		{""},
		{"run"},
		{"--help"},
		{"-mode="},
		{"-mode=RUN"},
		{"-mode=both"},
		{"-mode=run", "extra"},
		{"-mode=run", "-mode=health"},
		{"-mode=", "-mode=run"},
		{"-mode=run", "-dry-run"},
	} {
		_, err := parseMode(args)
		require.ErrorIs(t, err, errInvalidArguments, "%q", args)
	}
}

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestResolveTargetPinsEnvironmentProjectAndRegion(t *testing.T) {
	for env, want := range map[string]target{
		"development": {"test-youtube-study-space", "asia-southeast2", "mypage-metadata-cleanup"},
		"production":  {"youtube-study-space", "asia-northeast2", "mypage-metadata-cleanup"},
	} {
		got, err := resolveTarget(envFrom(map[string]string{"MYPAGE_ENVIRONMENT": env, "GOOGLE_CLOUD_PROJECT": want.project, "CLOUD_RUN_JOB": want.job}))
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	for _, values := range []map[string]string{
		{},
		{"MYPAGE_ENVIRONMENT": "staging", "GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "CLOUD_RUN_JOB": "job"},
		{"MYPAGE_ENVIRONMENT": "development", "GOOGLE_CLOUD_PROJECT": "youtube-study-space", "CLOUD_RUN_JOB": "job"},
		{"MYPAGE_ENVIRONMENT": "production", "GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "CLOUD_RUN_JOB": "job"},
		{"MYPAGE_ENVIRONMENT": "production", "GOOGLE_CLOUD_PROJECT": "youtube-study-space"},
	} {
		_, err := resolveTarget(envFrom(values))
		require.Error(t, err, "%v", values)
	}
}

// 認証情報探索や Firestore 接続より前に、引数・実行先・ジョブ名の不備で失敗する。
func TestRunMainRejectsBeforeCreatingClients(t *testing.T) {
	valid := map[string]string{"MYPAGE_ENVIRONMENT": "development", "GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "CLOUD_RUN_JOB": "mypage-metadata-cleanup"}
	for _, tt := range []struct {
		args []string
		env  map[string]string
	}{
		{nil, valid},
		{[]string{"-mode=run", "extra"}, valid},
		{[]string{"-mode=run"}, map[string]string{"MYPAGE_ENVIRONMENT": "development", "GOOGLE_CLOUD_PROJECT": "youtube-study-space", "CLOUD_RUN_JOB": "job"}},
		{[]string{"-mode=health"}, map[string]string{"MYPAGE_ENVIRONMENT": "development", "GOOGLE_CLOUD_PROJECT": "test-youtube-study-space", "CLOUD_RUN_JOB": "Invalid_Job"}},
	} {
		var output bytes.Buffer
		status := runMain(tt.args, envFrom(tt.env), &output)
		require.NotEmpty(t, status)
		require.Empty(t, output.String())
	}
}

func TestTimeoutsLeaveRoomForPostProcessing(t *testing.T) {
	require.Greater(t, timeoutFor(modeRun), mypage.MetadataCleanupBudget)
	require.LessOrEqual(t, timeoutFor(modeRun), mypage.MetadataCleanupBudget+2*time.Minute)
	require.Equal(t, 30*time.Second, timeoutFor(modeHealth))
}

type runnerFake struct {
	observation mypage.MetadataCleanupObservation
	signal      mypage.MetadataCleanupHealthObservation
	err         error
	runs        int
	checks      int
}

func (f *runnerFake) Run(context.Context) (mypage.MetadataCleanupObservation, error) {
	f.runs++
	return f.observation, f.err
}

func (f *runnerFake) CheckHealth(context.Context) (mypage.MetadataCleanupHealthObservation, error) {
	f.checks++
	return f.signal, f.err
}

func decodeKeys(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	return got
}

func TestExecuteWritesFixedShapeResultAndKeepsModesIndependent(t *testing.T) {
	at := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	failure := errors.New("synthetic-private-channel-UCsynthetic0000000000001")
	fake := &runnerFake{
		observation: mypage.MetadataCleanupObservation{StartedAt: at, CompletedAt: at.Add(time.Minute), Stage: "clear", Scanned: 2, Cleared: 1, Failed: 1},
		signal:      mypage.MetadataCleanupHealthObservation{CheckedAt: at, Alert: true, Reason: "run_failed"},
		err:         failure,
	}
	var output bytes.Buffer
	require.ErrorIs(t, execute(context.Background(), modeRun, fake, &output), failure)
	require.Equal(t, 1, fake.runs)
	require.Zero(t, fake.checks, "run mode must not trigger the independent watchdog directly")
	require.Equal(t, map[string]any{
		"mode": "run", "startedAt": "2026-10-14T00:00:00Z", "completedAt": "2026-10-14T00:01:00Z",
		"succeeded": false, "stage": "clear", "scanned": float64(2), "cleared": float64(1), "changed": float64(0),
		"missing": float64(0), "failed": float64(1), "retentionViolations": float64(0),
	}, decodeKeys(t, output.Bytes()))
	require.NotContains(t, output.String(), "synthetic-private")

	output.Reset()
	fake.err = nil
	require.NoError(t, execute(context.Background(), modeHealth, fake, &output))
	require.Equal(t, 1, fake.runs, "health mode must never run a cleanup")
	require.Equal(t, 1, fake.checks)
	require.Equal(t, map[string]any{"mode": "health", "checkedAt": "2026-10-14T00:00:00Z", "alert": true, "reason": "run_failed"}, decodeKeys(t, output.Bytes()))
}
