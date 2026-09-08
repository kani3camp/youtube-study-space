package operatorauth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/option"

	"app.modules/internal/awsruntime"
)

const (
	AWSProfileEnv         = "AWS_PROFILE"
	awsAccessKeyIDEnv     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKeyEnv = "AWS_SECRET_ACCESS_KEY"
	awsSessionTokenEnv    = "AWS_SESSION_TOKEN"

	developmentProjectID = "test-youtube-study-space"
	productionProjectID  = "youtube-study-space"
)

type GoogleConfig struct {
	Environment string
	ProjectID   string
	AWSProfile  string
	WIF         awsruntime.GoogleWIFConfig
}

func GoogleConfigFromEnv(environment, expectedProjectID string) (GoogleConfig, error) {
	environment = strings.TrimSpace(environment)
	environmentProjectID, err := projectIDForEnvironment(environment)
	if err != nil {
		return GoogleConfig{}, err
	}

	expectedProjectID = strings.TrimSpace(expectedProjectID)
	if expectedProjectID == "" {
		return GoogleConfig{}, errors.New("expected GCP project ID is required")
	}
	if expectedProjectID != environmentProjectID {
		return GoogleConfig{}, fmt.Errorf(
			"environment/project mismatch: environment=%q requires project=%q, got=%q",
			environment,
			environmentProjectID,
			expectedProjectID,
		)
	}

	profile := strings.TrimSpace(os.Getenv(AWSProfileEnv))
	if profile == "" {
		return GoogleConfig{}, fmt.Errorf("%s is required for local operator authentication", AWSProfileEnv)
	}

	if staticAWSCredentialsPresent() {
		return GoogleConfig{}, fmt.Errorf(
			"%s/%s/%s must be unset for local operator authentication; refusing to let environment credentials override the selected SSO profile",
			awsAccessKeyIDEnv,
			awsSecretAccessKeyEnv,
			awsSessionTokenEnv,
		)
	}

	wif, err := awsruntime.GoogleWIFConfigFromEnv()
	if err != nil {
		return GoogleConfig{}, fmt.Errorf("load Google WIF configuration: %w", err)
	}
	if wif.ProjectID != expectedProjectID {
		return GoogleConfig{}, fmt.Errorf(
			"GCP project mismatch: expected=%q configured=%q; refusing to authenticate",
			expectedProjectID,
			wif.ProjectID,
		)
	}

	return GoogleConfig{
		Environment: environment,
		ProjectID:   expectedProjectID,
		AWSProfile:  profile,
		WIF:         wif,
	}, nil
}

func (c GoogleConfig) ClientOption(ctx context.Context) (option.ClientOption, error) {
	clientOption, err := awsruntime.GoogleClientOptionWithConfig(ctx, c.WIF, c.AWSProfile)
	if err != nil {
		return nil, fmt.Errorf("create Google client option for AWS profile %q: %w", c.AWSProfile, err)
	}
	return clientOption, nil
}

func projectIDForEnvironment(environment string) (string, error) {
	switch environment {
	case "development":
		return developmentProjectID, nil
	case "production":
		return productionProjectID, nil
	default:
		return "", fmt.Errorf("environment must be development or production: %q", environment)
	}
}

func staticAWSCredentialsPresent() bool {
	return strings.TrimSpace(os.Getenv(awsAccessKeyIDEnv)) != "" ||
		strings.TrimSpace(os.Getenv(awsSecretAccessKeyEnv)) != "" ||
		strings.TrimSpace(os.Getenv(awsSessionTokenEnv)) != ""
}
