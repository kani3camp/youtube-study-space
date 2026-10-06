package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/option"

	"app.modules/core/mybigquery"
	"app.modules/core/utils"
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

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "user-activity-schema-audit:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) != 4 {
		return usageError()
	}

	environment := strings.TrimSpace(args[1])
	expectedProjectID := strings.TrimSpace(args[2])
	workingRegion := strings.TrimSpace(args[3])
	if workingRegion == "" {
		return errors.New("BigQuery location is required")
	}

	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Operator-controlled credential file for this read-only audit.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	actualProjectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("resolve GCP project ID: %w", err)
	}
	target, err := buildAuditTarget(environment, expectedProjectID, actualProjectID)
	if err != nil {
		return err
	}

	bqClient, err := mybigquery.NewBigqueryClient(ctx, actualProjectID, clientOption, workingRegion)
	if err != nil {
		return fmt.Errorf("initialize BigQuery: %w", err)
	}
	defer bqClient.CloseClient()

	audit, err := bqClient.InspectUserActivityLegacyTimestamp(ctx)
	if err != nil {
		return fmt.Errorf("inspect user-activity schema: %w", err)
	}

	output := auditOutput{
		Mode:   "read-only-aggregate",
		Target: target,
		Audit:  audit,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encode audit output: %w", err)
	}
	return nil
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
	if actualProjectID == "" {
		return auditTarget{}, errors.New("credential GCP project ID is empty")
	}
	if expectedProjectID != actualProjectID {
		return auditTarget{}, fmt.Errorf(
			"GCP project mismatch: expected=%q credential=%q; refusing read-only audit",
			expectedProjectID,
			actualProjectID,
		)
	}

	return auditTarget{
		Environment: environment,
		ProjectID:   actualProjectID,
		Dataset:     mybigquery.DatasetName,
		Table:       mybigquery.UserActivityHistoryMainTableName,
	}, nil
}

func usageError() error {
	return errors.New("usage: user-activity-schema-audit <development|production> <expected-project-id> <bigquery-location>")
}
