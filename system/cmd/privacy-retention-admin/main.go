package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
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
)

type previewOutput struct {
	Mode                 string                               `json:"mode"`
	Cutoff               time.Time                            `json:"cutoff"`
	Audit                mybigquery.RawLiveChatRetentionAudit `json:"audit"`
	DeleteCandidates     int64                                `json:"delete_candidates"`
	RequiredConfirmation string                               `json:"required_confirmation"`
}

type applyOutput struct {
	Mode             string                               `json:"mode"`
	Cutoff           time.Time                            `json:"cutoff"`
	ConfirmedRows    int64                                `json:"confirmed_rows"`
	Before           mybigquery.RawLiveChatRetentionAudit `json:"before"`
	After            mybigquery.RawLiveChatRetentionAudit `json:"after"`
	DeleteCandidates int64                                `json:"delete_candidates"`
}

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-retention-admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 3 {
		return usageError()
	}

	cutoff, err := time.Parse(time.RFC3339, strings.TrimSpace(args[2]))
	if err != nil {
		return fmt.Errorf("parse cutoff as RFC3339: %w", err)
	}
	cutoff = cutoff.UTC()

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
	projectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("resolve GCP project ID: %w", err)
	}
	bqClient, err := mybigquery.NewBigqueryClient(
		ctx,
		projectID,
		clientOption,
		constants.GcpRegion,
	)
	if err != nil {
		return fmt.Errorf("initialize BigQuery: %w", err)
	}
	defer bqClient.CloseClient()

	switch args[1] {
	case previewBigQueryCommand:
		if len(args) != 3 {
			return usageError()
		}
		return previewBigQuery(ctx, bqClient, cutoff)
	case applyBigQueryCommand:
		if len(args) != 7 || args[3] != "--expected-rows" || args[5] != "--confirm" {
			return usageError()
		}
		expectedRows, err := strconv.ParseInt(args[4], 10, 64)
		if err != nil || expectedRows <= 0 {
			return fmt.Errorf("expected rows must be a positive integer: %q", args[4])
		}
		return applyBigQuery(ctx, bqClient, cutoff, expectedRows, args[6])
	default:
		return usageError()
	}
}

func previewBigQuery(
	ctx context.Context,
	client *mybigquery.BigqueryController,
	cutoff time.Time,
) error {
	audit, err := client.InspectRawLiveChatRetention(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("inspect BigQuery raw chat: %w", err)
	}
	candidates := deleteCandidateCount(audit)
	output := previewOutput{
		Mode:                 "preview",
		Cutoff:               cutoff,
		Audit:                audit,
		DeleteCandidates:     candidates,
		RequiredConfirmation: confirmationToken(cutoff, candidates),
	}
	return writeJSON(output)
}

func applyBigQuery(
	ctx context.Context,
	client *mybigquery.BigqueryController,
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

	requiredConfirmation := confirmationToken(cutoff, expectedRows)
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

func confirmationToken(cutoff time.Time, rows int64) string {
	return fmt.Sprintf(
		"DELETE %d RAW YOUTUBE LIVE CHAT ROWS BEFORE %s",
		rows,
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
		"usage: privacy-retention-admin preview-bigquery <cutoff-rfc3339> OR " +
			"privacy-retention-admin apply-bigquery <cutoff-rfc3339> " +
			"--expected-rows <count> --confirm <exact-preview-token>",
	)
}
