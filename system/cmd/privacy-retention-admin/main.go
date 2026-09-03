package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/api/option"

	"app.modules/core/mybigquery"
	"app.modules/core/repository"
	"app.modules/core/utils"
)

const (
	previewBigQueryCommand = "preview-bigquery"
	applyBigQueryCommand   = "apply-bigquery"

	developmentEnvironment = "development"
	productionEnvironment  = "production"
)

type purgeTarget struct {
	Environment    string `json:"environment"`
	ProjectID      string `json:"project_id"`
	Dataset        string `json:"dataset"`
	Table          string `json:"table"`
	QualifiedTable string `json:"qualified_table"`
}

type previewOutput struct {
	Mode                 string                               `json:"mode"`
	Target               purgeTarget                          `json:"target"`
	Cutoff               time.Time                            `json:"cutoff"`
	Audit                mybigquery.RawLiveChatRetentionAudit `json:"audit"`
	DeleteCandidates     int64                                `json:"delete_candidates"`
	RequiredConfirmation string                               `json:"required_confirmation"`
}

type applyOutput struct {
	Mode             string                               `json:"mode"`
	Target           purgeTarget                          `json:"target"`
	Cutoff           time.Time                            `json:"cutoff"`
	ConfirmedRows    int64                                `json:"confirmed_rows"`
	Before           mybigquery.RawLiveChatRetentionAudit `json:"before"`
	After            mybigquery.RawLiveChatRetentionAudit `json:"after"`
	DeleteCandidates int64                                `json:"delete_candidates"`
}

type applyArgs struct {
	ExpectedRows    int64
	Confirmation    string
	AllowProduction bool
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-retention-admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 5 {
		return usageError()
	}

	command := args[1]
	environment := strings.TrimSpace(args[2])
	expectedProjectID := strings.TrimSpace(args[3])
	cutoff, err := time.Parse(time.RFC3339, strings.TrimSpace(args[4]))
	if err != nil {
		return fmt.Errorf("parse cutoff as RFC3339: %w", err)
	}
	cutoff = cutoff.UTC()

	if command == previewBigQueryCommand && len(args) != 5 {
		return usageError()
	}

	var parsedApplyArgs applyArgs
	if command == applyBigQueryCommand {
		parsedApplyArgs, err = parseApplyArgs(args[5:])
		if err != nil {
			return err
		}
	} else if command != previewBigQueryCommand {
		return usageError()
	}

	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Credential file path is operator-controlled for this explicit admin operation.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "privacy-retention-admin: close Firestore:", err)
		}
	}()

	constants, err := repo.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return fmt.Errorf("read system constants: %w", err)
	}
	actualProjectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("resolve GCP project ID: %w", err)
	}
	target, err := buildPurgeTarget(environment, expectedProjectID, actualProjectID)
	if err != nil {
		return err
	}

	bqClient, err := mybigquery.NewBigqueryClient(
		ctx,
		actualProjectID,
		clientOption,
		constants.GcpRegion,
	)
	if err != nil {
		return fmt.Errorf("initialize BigQuery: %w", err)
	}
	defer bqClient.CloseClient()

	switch command {
	case previewBigQueryCommand:
		return previewBigQuery(ctx, bqClient, target, cutoff)
	case applyBigQueryCommand:
		if err := validateApplyTarget(target, parsedApplyArgs.AllowProduction); err != nil {
			return err
		}
		return applyBigQuery(
			ctx,
			bqClient,
			target,
			cutoff,
			parsedApplyArgs.ExpectedRows,
			parsedApplyArgs.Confirmation,
		)
	default:
		return usageError()
	}
}

func parseApplyArgs(args []string) (applyArgs, error) {
	flags := flag.NewFlagSet(applyBigQueryCommand, flag.ContinueOnError)
	expectedRows := flags.Int64("expected-rows", 0, "candidate row count from preview")
	confirmation := flags.String("confirm", "", "exact confirmation token from preview")
	allowProduction := flags.Bool("allow-production", false, "explicitly allow a production apply")
	if err := flags.Parse(args); err != nil {
		return applyArgs{}, usageError()
	}
	if flags.NArg() != 0 || *expectedRows <= 0 || strings.TrimSpace(*confirmation) == "" {
		return applyArgs{}, usageError()
	}
	return applyArgs{
		ExpectedRows:    *expectedRows,
		Confirmation:    *confirmation,
		AllowProduction: *allowProduction,
	}, nil
}

func buildPurgeTarget(environment, expectedProjectID, actualProjectID string) (purgeTarget, error) {
	environment = strings.TrimSpace(environment)
	expectedProjectID = strings.TrimSpace(expectedProjectID)
	actualProjectID = strings.TrimSpace(actualProjectID)

	if environment != developmentEnvironment && environment != productionEnvironment {
		return purgeTarget{}, fmt.Errorf(
			"environment must be %q or %q: %q",
			developmentEnvironment,
			productionEnvironment,
			environment,
		)
	}
	if expectedProjectID == "" {
		return purgeTarget{}, errors.New("expected GCP project ID is required")
	}
	if actualProjectID == "" {
		return purgeTarget{}, errors.New("credential GCP project ID is empty")
	}
	if expectedProjectID != actualProjectID {
		return purgeTarget{}, fmt.Errorf(
			"GCP project mismatch: expected=%q credential=%q; refusing to inspect or mutate",
			expectedProjectID,
			actualProjectID,
		)
	}

	qualifiedTable := strings.Join([]string{
		actualProjectID,
		mybigquery.DatasetName,
		mybigquery.LiveChatHistoryMainTableName,
	}, ".")
	return purgeTarget{
		Environment:    environment,
		ProjectID:      actualProjectID,
		Dataset:        mybigquery.DatasetName,
		Table:          mybigquery.LiveChatHistoryMainTableName,
		QualifiedTable: qualifiedTable,
	}, nil
}

func validateApplyTarget(target purgeTarget, allowProduction bool) error {
	if target.Environment == productionEnvironment && !allowProduction {
		return errors.New("production apply requires --allow-production")
	}
	if target.Environment == developmentEnvironment && allowProduction {
		return errors.New("--allow-production is only valid with environment=production")
	}
	return nil
}

func previewBigQuery(
	ctx context.Context,
	client *mybigquery.BigqueryController,
	target purgeTarget,
	cutoff time.Time,
) error {
	audit, err := client.InspectRawLiveChatRetention(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("inspect BigQuery raw chat: %w", err)
	}
	candidates := deleteCandidateCount(audit)
	output := previewOutput{
		Mode:                 "preview",
		Target:               target,
		Cutoff:               cutoff,
		Audit:                audit,
		DeleteCandidates:     candidates,
		RequiredConfirmation: confirmationToken(target, cutoff, candidates),
	}
	return writeJSON(output)
}

func applyBigQuery(
	ctx context.Context,
	client *mybigquery.BigqueryController,
	target purgeTarget,
	cutoff time.Time,
	expectedRows int64,
	confirmation string,
) error {
	before, err := client.InspectRawLiveChatRetention(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("inspect BigQuery raw chat before purge: %w", err)
	}
	candidates := deleteCandidateCount(before)
	if candidates == 0 {
		return errors.New("there are no raw live chat rows to purge for this cutoff")
	}
	if candidates != expectedRows {
		return fmt.Errorf(
			"candidate count changed: expected=%d current=%d; rerun preview",
			expectedRows,
			candidates,
		)
	}

	requiredConfirmation := confirmationToken(target, cutoff, expectedRows)
	if confirmation != requiredConfirmation {
		return fmt.Errorf("confirmation mismatch; required exactly %q", requiredConfirmation)
	}

	if err := client.PurgeRawLiveChatBefore(ctx, cutoff, expectedRows); err != nil {
		return fmt.Errorf("purge BigQuery raw chat: %w", err)
	}

	after, err := client.InspectRawLiveChatRetention(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("inspect BigQuery raw chat after purge: %w", err)
	}
	if remaining := deleteCandidateCount(after); remaining != 0 {
		return fmt.Errorf("post-purge verification failed: %d candidate rows remain", remaining)
	}

	output := applyOutput{
		Mode:             "applied",
		Target:           target,
		Cutoff:           cutoff,
		ConfirmedRows:    expectedRows,
		Before:           before,
		After:            after,
		DeleteCandidates: candidates,
	}
	return writeJSON(output)
}

func deleteCandidateCount(audit mybigquery.RawLiveChatRetentionAudit) int64 {
	return audit.RowsOlderThanCutoff + audit.UndatedRows
}

func confirmationToken(target purgeTarget, cutoff time.Time, rows int64) string {
	return fmt.Sprintf(
		"DELETE %d RAW YOUTUBE LIVE CHAT ROWS FROM %s PROJECT %s TABLE %s BEFORE %s",
		rows,
		strings.ToUpper(target.Environment),
		target.ProjectID,
		target.QualifiedTable,
		cutoff.UTC().Format(time.RFC3339),
	)
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}

func usageError() error {
	return errors.New(
		"usage: privacy-retention-admin preview-bigquery <development|production> <expected-project-id> <cutoff-rfc3339> OR " +
			"privacy-retention-admin apply-bigquery <development|production> <expected-project-id> <cutoff-rfc3339> " +
			"--expected-rows <count> --confirm <exact-preview-token> [--allow-production]",
	)
}
