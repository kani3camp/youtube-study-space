package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"

	"app.modules/core/mybigquery"
)

const (
	developmentEnvironment = "development"
	productionEnvironment  = "production"
)

type auditTarget struct {
	Environment string `json:"environment"`
	ProjectID   string `json:"project_id"`
	Dataset     string `json:"dataset"`
	Table       string `json:"table"`
}

type auditOutput struct {
	Mode   string                             `json:"mode"`
	Target auditTarget                        `json:"target"`
	Audit  mybigquery.UserActivitySchemaAudit `json:"audit"`
}

type auditConfig struct {
	Target           auditTarget
	WorkingRegion    string
	MaxBytesBilled   int64
	CredentialOption option.ClientOption
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "user-activity-schema-audit:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	config, err := prepareAudit(ctx, args)
	if err != nil {
		return err
	}
	bqClient, err := mybigquery.NewBigqueryClient(ctx, config.Target.ProjectID, config.CredentialOption, config.WorkingRegion)
	if err != nil {
		return fmt.Errorf("initialize BigQuery: %w", err)
	}
	defer bqClient.CloseClient()

	audit, err := bqClient.InspectUserActivityLegacyTimestamp(ctx, config.MaxBytesBilled)
	if err != nil {
		return fmt.Errorf("inspect user-activity schema: %w", err)
	}

	output := auditOutput{
		Mode:   "read-only-aggregate",
		Target: config.Target,
		Audit:  audit,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encode audit output: %w", err)
	}
	return nil
}

func prepareAudit(ctx context.Context, args []string) (auditConfig, error) {
	if len(args) != 5 {
		return auditConfig{}, usageError()
	}

	environment := strings.TrimSpace(args[1])
	expectedProjectID := strings.TrimSpace(args[2])
	workingRegion := strings.TrimSpace(args[3])
	if workingRegion == "" {
		return auditConfig{}, errors.New("BigQuery location is required")
	}
	budget := strings.TrimSpace(args[4])
	if budget == "" || strings.IndexFunc(budget, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return auditConfig{}, errors.New("audit maximum bytes billed must be an explicit positive decimal integer")
	}
	maxBytesBilled, err := strconv.ParseInt(budget, 10, 64)
	if err != nil {
		return auditConfig{}, errors.New("invalid audit maximum bytes billed")
	}
	if err := mybigquery.ValidateUserActivityAuditBudget(maxBytesBilled); err != nil {
		return auditConfig{}, fmt.Errorf("validate audit query budget: %w", err)
	}
	// Validate the explicit resource target before loading a credential. WIF
	// credentials identify a principal and need not contain a resource project.
	if _, err := buildAuditTarget(environment, expectedProjectID, ""); err != nil {
		return auditConfig{}, err
	}

	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return auditConfig{}, errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Operator-controlled credential file for this read-only audit.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	credentials, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return auditConfig{}, fmt.Errorf("load audit credential metadata: %w", err)
	}
	target, err := buildAuditTarget(environment, expectedProjectID, credentials.ProjectID)
	if err != nil {
		return auditConfig{}, err
	}
	return auditConfig{Target: target, WorkingRegion: workingRegion, MaxBytesBilled: maxBytesBilled, CredentialOption: clientOption}, nil
}

func buildAuditTarget(environment, expectedProjectID, actualProjectID string) (auditTarget, error) {
	environment = strings.TrimSpace(environment)
	expectedProjectID = strings.TrimSpace(expectedProjectID)
	actualProjectID = strings.TrimSpace(actualProjectID)

	if environment != developmentEnvironment && environment != productionEnvironment {
		return auditTarget{}, fmt.Errorf(
			"environment must be %q or %q: %q",
			developmentEnvironment,
			productionEnvironment,
			environment,
		)
	}
	if expectedProjectID == "" {
		return auditTarget{}, errors.New("expected GCP project ID is required")
	}
	canonicalProjectID := "test-youtube-study-space"
	if environment == productionEnvironment {
		canonicalProjectID = "youtube-study-space"
	}
	if expectedProjectID != canonicalProjectID {
		return auditTarget{}, errors.New("GCP project does not match the selected audit environment")
	}
	if actualProjectID != "" && expectedProjectID != actualProjectID {
		return auditTarget{}, fmt.Errorf(
			"GCP project mismatch: expected=%q credential=%q; refusing read-only audit",
			expectedProjectID,
			actualProjectID,
		)
	}

	return auditTarget{
		Environment: environment,
		ProjectID:   expectedProjectID,
		Dataset:     mybigquery.DatasetName,
		Table:       mybigquery.UserActivityHistoryMainTableName,
	}, nil
}

func usageError() error {
	return errors.New("usage: user-activity-schema-audit <development|production> <expected-project-id> <bigquery-location> <maximum-bytes-billed>")
}
