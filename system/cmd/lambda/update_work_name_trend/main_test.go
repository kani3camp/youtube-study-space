package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/api/option"
)

func TestUpdateWorkNameTrendDependencyErrorsDoNotLogPayloads(t *testing.T) {
	const privateValue = "PRIVATE_WORK_OR_PROVIDER_BODY_719"
	for _, tc := range []struct {
		name         string
		secretErr    error
		firestoreErr error
		initErr      error
		app          updateWorkNameTrendApp
		wantClass    string
	}{
		{name: "secret", secretErr: errors.New(privateValue), wantClass: "secret_fetch_failed"},
		{name: "firestore option", firestoreErr: errors.New(privateValue), wantClass: "firestore_option_failed"},
		{name: "workspace init", initErr: errors.New(privateValue), wantClass: "workspace_init_failed"},
		{name: "trend update", app: &mockUpdateTrendApp{updateErr: errors.New(privateValue)}, wantClass: "trend_update_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SECRET_NAME", "synthetic-secret-name")
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previousLogger) })
			restore := stubUpdateTrendDeps(t, tc.secretErr, tc.firestoreErr, tc.initErr, tc.app)
			t.Cleanup(restore)

			if err := UpdateWorkNameTrend(context.Background()); err != nil {
				t.Fatalf("expected handled error, got %v", err)
			}
			if !strings.Contains(logs.String(), `"error_class":"`+tc.wantClass+`"`) {
				t.Fatalf("missing safe error class: %s", logs.String())
			}
			if strings.Contains(logs.String(), privateValue) || strings.Contains(logs.String(), "synthetic-secret-name") {
				t.Fatalf("logs exposed synthetic private value: %s", logs.String())
			}
		})
	}
}

type mockUpdateTrendApp struct {
	updateErr error
	closed    bool
}

func (m *mockUpdateTrendApp) UpdateWorkNameTrend(ctx context.Context, apiKey string) error {
	return m.updateErr
}

func (m *mockUpdateTrendApp) CloseFirestoreClient() {
	m.closed = true
}

func TestUpdateWorkNameTrendSecretNameMissingReturnsNil(t *testing.T) {
	t.Setenv("SECRET_NAME", "")
	restore := stubUpdateTrendDeps(t, nil, nil, nil, nil)
	defer restore()

	if err := UpdateWorkNameTrend(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateWorkNameTrendSecretFetchFailureReturnsNil(t *testing.T) {
	t.Setenv("SECRET_NAME", "my-secret")
	restore := stubUpdateTrendDeps(t, errors.New("secret fetch failed"), nil, nil, nil)
	defer restore()

	if err := UpdateWorkNameTrend(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateWorkNameTrendFirestoreFailureReturnsNil(t *testing.T) {
	t.Setenv("SECRET_NAME", "my-secret")
	restore := stubUpdateTrendDeps(t, nil, errors.New("firestore failed"), nil, nil)
	defer restore()

	if err := UpdateWorkNameTrend(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateWorkNameTrendWorkspaceInitFailureReturnsNil(t *testing.T) {
	t.Setenv("SECRET_NAME", "my-secret")
	restore := stubUpdateTrendDeps(t, nil, nil, errors.New("workspace init"), nil)
	defer restore()

	if err := UpdateWorkNameTrend(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateWorkNameTrendUpdateFailureReturnsNil(t *testing.T) {
	t.Setenv("SECRET_NAME", "my-secret")
	app := &mockUpdateTrendApp{updateErr: errors.New("trend failed")}
	restore := stubUpdateTrendDeps(t, nil, nil, nil, app)
	defer restore()

	if err := UpdateWorkNameTrend(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient")
	}
}

func TestUpdateWorkNameTrendTimeoutReturnsNil(t *testing.T) {
	t.Setenv("SECRET_NAME", "my-secret")
	app := &mockUpdateTrendApp{
		updateErr: context.DeadlineExceeded,
	}
	restore := stubUpdateTrendDeps(t, nil, nil, nil, app)
	defer restore()

	if err := UpdateWorkNameTrend(context.Background()); err != nil {
		t.Fatalf("expected nil error on handled timeout, got %v", err)
	}
	if !app.closed {
		t.Fatal("expected CloseFirestoreClient")
	}
}

func stubUpdateTrendDeps(
	t *testing.T,
	secretErr error,
	firestoreErr error,
	newAppErr error,
	app updateWorkNameTrendApp,
) func() {
	t.Helper()
	origS := secretFieldFromSecretsManager
	origF := firestoreClientOptionTrend
	origN := newTrendWorkspaceApp

	secretFieldFromSecretsManager = func(ctx context.Context, secretName string, field string) (string, error) {
		if secretErr != nil {
			return "", secretErr
		}
		return "dummy-api-key", nil
	}
	firestoreClientOptionTrend = func() (option.ClientOption, error) {
		return option.WithoutAuthentication(), firestoreErr
	}
	newTrendWorkspaceApp = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (updateWorkNameTrendApp, error) {
		if newAppErr != nil {
			return nil, newAppErr
		}
		return app, nil
	}

	return func() {
		secretFieldFromSecretsManager = origS
		firestoreClientOptionTrend = origF
		newTrendWorkspaceApp = origN
	}
}
