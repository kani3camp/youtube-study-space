package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/mypage"
)

func inputFailureObservation(t *testing.T) mypage.OAuthFailureObservation {
	t.Helper()
	queue := make(chan mypage.OAuthFailureObservation, 1)
	service := &mypage.AuthService{OAuthFailures: queue}
	err := service.Callback(context.Background(), "", "state-secret", "code-secret", false)
	require.Error(t, err)
	return <-queue
}

func TestOAuthFailureLogHasOnlyFixedStageAndCode(t *testing.T) {
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	logOAuthFailure(logger, inputFailureObservation(t))
	require.Equal(t, "mypage_oauth_failure stage=callback_input code=OAUTH_TRANSACTION_REQUIRED\n", output.String())
	output.Reset()
	logOAuthFailure(logger, mypage.OAuthFailureObservation{})
	require.Equal(t, "mypage_oauth_failure stage=unknown code=INTERNAL_ERROR\n", output.String())
}

type blockedDiagnosticWriter struct {
	entered chan struct{}
	release chan struct{}
}

func (w blockedDiagnosticWriter) Write(p []byte) (int, error) {
	select {
	case w.entered <- struct{}{}:
	default:
	}
	<-w.release
	return len(p), nil
}

func TestOAuthFailureBlockedLogDoesNotDelayAuthenticationOrGrowQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	writer := blockedDiagnosticWriter{entered: make(chan struct{}, 1), release: make(chan struct{})}
	defer close(writer.release)
	defer cancel()
	queue := oauthFailureDiagnostics(ctx, log.New(writer, "", 0))
	require.Equal(t, 32, cap(queue))
	queue <- inputFailureObservation(t)
	select {
	case <-writer.entered:
	case <-time.After(time.Second):
		t.Fatal("diagnostic worker did not receive event")
	}
	service := &mypage.AuthService{OAuthFailures: queue}
	done := make(chan bool, 1)
	go func() {
		for range 64 {
			if service.Callback(context.Background(), "", "state-secret", "code-secret", false) == nil {
				done <- false
				return
			}
		}
		done <- true
	}()
	select {
	case unchanged := <-done:
		require.True(t, unchanged)
	case <-time.After(time.Second):
		t.Fatal("blocked logger delayed authentication")
	}
	require.Len(t, queue, cap(queue), "excess events must be dropped")
}

func TestOAuthFailureDiagnosticsWiredOnlyForDevelopment(t *testing.T) {
	for _, environment := range []string{"development", "production"} {
		t.Run(environment, func(t *testing.T) {
			values := syntheticConfiguration()
			values["MYPAGE_ENVIRONMENT"] = environment
			if environment == "production" {
				values["MYPAGE_REGION"] = "asia-northeast2"
			}
			var events []string
			err := runWith(context.Background(), func(key string) string { return values[key] }, syntheticBootstrap(t, "", &events), func(_ context.Context, server *http.Server) error {
				handler, ok := server.Handler.(*mypage.HTTPHandler)
				require.True(t, ok)
				if environment == "development" {
					require.NotNil(t, handler.Auth.OAuthFailures)
				} else {
					require.Nil(t, handler.Auth.OAuthFailures)
				}
				return nil
			})
			require.NoError(t, err)
		})
	}
}

type panickingDiagnosticWriter struct{ entered chan struct{} }

func (w panickingDiagnosticWriter) Write([]byte) (int, error) {
	close(w.entered)
	panic("DO_NOT_LOG diagnostic-writer-secret")
}

func TestOAuthFailureLogPanicCannotCrashAuthentication(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := panickingDiagnosticWriter{entered: make(chan struct{})}
	queue := oauthFailureDiagnostics(ctx, log.New(writer, "", 0))
	queue <- inputFailureObservation(t)
	select {
	case <-writer.entered:
	case <-time.After(time.Second):
		t.Fatal("diagnostic sink was not called")
	}
	service := &mypage.AuthService{OAuthFailures: queue}
	for range 64 {
		require.Error(t, service.Callback(context.Background(), "", "state-secret", "code-secret", false))
	}
}
