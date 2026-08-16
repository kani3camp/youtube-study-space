package main

import (
	"errors"
	"fmt"
	"strings"
)

const (
	developmentEnvironment = "development"
	productionEnvironment  = "production"
)

type gcsPurgeTarget struct {
	Environment string `json:"environment"`
	ProjectID   string `json:"project_id"`
	BucketName  string `json:"bucket_name"`
}

func buildGCSPurgeTarget(
	environment string,
	expectedProjectID string,
	actualProjectID string,
	expectedBucketName string,
	configuredBucketName string,
) (gcsPurgeTarget, error) {
	environment = strings.TrimSpace(environment)
	expectedProjectID = strings.TrimSpace(expectedProjectID)
	actualProjectID = strings.TrimSpace(actualProjectID)
	expectedBucketName = strings.TrimSpace(expectedBucketName)
	configuredBucketName = strings.TrimSpace(configuredBucketName)

	if environment != developmentEnvironment && environment != productionEnvironment {
		return gcsPurgeTarget{}, fmt.Errorf(
			"environment must be %q or %q: %q",
			developmentEnvironment,
			productionEnvironment,
			environment,
		)
	}
	if expectedProjectID == "" {
		return gcsPurgeTarget{}, errors.New("expected GCP project ID is required")
	}
	if actualProjectID == "" {
		return gcsPurgeTarget{}, errors.New("credential GCP project ID is empty")
	}
	if expectedProjectID != actualProjectID {
		return gcsPurgeTarget{}, fmt.Errorf(
			"GCP project mismatch: expected=%q credential=%q; refusing to inspect or mutate",
			expectedProjectID,
			actualProjectID,
		)
	}
	if expectedBucketName == "" {
		return gcsPurgeTarget{}, errors.New("expected GCS bucket name is required")
	}
	if configuredBucketName == "" {
		return gcsPurgeTarget{}, errors.New("configured GCS bucket name is empty")
	}
	if expectedBucketName != configuredBucketName {
		return gcsPurgeTarget{}, fmt.Errorf(
			"GCS bucket mismatch: expected=%q configured=%q; refusing to inspect or mutate",
			expectedBucketName,
			configuredBucketName,
		)
	}

	return gcsPurgeTarget{
		Environment: environment,
		ProjectID:   actualProjectID,
		BucketName:  configuredBucketName,
	}, nil
}

func validateGCSApplyTarget(target gcsPurgeTarget, allowProduction bool) error {
	if target.Environment == productionEnvironment && !allowProduction {
		return errors.New("production apply requires --allow-production")
	}
	if target.Environment == developmentEnvironment && allowProduction {
		return errors.New("--allow-production is only valid with environment=production")
	}
	return nil
}
