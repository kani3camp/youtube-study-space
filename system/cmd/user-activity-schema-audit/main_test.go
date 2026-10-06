package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"app.modules/core/mybigquery"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

func syntheticWIFFile(t *testing.T, project string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic-wif.json")
	fixture := map[string]any{
		"type":                              "external_account",
		"audience":                          "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/synthetic-pool/providers/synthetic-provider",
		"subject_token_type":                "urn:ietf:params:oauth:token-type:jwt",
		"token_url":                         "https://sts.googleapis.com/v1/token",
		"credential_source":                 map[string]any{"file": filepath.Join(t.TempDir(), "absent-synthetic-token")},
		"service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/synthetic-audit@test-youtube-study-space.iam.gserviceaccount.com:generateAccessToken",
	}
	if project != "" {
		fixture["project_id"] = project
	}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProjectlessWIFFixture(t *testing.T) {
	t.Setenv("USER_ACTIVITY_SCHEMA_AUDIT_MAX_BYTES_BILLED", "1073741824")
	path := syntheticWIFFile(t, "")
	// auth's project_id input is exported to environment variables, not embedded
	// in the external_account JSON. The legacy SDK parser does not use this env.
	t.Setenv("GOOGLE_CLOUD_PROJECT", "test-youtube-study-space")
	//nolint:staticcheck // Synthetic fixture reproduces the CLI's legacy credential option.
	credentials, err := transport.Creds(context.Background(), option.WithCredentialsFile(path))
	if err != nil {
		t.Fatalf("read synthetic WIF metadata: %v", err)
	}
	if credentials.ProjectID != "" {
		t.Fatalf("WIF credential ProjectID = %q, want empty", credentials.ProjectID)
	}
	if _, err := buildAuditTarget("development", "test-youtube-study-space", credentials.ProjectID); err != nil {
		t.Fatalf("explicit development target must support projectless WIF: %v", err)
	}

	// A trusted workflow has no .env. Ambient project variables must not choose
	// either the job project or the retained table's resource project.
	t.Chdir(t.TempDir())
	t.Setenv("CREDENTIAL_FILE_LOCATION", path)
	t.Setenv("GOOGLE_CLOUD_PROJECT", "youtube-study-space")
	config, err := prepareAudit(context.Background(), []string{
		"user-activity-schema-audit", "development", "test-youtube-study-space", "asia-southeast2",
	})
	if err != nil {
		t.Fatalf("prepare audit without .env or a token file: %v", err)
	}
	if config.Target.ProjectID != "test-youtube-study-space" || config.WorkingRegion != "asia-southeast2" || config.MaxBytesBilled != mybigquery.MaxUserActivityAuditBytes {
		t.Fatalf("prepared wrong explicit target: %#v", config.Target)
	}
	client, err := mybigquery.NewBigqueryClient(context.Background(), config.Target.ProjectID, config.CredentialOption, config.WorkingRegion)
	if err != nil {
		t.Fatalf("initialize explicit BigQuery client without token exchange: %v", err)
	}
	defer client.CloseClient()
	if client.Client.Project() != config.Target.ProjectID {
		t.Fatalf("BigQuery project = %q, want explicit target", client.Client.Project())
	}
}

func TestPrepareAuditRejectsPresentCredentialProjectMismatch(t *testing.T) {
	t.Setenv("USER_ACTIVITY_SCHEMA_AUDIT_MAX_BYTES_BILLED", "1073741824")
	t.Setenv("CREDENTIAL_FILE_LOCATION", syntheticWIFFile(t, "youtube-study-space"))
	_, err := prepareAudit(context.Background(), []string{
		"user-activity-schema-audit", "development", "test-youtube-study-space", "asia-southeast2",
	})
	if err == nil || !strings.Contains(err.Error(), "GCP project mismatch") {
		t.Fatalf("prepareAudit() error = %v, want credential mismatch", err)
	}
}

func TestBuildAuditTarget(t *testing.T) {
	t.Parallel()

	target, err := buildAuditTarget("development", "test-youtube-study-space", "test-youtube-study-space")
	if err != nil {
		t.Fatalf("buildAuditTarget() error = %v", err)
	}
	if target.Environment != "development" || target.ProjectID != "test-youtube-study-space" ||
		target.Dataset != mybigquery.DatasetName || target.Table != mybigquery.UserActivityHistoryMainTableName {
		t.Fatalf("buildAuditTarget() = %#v", target)
	}

	for _, tc := range []struct {
		name        string
		environment string
		expected    string
		actual      string
		want        string
	}{
		{name: "unknown environment", environment: "staging", expected: "p", actual: "p", want: "environment must be"},
		{name: "missing expected project", environment: "development", expected: "", actual: "p", want: "expected GCP project ID is required"},
		{name: "credential project mismatch", environment: "production", expected: "youtube-study-space", actual: "test-youtube-study-space", want: "GCP project mismatch"},
		{name: "cross environment explicit target", environment: "development", expected: "youtube-study-space", actual: "", want: "does not match"},
		{name: "arbitrary explicit target", environment: "development", expected: "synthetic-project", actual: "", want: "does not match"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := buildAuditTarget(tc.environment, tc.expected, tc.actual)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("buildAuditTarget() error = %v, want substring %q", err, tc.want)
			}
		})
	}
	for _, environment := range []string{"development", "production"} {
		project := "test-youtube-study-space"
		if environment == "production" {
			project = "youtube-study-space"
		}
		if target, err := buildAuditTarget(environment, project, ""); err != nil || target.ProjectID != project {
			t.Fatalf("projectless explicit %s target = %#v, error = %v", environment, target, err)
		}
	}
}

func TestPrepareAuditRejectsInvalidArgumentsBeforeCredentials(t *testing.T) {
	t.Setenv("USER_ACTIVITY_SCHEMA_AUDIT_MAX_BYTES_BILLED", "1073741824")
	t.Setenv("CREDENTIAL_FILE_LOCATION", "")
	for _, args := range [][]string{
		{"audit"},
		{"audit", "development", "test-youtube-study-space", ""},
		{"audit", "development", "youtube-study-space", "asia-southeast2"},
		{"audit", "unknown", "test-youtube-study-space", "asia-southeast2"},
	} {
		if _, err := prepareAudit(context.Background(), args); err == nil || strings.Contains(err.Error(), "CREDENTIAL_FILE_LOCATION") {
			t.Fatalf("invalid arguments must fail before credentials: %v, error = %v", args, err)
		}
	}
}

func TestUsageErrorMentionsBigQueryLocation(t *testing.T) {
	t.Parallel()

	err := usageError()
	if err == nil || !strings.Contains(err.Error(), "<bigquery-location>") {
		t.Fatalf("usageError() = %v, want bigquery location argument", err)
	}
}

func TestPrepareAuditRejectsUnboundedBudgetBeforeCredentials(t *testing.T) {
	t.Setenv("CREDENTIAL_FILE_LOCATION", "")
	for _, value := range []string{"", "0", "-1", "+1", "1.5", "1073741825", "9223372036854775808"} {
		t.Setenv("USER_ACTIVITY_SCHEMA_AUDIT_MAX_BYTES_BILLED", value)
		_, err := prepareAudit(context.Background(), []string{"audit", "development", "test-youtube-study-space", "asia-southeast2"})
		if err == nil || strings.Contains(err.Error(), "CREDENTIAL_FILE_LOCATION") {
			t.Fatalf("budget %q must fail before credential loading: %v", value, err)
		}
	}
	t.Setenv("USER_ACTIVITY_SCHEMA_AUDIT_MAX_BYTES_BILLED", "")
	_, err := prepareAudit(context.Background(), []string{"audit", "development", "test-youtube-study-space", "asia-southeast2"})
	if err == nil || strings.Contains(err.Error(), "CREDENTIAL_FILE_LOCATION") {
		t.Fatalf("missing budget must fail: %v", err)
	}
}
