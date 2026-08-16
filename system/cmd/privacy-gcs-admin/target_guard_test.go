package main

import (
	"strings"
	"testing"
)

func TestBuildGCSPurgeTarget(t *testing.T) {
	t.Parallel()

	target, err := buildGCSPurgeTarget(
		productionEnvironment,
		"youtube-study-space-prod",
		"youtube-study-space-prod",
		"firestore-backup-prod",
		"firestore-backup-prod",
	)
	if err != nil {
		t.Fatalf("buildGCSPurgeTarget() error = %v", err)
	}
	if target.Environment != productionEnvironment || target.ProjectID != "youtube-study-space-prod" || target.BucketName != "firestore-backup-prod" {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestBuildGCSPurgeTargetRejectsProjectMismatch(t *testing.T) {
	t.Parallel()

	_, err := buildGCSPurgeTarget(
		developmentEnvironment,
		"expected-dev",
		"different-project",
		"backup-dev",
		"backup-dev",
	)
	if err == nil || !strings.Contains(err.Error(), "GCP project mismatch") {
		t.Fatalf("error = %v, want project mismatch", err)
	}
}

func TestBuildGCSPurgeTargetRejectsBucketMismatch(t *testing.T) {
	t.Parallel()

	_, err := buildGCSPurgeTarget(
		developmentEnvironment,
		"project-dev",
		"project-dev",
		"expected-bucket",
		"configured-bucket",
	)
	if err == nil || !strings.Contains(err.Error(), "GCS bucket mismatch") {
		t.Fatalf("error = %v, want bucket mismatch", err)
	}
}

func TestBuildGCSPurgeTargetRejectsUnknownEnvironment(t *testing.T) {
	t.Parallel()

	_, err := buildGCSPurgeTarget("staging", "project", "project", "bucket", "bucket")
	if err == nil || !strings.Contains(err.Error(), "environment must be") {
		t.Fatalf("error = %v, want environment validation error", err)
	}
}

func TestValidateGCSApplyTarget(t *testing.T) {
	t.Parallel()

	if err := validateGCSApplyTarget(gcsPurgeTarget{Environment: developmentEnvironment}, false); err != nil {
		t.Fatalf("development apply rejected: %v", err)
	}
	if err := validateGCSApplyTarget(gcsPurgeTarget{Environment: developmentEnvironment}, true); err == nil {
		t.Fatal("development apply accepted --allow-production")
	}
	if err := validateGCSApplyTarget(gcsPurgeTarget{Environment: productionEnvironment}, false); err == nil {
		t.Fatal("production apply accepted without --allow-production")
	}
	if err := validateGCSApplyTarget(gcsPurgeTarget{Environment: productionEnvironment}, true); err != nil {
		t.Fatalf("production apply rejected with --allow-production: %v", err)
	}
}
