package operatorauth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"

	"app.modules/internal/awsruntime"
)

const (
	AuthModeEnv            = "OPERATOR_GCP_AUTH_MODE"
	AuthModeServiceAccount = "service-account"
	AuthModeWIF            = "wif"

	credentialFileLocationEnv = "CREDENTIAL_FILE_LOCATION"
)

type Target struct {
	Environment string
	ProjectID   string
	AWSProfile  string
}

type GoogleCredentials struct {
	ClientOption option.ClientOption
	ProjectID    string
	AuthMode     string
}

var googleClientOptionWithConfig = awsruntime.GoogleClientOptionWithConfig

// ResolveTarget binds the operator-supplied environment to the repository's
// confirmed project/profile pair. A mismatched environment/project combination
// is rejected before any credential is loaded.
func ResolveTarget(environment, expectedProjectID string) (Target, error) {
	environment = strings.TrimSpace(environment)
	expectedProjectID = strings.TrimSpace(expectedProjectID)

	var target Target
	switch environment {
	case "development":
		target = Target{
			Environment: "development",
			ProjectID:   "test-youtube-study-space",
			AWSProfile:  "soraride-google-operator-dev",
		}
	case "production":
		target = Target{
			Environment: "production",
			ProjectID:   "youtube-study-space",
			AWSProfile:  "soraride-google-operator-prod",
		}
	default:
		return Target{}, fmt.Errorf("environment must be development or production: %q", environment)
	}

	if expectedProjectID == "" {
		return Target{}, fmt.Errorf("expected GCP project ID is required")
	}
	if expectedProjectID != target.ProjectID {
		return Target{}, fmt.Errorf(
			"environment/project mismatch: environment=%q requires project=%q, got=%q",
			target.Environment,
			target.ProjectID,
			expectedProjectID,
		)
	}
	return target, nil
}

// NewGoogleCredentials returns credentials for a validated operator target.
// The default service-account mode preserves the current rollback path while
// WIF is rolled out. WIF mode never loads the legacy .env credential file.
func NewGoogleCredentials(ctx context.Context, target Target) (GoogleCredentials, error) {
	mode := strings.TrimSpace(os.Getenv(AuthModeEnv))
	if mode == "" {
		mode = AuthModeServiceAccount
	}

	switch mode {
	case AuthModeServiceAccount:
		return serviceAccountCredentials(ctx, target)
	case AuthModeWIF:
		return wifCredentials(ctx, target)
	default:
		return GoogleCredentials{}, fmt.Errorf(
			"%s must be %q or %q, got %q",
			AuthModeEnv,
			AuthModeServiceAccount,
			AuthModeWIF,
			mode,
		)
	}
}

func serviceAccountCredentials(ctx context.Context, target Target) (GoogleCredentials, error) {
	if err := godotenv.Load(".env"); err != nil {
		return GoogleCredentials{}, fmt.Errorf("load legacy operator .env: %w", err)
	}
	credentialFilePath := strings.TrimSpace(os.Getenv(credentialFileLocationEnv))
	if credentialFilePath == "" {
		return GoogleCredentials{}, fmt.Errorf("%s is required", credentialFileLocationEnv)
	}

	//nolint:staticcheck // Temporary rollback path until operator WIF is validated in both environments.
	clientOption := option.WithCredentialsFile(credentialFilePath)
	actualProjectID, err := projectIDFromCredentials(ctx, clientOption)
	if err != nil {
		return GoogleCredentials{}, fmt.Errorf("resolve GCP project ID from legacy credential: %w", err)
	}
	if actualProjectID != target.ProjectID {
		return GoogleCredentials{}, fmt.Errorf(
			"GCP project mismatch: target=%q credential=%q; refusing to continue",
			target.ProjectID,
			actualProjectID,
		)
	}

	return GoogleCredentials{
		ClientOption: clientOption,
		ProjectID:    actualProjectID,
		AuthMode:     AuthModeServiceAccount,
	}, nil
}

func wifCredentials(ctx context.Context, target Target) (GoogleCredentials, error) {
	wif, err := awsruntime.GoogleWIFConfigFromEnv()
	if err != nil {
		return GoogleCredentials{}, fmt.Errorf("load operator WIF config: %w", err)
	}
	if wif.ProjectID != target.ProjectID {
		return GoogleCredentials{}, fmt.Errorf(
			"GCP project mismatch: target=%q configured=%q; refusing to load AWS credentials",
			target.ProjectID,
			wif.ProjectID,
		)
	}

	clientOption, err := googleClientOptionWithConfig(ctx, wif, target.AWSProfile)
	if err != nil {
		return GoogleCredentials{}, fmt.Errorf("initialize operator WIF credentials with AWS profile %q: %w", target.AWSProfile, err)
	}
	return GoogleCredentials{
		ClientOption: clientOption,
		ProjectID:    target.ProjectID,
		AuthMode:     AuthModeWIF,
	}, nil
}

func projectIDFromCredentials(ctx context.Context, clientOption option.ClientOption) (string, error) {
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return "", fmt.Errorf("load Google credentials: %w", err)
	}
	if strings.TrimSpace(creds.ProjectID) == "" {
		return "", fmt.Errorf("Google credentials did not provide a project ID")
	}
	return creds.ProjectID, nil
}
