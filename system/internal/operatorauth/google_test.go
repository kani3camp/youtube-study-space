package operatorauth

import (
	"strings"
	"testing"
)

func clearStaticAWSEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")
}

func TestGoogleConfigFromEnv(t *testing.T) {
	clearStaticAWSEnv(t)
	t.Setenv("AWS_PROFILE", "broad-profile-that-must-be-ignored")
	t.Setenv("GOOGLE_CLOUD_PROJECT", developmentProjectID)
	t.Setenv("GCP_WIF_AUDIENCE", "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/operator/providers/aws")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "operator@test-youtube-study-space.iam.gserviceaccount.com")

	got, err := GoogleConfigFromEnv("development", developmentProjectID)
	if err != nil {
		t.Fatalf("GoogleConfigFromEnv returned error: %v", err)
	}
	if got.Environment != "development" {
		t.Fatalf("environment = %q", got.Environment)
	}
	if got.ProjectID != developmentProjectID {
		t.Fatalf("project ID = %q", got.ProjectID)
	}
	if got.AWSProfile != developmentAWSProfile {
		t.Fatalf("AWS profile = %q, want %q", got.AWSProfile, developmentAWSProfile)
	}
}

func TestGoogleConfigFromEnvBindsProductionProfile(t *testing.T) {
	clearStaticAWSEnv(t)
	t.Setenv("GOOGLE_CLOUD_PROJECT", productionProjectID)
	t.Setenv("GCP_WIF_AUDIENCE", "audience")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "operator@example.iam.gserviceaccount.com")

	got, err := GoogleConfigFromEnv("production", productionProjectID)
	if err != nil {
		t.Fatalf("GoogleConfigFromEnv returned error: %v", err)
	}
	if got.AWSProfile != productionAWSProfile {
		t.Fatalf("AWS profile = %q, want %q", got.AWSProfile, productionAWSProfile)
	}
}

func TestGoogleConfigFromEnvRejectsEnvironmentProjectMismatch(t *testing.T) {
	_, err := GoogleConfigFromEnv("development", productionProjectID)
	if err == nil || !strings.Contains(err.Error(), "environment/project mismatch") {
		t.Fatalf("expected environment/project mismatch, got %v", err)
	}
}

func TestGoogleConfigFromEnvRejectsConfiguredProjectMismatch(t *testing.T) {
	clearStaticAWSEnv(t)
	t.Setenv("GOOGLE_CLOUD_PROJECT", developmentProjectID)
	t.Setenv("GCP_WIF_AUDIENCE", "audience")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "operator@example.iam.gserviceaccount.com")

	_, err := GoogleConfigFromEnv("production", productionProjectID)
	if err == nil || !strings.Contains(err.Error(), "GCP project mismatch") {
		t.Fatalf("expected project mismatch, got %v", err)
	}
}

func TestGoogleConfigFromEnvRejectsStaticAWSCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_SESSION_TOKEN", "session")
	t.Setenv("GOOGLE_CLOUD_PROJECT", developmentProjectID)
	t.Setenv("GCP_WIF_AUDIENCE", "audience")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "operator@example.iam.gserviceaccount.com")

	_, err := GoogleConfigFromEnv("development", developmentProjectID)
	if err == nil || !strings.Contains(err.Error(), "must be unset") {
		t.Fatalf("expected static AWS credential rejection, got %v", err)
	}
}

func TestGoogleConfigFromEnvRejectsUnknownEnvironmentBeforeAuthConfig(t *testing.T) {
	_, err := GoogleConfigFromEnv("staging", "project-id")
	if err == nil || !strings.Contains(err.Error(), "development or production") {
		t.Fatalf("expected environment validation error, got %v", err)
	}
}
