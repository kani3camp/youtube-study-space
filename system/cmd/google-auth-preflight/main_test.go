package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunRequiresExplicitTarget(t *testing.T) {
	err := run(context.Background(), []string{"google-auth-preflight"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestRunRejectsUnknownEnvironmentBeforeCredentialAccess(t *testing.T) {
	err := run(
		context.Background(),
		[]string{"google-auth-preflight", "staging", "project-id"},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "development or production") {
		t.Fatalf("expected environment validation error, got %v", err)
	}
}

func TestRunRejectsConfiguredProjectMismatchBeforeCredentialAccess(t *testing.T) {
	t.Setenv("AWS_PROFILE", "study-space-prod")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-youtube-study-space")
	t.Setenv("GCP_WIF_AUDIENCE", "audience")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "operator@example.iam.gserviceaccount.com")

	err := run(
		context.Background(),
		[]string{"google-auth-preflight", "production", "youtube-study-space"},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "GCP project mismatch") {
		t.Fatalf("expected project mismatch, got %v", err)
	}
}

func TestRunRejectsStaticAWSCredentialsBeforeCredentialAccess(t *testing.T) {
	t.Setenv("AWS_PROFILE", "study-space-dev")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-youtube-study-space")
	t.Setenv("GCP_WIF_AUDIENCE", "audience")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "operator@example.iam.gserviceaccount.com")

	err := run(
		context.Background(),
		[]string{"google-auth-preflight", "development", "test-youtube-study-space"},
		&bytes.Buffer{},
	)
	if err == nil || !strings.Contains(err.Error(), "must be unset") {
		t.Fatalf("expected static AWS credential rejection, got %v", err)
	}
}
