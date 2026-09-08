package main

import (
	"strings"
	"testing"

	"app.modules/internal/googleauth"
)

func TestValidateYoutubeBotTarget(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		projectID   string
	}{
		{
			name:        "development",
			environment: "development",
			projectID:   googleauth.DevelopmentProjectID,
		},
		{
			name:        "production",
			environment: "production",
			projectID:   googleauth.ProductionProjectID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateYoutubeBotTarget(tt.environment, tt.projectID); err != nil {
				t.Fatalf("validateYoutubeBotTarget returned error: %v", err)
			}
		})
	}
}

func TestValidateYoutubeBotTargetRejectsUnknownEnvironment(t *testing.T) {
	err := validateYoutubeBotTarget("disabled", googleauth.DevelopmentProjectID)
	if err == nil || !strings.Contains(err.Error(), youtubeBotEnvironmentEnv) {
		t.Fatalf("expected environment error, got %v", err)
	}
}

func TestValidateYoutubeBotTargetRejectsProjectMismatch(t *testing.T) {
	err := validateYoutubeBotTarget("development", googleauth.ProductionProjectID)
	if err == nil || !strings.Contains(err.Error(), "environment/project mismatch") {
		t.Fatalf("expected project mismatch, got %v", err)
	}
}

func TestValidateYoutubeBotTargetRequiresProjectID(t *testing.T) {
	err := validateYoutubeBotTarget("development", "")
	if err == nil || !strings.Contains(err.Error(), googleCloudProjectEnvName) {
		t.Fatalf("expected missing project ID error, got %v", err)
	}
}

func TestInitGoogleClientRejectsUnknownModeBeforeCredentialAccess(t *testing.T) {
	t.Setenv(youtubeBotAuthModeEnv, "unknown")

	_, _, err := initGoogleClient(t.Context())
	if err == nil || !strings.Contains(err.Error(), youtubeBotAuthModeEnv) {
		t.Fatalf("expected auth mode error, got %v", err)
	}
}

func TestInitGoogleClientWIFRejectsTargetBeforeAWSCredentialAccess(t *testing.T) {
	t.Setenv(youtubeBotAuthModeEnv, youtubeBotAuthModeWIF)
	t.Setenv(youtubeBotEnvironmentEnv, "development")
	t.Setenv(googleCloudProjectEnvName, googleauth.ProductionProjectID)
	t.Setenv("GCP_WIF_AUDIENCE", "")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "")

	_, _, err := initGoogleClient(t.Context())
	if err == nil || !strings.Contains(err.Error(), "environment/project mismatch") {
		t.Fatalf("expected target mismatch before WIF config access, got %v", err)
	}
}
