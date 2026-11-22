package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const syntheticCleanupLogToken = "synthetic-access-token-value"

func cloudLoggingRecorderFor(server *httptest.Server) *CloudLoggingMetadataCleanupHealthRecorder {
	return &CloudLoggingMetadataCleanupHealthRecorder{
		ProjectID: "test-youtube-study-space", Location: "asia-southeast2", JobName: "mypage-metadata-cleanup",
		Tokens:   oauth2.StaticTokenSource(&oauth2.Token{AccessToken: syntheticCleanupLogToken, TokenType: "Bearer"}),
		endpoint: server.URL + "/v2/entries:write",
	}
}

type cleanupTokenSourceFunc func() (*oauth2.Token, error)

func (f cleanupTokenSourceFunc) Token() (*oauth2.Token, error) { return f() }

func TestCloudLoggingHealthRecorderWritesOnlyFixedEntryShape(t *testing.T) {
	checkedAt := time.Date(2026, 10, 14, 9, 0, 0, 123456789, time.FixedZone("JST", 9*60*60))
	for _, tt := range []struct {
		signal   MetadataCleanupHealthObservation
		severity string
	}{
		{MetadataCleanupHealthObservation{CheckedAt: checkedAt, Reason: "healthy"}, "INFO"},
		{MetadataCleanupHealthObservation{CheckedAt: checkedAt, Alert: true, Reason: "retention_deadline"}, "WARNING"},
	} {
		t.Run(tt.signal.Reason, func(t *testing.T) {
			var mu sync.Mutex
			var got map[string]any
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/v2/entries:write" ||
					r.Header.Get("Authorization") != "Bearer "+syntheticCleanupLogToken || r.Header.Get("Content-Type") != "application/json" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if _, err := w.Write([]byte(`{}`)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			require.NoError(t, cloudLoggingRecorderFor(server).RecordHealth(context.Background(), tt.signal))
			require.EqualValues(t, 1, calls.Load())
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, map[string]any{"entries": []any{map[string]any{
				"logName": "projects/test-youtube-study-space/logs/mypage-metadata-cleanup-health",
				"resource": map[string]any{"type": "cloud_run_job", "labels": map[string]any{
					"job_name": "mypage-metadata-cleanup", "location": "asia-southeast2", "project_id": "test-youtube-study-space",
				}},
				"severity": tt.severity,
				"jsonPayload": map[string]any{
					"checkedAt": "2026-10-14T00:00:00.123456789Z", "alert": tt.signal.Alert, "reason": tt.signal.Reason,
				},
			}}}, got)
		})
	}
}

func TestCloudLoggingHealthRecorderRejectsBeforeAnyRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	for _, bad := range []MetadataCleanupHealthObservation{
		{CheckedAt: now, Alert: true, Reason: "synthetic-private-id"},
		{CheckedAt: now, Alert: false, Reason: "run_failed"},
		{CheckedAt: now, Alert: true, Reason: "healthy"},
		{Alert: true, Reason: "run_failed"},
	} {
		require.ErrorIs(t, cloudLoggingRecorderFor(server).RecordHealth(context.Background(), bad), ErrMetadataCleanupIncomplete)
	}
	healthy := MetadataCleanupHealthObservation{CheckedAt: now, Reason: "healthy"}
	for _, mutate := range []func(*CloudLoggingMetadataCleanupHealthRecorder){
		func(r *CloudLoggingMetadataCleanupHealthRecorder) { r.ProjectID = "" },
		func(r *CloudLoggingMetadataCleanupHealthRecorder) { r.ProjectID = "test-youtube-study-space/logs/x" },
		func(r *CloudLoggingMetadataCleanupHealthRecorder) { r.Location = "" },
		func(r *CloudLoggingMetadataCleanupHealthRecorder) { r.JobName = "" },
		func(r *CloudLoggingMetadataCleanupHealthRecorder) { r.JobName = "Invalid_Job" },
		func(r *CloudLoggingMetadataCleanupHealthRecorder) { r.Tokens = nil },
	} {
		recorder := cloudLoggingRecorderFor(server)
		mutate(recorder)
		require.ErrorIs(t, recorder.RecordHealth(context.Background(), healthy), ErrMetadataCleanupIncomplete)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, cloudLoggingRecorderFor(server).RecordHealth(canceled, healthy), context.Canceled)
	recorder := cloudLoggingRecorderFor(server)
	recorder.Tokens = cleanupTokenSourceFunc(func() (*oauth2.Token, error) {
		return nil, errors.New("synthetic-private-credential-response")
	})
	err := recorder.RecordHealth(context.Background(), healthy)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "synthetic-private")
	require.Zero(t, calls.Load(), "rejected signals must not reach Cloud Logging")
}

func TestCloudLoggingHealthRecorderFailsWithoutLeakingResponse(t *testing.T) {
	healthy := MetadataCleanupHealthObservation{CheckedAt: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), Reason: "healthy"}
	t.Run("non-2xx", func(t *testing.T) {
		for _, code := range []int{http.StatusForbidden, http.StatusInternalServerError, http.StatusTemporaryRedirect} {
			var redirected atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(code)
				if _, err := w.Write([]byte("synthetic-private-response-body")); err != nil {
					t.Error(err)
				}
			}))
			err := cloudLoggingRecorderFor(server).RecordHealth(context.Background(), healthy)
			server.Close()
			target.Close()
			require.Error(t, err)
			require.Contains(t, err.Error(), fmt.Sprintf("status %d", code))
			require.NotContains(t, err.Error(), "synthetic-private")
			require.NotContains(t, err.Error(), syntheticCleanupLogToken)
			require.Zero(t, redirected.Load(), "the bearer token must not follow redirects")
		}
	})
	t.Run("injected-client-cannot-follow-redirects", func(t *testing.T) {
		var redirected atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
		defer target.Close()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
		}))
		defer server.Close()
		recorder := cloudLoggingRecorderFor(server)
		recorder.client = &http.Client{}
		require.Error(t, recorder.RecordHealth(context.Background(), healthy))
		require.Zero(t, redirected.Load())
	})
	t.Run("truncated-2xx-is-not-accepted", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// 宣言より短い本文で切断し、2xx でも受理を確認できない応答を作る。
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("{")); err != nil {
				t.Error(err)
			}
		}))
		defer server.Close()
		err := cloudLoggingRecorderFor(server).RecordHealth(context.Background(), healthy)
		require.Error(t, err)
		require.Contains(t, err.Error(), "read cleanup health log response")
	})
	t.Run("deadline", func(t *testing.T) {
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		defer server.Close()
		defer close(release)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, cloudLoggingRecorderFor(server).RecordHealth(ctx, healthy), context.DeadlineExceeded)
	})
	t.Run("token-deadline", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
		defer server.Close()
		release := make(chan struct{})
		defer close(release)
		recorder := cloudLoggingRecorderFor(server)
		recorder.Tokens = cleanupTokenSourceFunc(func() (*oauth2.Token, error) {
			<-release
			return nil, errors.New("late")
		})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, recorder.RecordHealth(ctx, healthy), context.DeadlineExceeded)
		require.Zero(t, calls.Load())
	})
}

// Cloud Logging が受理（2xx）した後にだけ保持期限違反を ACK し、拒否時は再通知できるよう保留を残す。
func TestCloudLoggingHealthRecorderGatesRetentionAcknowledgement(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	violation := cleanupCompleteObservation(now.Add(-time.Hour))
	violation.Scanned, violation.Cleared, violation.RetentionViolations = 1, 1, 1
	monitor := &cleanupMonitorFake{}
	require.NoError(t, monitor.Record(context.Background(), violation))
	var status atomic.Int32
	status.Store(http.StatusServiceUnavailable)
	var mu sync.Mutex
	var reasons []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body cloudLoggingWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && len(body.Entries) == 1 {
			mu.Lock()
			reasons = append(reasons, body.Entries[0].JSONPayload.Reason)
			mu.Unlock()
		}
		w.WriteHeader(int(status.Load()))
	}))
	defer server.Close()
	runner := MetadataCleanupRunner{Monitor: monitor, Signals: cloudLoggingRecorderFor(server), Now: func() time.Time { return now }}
	signal, err := runner.CheckHealth(context.Background())
	require.Error(t, err)
	require.Equal(t, "retention_deadline", signal.Reason)
	require.Equal(t, violation.StartedAt, monitor.state.PendingRetentionStartedAt, "rejected signal must stay pending")
	status.Store(http.StatusOK)
	signal, err = runner.CheckHealth(context.Background())
	require.NoError(t, err)
	require.Equal(t, "retention_deadline", signal.Reason)
	require.True(t, monitor.state.PendingRetentionStartedAt.IsZero())
	require.NoError(t, monitor.Record(context.Background(), cleanupCompleteObservation(now.Add(-time.Minute))))
	signal, err = runner.CheckHealth(context.Background())
	require.NoError(t, err)
	require.Equal(t, "healthy", signal.Reason)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"retention_deadline", "retention_deadline", "healthy"}, reasons)
}
