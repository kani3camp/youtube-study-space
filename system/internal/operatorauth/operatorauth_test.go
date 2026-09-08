package operatorauth

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/api/option"

	"app.modules/internal/awsruntime"
)

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		projectID   string
		wantProfile string
	}{
		{name: "development", environment: "development", projectID: "test-youtube-study-space", wantProfile: "soraride-google-operator-dev"},
		{name: "production", environment: "production", projectID: "youtube-study-space", wantProfile: "soraride-google-operator-prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := ResolveTarget(tt.environment, tt.projectID)
			if err != nil {
				t.Fatalf("ResolveTarget returned error: %v", err)
			}
			if target.AWSProfile != tt.wantProfile {
				t.Fatalf("AWS profile = %q, want %q", target.AWSProfile, tt.wantProfile)
			}
		})
	}
}

func TestResolveTargetRejectsUnknownEnvironment(t *testing.T) {
	_, err := ResolveTarget("staging", "test-youtube-study-space")
	if err == nil || !strings.Contains(err.Error(), "development or production") {
		t.Fatalf("expected environment validation error, got %v", err)
	}
}

func TestResolveTargetRejectsEnvironmentProjectMismatch(t *testing.T) {
	_, err := ResolveTarget("development", "youtube-study-space")
	if err == nil || !strings.Contains(err.Error(), "environment/project mismatch") {
		t.Fatalf("expected environment/project mismatch error, got %v", err)
	}
}

func TestWIFRejectsConfiguredProjectMismatchBeforeAWSAccess(t *testing.T) {
	t.Setenv(AuthModeEnv, AuthModeWIF)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "youtube-study-space")
	t.Setenv("GCP_WIF_AUDIENCE", "audience")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "runtime@example.iam.gserviceaccount.com")

	target, err := ResolveTarget("development", "test-youtube-study-space")
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}

	originalGoogleClientOptionWithConfig := googleClientOptionWithConfig
	called := false
	googleClientOptionWithConfig = func(context.Context, awsruntime.GoogleWIFConfig, string) (option.ClientOption, error) {
		called = true
		return option.WithoutAuthentication(), nil
	}
	t.Cleanup(func() {
		googleClientOptionWithConfig = originalGoogleClientOptionWithConfig
	})

	_, err = NewGoogleCredentials(context.Background(), target)
	if err == nil || !strings.Contains(err.Error(), "refusing to load AWS credentials") {
		t.Fatalf("expected configured project mismatch error, got %v", err)
	}
	if called {
		t.Fatal("AWS-backed Google credential factory was called before target validation")
	}
}

func TestWIFUsesTargetAWSProfile(t *testing.T) {
	t.Setenv(AuthModeEnv, AuthModeWIF)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-youtube-study-space")
	t.Setenv("GCP_WIF_AUDIENCE", "audience")
	t.Setenv("GCP_WIF_SERVICE_ACCOUNT_EMAIL", "runtime@example.iam.gserviceaccount.com")

	target, err := ResolveTarget("development", "test-youtube-study-space")
	if err != nil {
		t.Fatalf("ResolveTarget returned error: %v", err)
	}

	originalGoogleClientOptionWithConfig := googleClientOptionWithConfig
	var gotProfile string
	googleClientOptionWithConfig = func(_ context.Context, _ awsruntime.GoogleWIFConfig, profile string) (option.ClientOption, error) {
		gotProfile = profile
		return option.WithoutAuthentication(), nil
	}
	t.Cleanup(func() {
		googleClientOptionWithConfig = originalGoogleClientOptionWithConfig
	})

	credentials, err := NewGoogleCredentials(context.Background(), target)
	if err != nil {
		t.Fatalf("NewGoogleCredentials returned error: %v", err)
	}
	if gotProfile != "soraride-google-operator-dev" {
		t.Fatalf("AWS profile = %q, want soraride-google-operator-dev", gotProfile)
	}
	if credentials.ProjectID != "test-youtube-study-space" || credentials.AuthMode != AuthModeWIF {
		t.Fatalf("unexpected credentials metadata: %#v", credentials)
	}
}
