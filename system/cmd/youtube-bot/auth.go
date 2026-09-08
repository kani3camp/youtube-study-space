package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"app.modules/core/utils"
	"app.modules/internal/awsruntime"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	youtubeBotAuthModeEnv     = "YOUTUBE_BOT_AUTH_MODE"
	youtubeBotEnvironmentEnv  = "YOUTUBE_BOT_ENVIRONMENT"
	youtubeBotAuthModeLegacy  = "service-account"
	youtubeBotAuthModeWIF     = "wif"
	googleCloudProjectEnvName = "GOOGLE_CLOUD_PROJECT"

	youtubeBotDevelopmentProjectID = "test-youtube-study-space"
	youtubeBotProductionProjectID  = "youtube-study-space"
)

func initGoogleClient(ctx context.Context) (option.ClientOption, bool, error) {
	mode := strings.TrimSpace(os.Getenv(youtubeBotAuthModeEnv))
	switch mode {
	case "", youtubeBotAuthModeLegacy:
		clientOption, err := initLegacyGoogleClient(ctx)
		if err != nil {
			return nil, false, err
		}
		return clientOption, true, nil
	case youtubeBotAuthModeWIF:
		clientOption, err := initWIFGoogleClient(ctx)
		if err != nil {
			return nil, false, err
		}
		return clientOption, false, nil
	default:
		return nil, false, fmt.Errorf(
			"%s must be %q or %q, got %q",
			youtubeBotAuthModeEnv,
			youtubeBotAuthModeLegacy,
			youtubeBotAuthModeWIF,
			mode,
		)
	}
}

func initLegacyGoogleClient(ctx context.Context) (option.ClientOption, error) {
	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return nil, errors.New("CREDENTIAL_FILE_LOCATION is required")
	}

	//nolint:staticcheck // Temporary streaming-PC path until the Fargate WIF cutover is validated.
	clientOption := option.WithCredentialsFile(credentialFilePath)
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return nil, fmt.Errorf("load Google credentials: %w", err)
	}

	fmt.Printf("Project ID: %s\n", creds.ProjectID)
	fmt.Println("Is this the correct project ID? (yes/no)")
	var confirmation string
	if _, err := fmt.Scanln(&confirmation); err != nil {
		return nil, fmt.Errorf("failed to read project confirmation: %w", err)
	}
	if confirmation != "yes" {
		return nil, errors.New("aborted")
	}
	return clientOption, nil
}

func initWIFGoogleClient(ctx context.Context) (option.ClientOption, error) {
	environment := strings.TrimSpace(os.Getenv(youtubeBotEnvironmentEnv))
	projectID := strings.TrimSpace(os.Getenv(googleCloudProjectEnvName))
	if err := validateYoutubeBotTarget(environment, projectID); err != nil {
		return nil, err
	}

	clientOption, err := awsruntime.GoogleClientOption(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize youtube-bot Google WIF credentials: %w", err)
	}
	return clientOption, nil
}

func validateYoutubeBotTarget(environment, projectID string) error {
	var expectedProjectID string
	switch environment {
	case "development":
		expectedProjectID = youtubeBotDevelopmentProjectID
	case "production":
		expectedProjectID = youtubeBotProductionProjectID
	default:
		return fmt.Errorf(
			"%s must be development or production, got %q",
			youtubeBotEnvironmentEnv,
			environment,
		)
	}
	if projectID == "" {
		return fmt.Errorf("%s is required", googleCloudProjectEnvName)
	}
	if projectID != expectedProjectID {
		return fmt.Errorf(
			"youtube-bot environment/project mismatch: environment=%q requires project=%q, configured=%q",
			environment,
			expectedProjectID,
			projectID,
		)
	}
	return nil
}
