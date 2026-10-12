package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"app.modules/core/mypage"
)

func syntheticBootstrap(t *testing.T, fail string, events *[]string) bootstrapDependencies {
	t.Helper()
	credentials := &google.Credentials{ProjectID: "demo-mypage", TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "synthetic-adc"})}
	var bootstrapCtx context.Context
	stage := func(ctx context.Context, name string) error {
		*events = append(*events, name)
		deadline, ok := ctx.Deadline()
		require.True(t, ok, "bootstrap must have a deadline")
		require.LessOrEqual(t, time.Until(deadline), 10*time.Second)
		if bootstrapCtx == nil {
			bootstrapCtx = ctx
		} else {
			require.Equal(t, bootstrapCtx, ctx, "bootstrap stages share one budget")
		}
		if name == fail {
			return errors.New("synthetic-dependency-secret")
		}
		return nil
	}
	return bootstrapDependencies{
		credentials: func(ctx context.Context) (*google.Credentials, error) {
			return credentials, stage(ctx, "credentials")
		},
		firebase: func(ctx context.Context, config configuration, got *google.Credentials) (mypage.FirebaseAuthClient, error) {
			require.Same(t, credentials, got)
			require.Equal(t, "demo-mypage", config.server.ProjectID)
			require.Equal(t, "mypage-signer@demo-mypage.iam.gserviceaccount.com", config.signer)
			return syntheticFirebase{t: t}, stage(ctx, "firebase")
		},
		database: func(ctx context.Context, config configuration, got *google.Credentials) (*firestore.Client, func(), error) {
			require.Same(t, credentials, got)
			require.Equal(t, "demo-mypage", config.server.ProjectID)
			if err := stage(ctx, "database"); err != nil {
				return nil, nil, err
			}
			// Handler assembly is lazy; the zero client must never be used for IO.
			return &firestore.Client{}, func() { *events = append(*events, "database closed") }, nil
		},
	}
}

func TestStartupRejectsConfigurationBeforeAnyExternalBootstrap(t *testing.T) {
	values := syntheticConfiguration()
	values["MYPAGE_INFRASTRUCTURE_READY"] = ""
	var events []string
	err := runWith(context.Background(), func(key string) string { return values[key] }, syntheticBootstrap(t, "", &events), func(context.Context, *http.Server) error {
		t.Fatal("invalid configuration reached HTTP startup")
		return nil
	})
	require.EqualError(t, err, "MyPage configuration unavailable")
	require.Empty(t, events)
}

func TestStartupFailuresStopAtTheirStageAndHideDependencyDetails(t *testing.T) {
	for _, tc := range []struct {
		stage, diagnostic string
		events            []string
	}{
		{"credentials", "MyPage keyless bootstrap unavailable", []string{"credentials"}},
		{"firebase", "MyPage keyless bootstrap unavailable", []string{"credentials", "firebase"}},
		{"database", "MyPage database bootstrap unavailable", []string{"credentials", "firebase", "database"}},
	} {
		t.Run(tc.stage, func(t *testing.T) {
			values := syntheticConfiguration()
			var events []string
			err := runWith(context.Background(), func(key string) string { return values[key] }, syntheticBootstrap(t, tc.stage, &events), func(context.Context, *http.Server) error {
				t.Fatal("failed bootstrap reached HTTP startup")
				return nil
			})
			require.EqualError(t, err, tc.diagnostic)
			require.Equal(t, tc.events, events)
		})
	}
}

func TestStartupAssemblesBoundedHTTPServerAndClosesDatabaseAfterServing(t *testing.T) {
	for _, metadataKey := range []string{"", "synthetic-api-key"} {
		t.Run("metadata="+metadataKey, func(t *testing.T) {
			transport := http.DefaultTransport
			http.DefaultTransport = rejectingTransport{t: t}
			t.Cleanup(func() { http.DefaultTransport = transport })
			values := syntheticConfiguration()
			values["PORT"] = "18080"
			values["MYPAGE_YOUTUBE_API_KEY"] = metadataKey
			var events []string
			err := runWith(context.Background(), func(key string) string { return values[key] }, syntheticBootstrap(t, "", &events), func(ctx context.Context, server *http.Server) error {
				require.NoError(t, ctx.Err())
				require.Equal(t, ":18080", server.Addr)
				require.Equal(t, 5*time.Second, server.ReadHeaderTimeout)
				require.Equal(t, 10*time.Second, server.ReadTimeout)
				require.Equal(t, 25*time.Second, server.WriteTimeout)
				require.Equal(t, 60*time.Second, server.IdleTimeout)
				require.Equal(t, 16384, server.MaxHeaderBytes)
				require.Equal(t, io.Discard, server.ErrorLog.Writer())
				handler, ok := server.Handler.(*mypage.HTTPHandler)
				require.True(t, ok)
				require.Nil(t, handler.Intake, "Oct9 automated Privacy intake remains off")
				require.Equal(t, "https://example.invalid", handler.PublicOrigin)
				require.Same(t, handler.Verifier, handler.Auth.Minter)
				if metadataKey == "" {
					require.Nil(t, handler.BFF.Metadata)
				} else {
					require.NotNil(t, handler.BFF.Metadata)
				}
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "https://example.invalid/api/unknown", nil))
				require.Equal(t, http.StatusNotFound, response.Code)
				require.NotContains(t, response.Body.String(), "synthetic")
				events = append(events, "serve")
				if metadataKey == "" {
					return nil
				}
				return errors.New("MyPage HTTP server unavailable")
			})
			if metadataKey == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, "MyPage HTTP server unavailable")
			}
			require.Equal(t, []string{"credentials", "firebase", "database", "serve", "database closed"}, events)
		})
	}
}

func TestStartupClosesDatabaseWhenHandlerAssemblyFails(t *testing.T) {
	values := syntheticConfiguration()
	var events []string
	deps := syntheticBootstrap(t, "", &events)
	deps.firebase = func(context.Context, configuration, *google.Credentials) (mypage.FirebaseAuthClient, error) {
		events = append(events, "firebase")
		return nil, nil
	}
	err := runWith(context.Background(), func(key string) string { return values[key] }, deps, func(context.Context, *http.Server) error {
		t.Fatal("invalid handler reached HTTP startup")
		return nil
	})
	require.EqualError(t, err, "MyPage handler unavailable")
	require.Equal(t, []string{"credentials", "firebase", "database", "database closed"}, events)
}

func TestStartupAlreadyCanceledDoesNotDiscoverCredentials(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	values := syntheticConfiguration()
	var events []string
	err := runWith(ctx, func(key string) string { return values[key] }, syntheticBootstrap(t, "", &events), func(context.Context, *http.Server) error {
		t.Fatal("canceled startup reached HTTP")
		return nil
	})
	require.EqualError(t, err, "MyPage keyless bootstrap unavailable")
	require.Empty(t, events)
}

func TestStartupHonorsCanceledBudgetEvenWhenDependencyReturnsSuccess(t *testing.T) {
	for _, stage := range []string{"credentials", "firebase", "database"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			values := syntheticConfiguration()
			var events []string
			deps := syntheticBootstrap(t, "", &events)
			switch stage {
			case "credentials":
				original := deps.credentials
				deps.credentials = func(ctx context.Context) (*google.Credentials, error) {
					result, err := original(ctx)
					cancel()
					return result, err
				}
			case "firebase":
				original := deps.firebase
				deps.firebase = func(ctx context.Context, config configuration, credentials *google.Credentials) (mypage.FirebaseAuthClient, error) {
					result, err := original(ctx, config, credentials)
					cancel()
					return result, err
				}
			case "database":
				original := deps.database
				deps.database = func(ctx context.Context, config configuration, credentials *google.Credentials) (*firestore.Client, func(), error) {
					result, closeDatabase, err := original(ctx, config, credentials)
					cancel()
					return result, closeDatabase, err
				}
			}
			err := runWith(ctx, func(key string) string { return values[key] }, deps, func(context.Context, *http.Server) error {
				events = append(events, "serve")
				return nil
			})
			require.Error(t, err, "canceled bootstrap must not start HTTP")
			require.NotContains(t, events, "serve")
			switch stage {
			case "credentials":
				require.Equal(t, []string{"credentials"}, events)
			case "firebase":
				require.Equal(t, []string{"credentials", "firebase"}, events)
			case "database":
				require.Equal(t, []string{"credentials", "firebase", "database", "database closed"}, events)
			}
		})
	}
}

func TestStartupParentDeadlineStopsBlockedCredentialDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	values := syntheticConfiguration()
	var events []string
	deps := syntheticBootstrap(t, "", &events)
	deps.credentials = func(ctx context.Context) (*google.Credentials, error) {
		<-ctx.Done()
		return nil, errors.New("synthetic-timeout-detail")
	}
	err := runWith(ctx, func(key string) string { return values[key] }, deps, func(context.Context, *http.Server) error {
		t.Fatal("expired bootstrap reached HTTP startup")
		return nil
	})
	require.EqualError(t, err, "MyPage keyless bootstrap unavailable")
	require.Empty(t, events)
}

func TestHTTPListenerFailureReturnsFixedDiagnosticAndTerminatesWatcher(t *testing.T) {
	listener := localListener(t)
	server := &http.Server{Addr: listener.Addr().String(), ReadHeaderTimeout: time.Second}
	err := serveHTTP(context.Background(), server, server.ListenAndServe, time.Second)
	require.EqualError(t, err, "MyPage HTTP server unavailable")
}

func TestHTTPRuntimeRejectsOversizedAndStalledHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := localListener(t)
	values := syntheticConfiguration()
	var events []string
	deps := syntheticBootstrap(t, "", &events)
	result := make(chan error, 1)
	go func() {
		result <- runWith(ctx, func(key string) string { return values[key] }, deps, func(ctx context.Context, server *http.Server) error {
			return serveHTTP(ctx, server, func() error { return server.Serve(listener) }, time.Second)
		})
	}()
	for _, tc := range []struct {
		name, request string
		status        int
	}{
		{"oversized", "GET /api/unknown HTTP/1.1\r\nHost: example.invalid\r\nX-Synthetic: " + strings.Repeat("x", 24000) + "\r\n\r\n", http.StatusRequestHeaderFieldsTooLarge},
		{"stalled", "GET /api/unknown HTTP/1.1\r\nHost: example.invalid\r\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			require.NoError(t, err)
			t.Cleanup(func() {
				connection.Close() //nolint:errcheck // Test cleanup may close an already-closed synthetic connection.
			})
			require.NoError(t, connection.SetDeadline(time.Now().Add(8*time.Second)))
			_, err = io.WriteString(connection, tc.request)
			require.NoError(t, err)
			started := time.Now()
			response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodGet})
			if tc.status == 0 {
				// net/http closes a header-timeout connection without a response.
				require.ErrorIs(t, err, io.ErrUnexpectedEOF)
				require.GreaterOrEqual(t, time.Since(started), 4*time.Second)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.status, response.StatusCode)
			require.NoError(t, response.Body.Close())
		})
	}
	cancel()
	require.NoError(t, waitResult(t, result))
	require.Equal(t, []string{"credentials", "firebase", "database", "database closed"}, events)
}

func TestHTTPShutdownWaitsForInflightRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := &closeObservedListener{Listener: localListener(t), closed: make(chan struct{})}
	started, release := make(chan struct{}), make(chan struct{})
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})}
	t.Cleanup(func() { require.NoError(t, server.Close()) })
	result := make(chan error, 1)
	go func() { result <- serveHTTP(ctx, server, func() error { return server.Serve(listener) }, time.Second) }()
	response := requestLocal(listener)
	waitClosed(t, started)
	cancel()
	waitClosed(t, listener.closed)
	select {
	case err := <-result:
		close(release)
		t.Fatalf("shutdown returned before request completed: %v", err)
	default:
	}
	close(release)
	require.NoError(t, waitResult(t, result))
	require.NoError(t, waitResult(t, response))
}

func TestHTTPShutdownTimeoutClosesActiveConnectionAndReportsFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := localListener(t)
	started, canceled := make(chan struct{}), make(chan struct{})
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})}
	t.Cleanup(func() { require.NoError(t, server.Close()) })
	result := make(chan error, 1)
	go func() {
		result <- serveHTTP(ctx, server, func() error { return server.Serve(listener) }, 20*time.Millisecond)
	}()
	response := requestLocal(listener)
	waitClosed(t, started)
	cancel()
	require.EqualError(t, waitResult(t, result), "MyPage HTTP server unavailable")
	waitClosed(t, canceled)
	require.Error(t, waitResult(t, response), "timed-out active connection must be closed")
}

func TestHTTPServeFailureClosesActiveConnectionBeforeReturning(t *testing.T) {
	fail := make(chan struct{})
	listener := &failAfterAcceptListener{Listener: localListener(t), fail: fail}
	started, canceled := make(chan struct{}), make(chan struct{})
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})}
	t.Cleanup(func() { require.NoError(t, server.Close()) })
	result := make(chan error, 1)
	go func() {
		result <- serveHTTP(context.Background(), server, func() error { return server.Serve(listener) }, time.Second)
	}()
	response := requestLocal(listener)
	waitClosed(t, started)
	close(fail)
	require.EqualError(t, waitResult(t, result), "MyPage HTTP server unavailable")
	waitClosed(t, canceled)
	require.Error(t, waitResult(t, response), "accept failure must close the active connection")
}

func localListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		listener.Close() //nolint:errcheck // The server may have already closed this test-only listener.
	})
	return listener
}

func requestLocal(listener net.Listener) <-chan error {
	result := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
		defer client.CloseIdleConnections()
		response, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			err = response.Body.Close()
		}
		result <- err
	}()
	return result
}

func waitClosed(t *testing.T, channel <-chan struct{}) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle signal timed out")
	}
}

func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle result timed out")
		return nil
	}
}

type closeObservedListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

type failAfterAcceptListener struct {
	net.Listener
	fail     <-chan struct{}
	accepted bool
}

func (l *failAfterAcceptListener) Accept() (net.Conn, error) {
	if l.accepted {
		<-l.fail
		return nil, errors.New("synthetic-accept-error-detail")
	}
	l.accepted = true
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept synthetic HTTP connection: %w", err)
	}
	return connection, nil
}

func (l *closeObservedListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.closed) })
	if err != nil {
		return fmt.Errorf("close synthetic HTTP listener: %w", err)
	}
	return nil
}

type syntheticFirebase struct{ t *testing.T }

func (f syntheticFirebase) VerifyIDToken(context.Context, string) (*auth.Token, error) {
	f.t.Fatal("bootstrap attempted live token verification")
	return nil, errors.New("synthetic-only")
}

func (f syntheticFirebase) CustomToken(context.Context, string) (string, error) {
	f.t.Fatal("bootstrap attempted token minting")
	return "", errors.New("synthetic-only")
}
