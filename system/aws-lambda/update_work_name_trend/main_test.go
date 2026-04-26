package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/api/option"
)

type mockUpdateTrendApp struct {
	updateErr        error
	notifyTimeoutErr error
	closed           bool
}

func (m *mockUpdateTrendApp) UpdateWorkNameTrend(ctx context.Context, apiKey string) error {
	return m.updateErr
}

func (m *mockUpdateTrendApp) NotifyTimeoutToOwner(ctx context.Context, err error) error {
	if m.notifyTimeoutErr != nil {
		return m.notifyTimeoutErr
	}
	return nil
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
	restore := stubUpdateTrendDeps(t, nil, errors.New("dynamo failed"), nil, nil)
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

func TestUpdateWorkNameTrendTimeoutNotifyFailureReturnsError(t *testing.T) {
	t.Setenv("SECRET_NAME", "my-secret")
	app := &mockUpdateTrendApp{
		updateErr:        context.DeadlineExceeded,
		notifyTimeoutErr: errors.New("notify failed"),
	}
	restore := stubUpdateTrendDeps(t, nil, nil, nil, app)
	defer restore()

	err := UpdateWorkNameTrend(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "timeout notification failed") {
		t.Fatalf("unexpected error: %v", err)
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
