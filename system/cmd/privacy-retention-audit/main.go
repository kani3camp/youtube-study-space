package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"app.modules/core/mybigquery"
	"app.modules/core/repository"
	"app.modules/core/utils"
)

const (
	rawYouTubeDataRetentionDays = 30
	firestoreCountAlias         = "row_count"
)

type report struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Cutoff      time.Time                 `json:"cutoff"`
	Config      configAudit               `json:"config"`
	Firestore   firestoreAudit            `json:"firestore"`
	BigQuery    mybigquery.RawLiveChatRetentionAudit `json:"bigquery"`
	GCS         gcsAudit                  `json:"gcs"`
	Warnings    []string                  `json:"warnings,omitempty"`
}

type configAudit struct {
	ConfiguredHistoryRetentionDays int    `json:"configured_history_retention_days"`
	GCSExportBucketName            string `json:"gcs_export_bucket_name"`
	GCPRegion                      string `json:"gcp_region"`
}

type firestoreAudit struct {
	RawLiveChatRowsOlderThanCutoff int64 `json:"raw_live_chat_rows_older_than_cutoff"`
}

type gcsAudit struct {
	BucketName               string                  `json:"bucket_name"`
	Location                 string                  `json:"location,omitempty"`
	VersioningEnabled        bool                    `json:"versioning_enabled"`
	LifecycleRules           []storage.LifecycleRule `json:"lifecycle_rules,omitempty"`
	RetentionPeriod          string                  `json:"retention_period,omitempty"`
	RetentionPolicyLocked    bool                    `json:"retention_policy_locked,omitempty"`
	SoftDeleteRetention      string                  `json:"soft_delete_retention,omitempty"`
	AttributesInspectionError string                 `json:"attributes_inspection_error,omitempty"`
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "privacy-retention-audit:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Credential file path is operator-controlled for this read-only admin audit.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "privacy-retention-audit: close Firestore:", err)
		}
	}()

	constants, err := repo.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return fmt.Errorf("read system constants: %w", err)
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -rawYouTubeDataRetentionDays)

	firestoreOlderRows, err := countOldFirestoreRawChat(ctx, repo.FirestoreClient(), cutoff)
	if err != nil {
		return fmt.Errorf("inspect Firestore raw chat age: %w", err)
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

	bqAudit, err := bqClient.InspectRawLiveChatRetention(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("inspect BigQuery raw chat age: %w", err)
	}

	gcs := inspectGCS(ctx, clientOption, constants.GcsFirestoreExportBucketName)

	report := report{
		GeneratedAt: now,
		Cutoff:      cutoff,
		Config: configAudit{
			ConfiguredHistoryRetentionDays: constants.CollectionHistoryRetentionDays,
			GCSExportBucketName:            constants.GcsFirestoreExportBucketName,
			GCPRegion:                      constants.GcpRegion,
		},
		Firestore: firestoreAudit{
			RawLiveChatRowsOlderThanCutoff: firestoreOlderRows,
		},
		BigQuery: bqAudit,
		GCS:      gcs,
	}

	if constants.CollectionHistoryRetentionDays > rawYouTubeDataRetentionDays {
		report.Warnings = append(
			report.Warnings,
			fmt.Sprintf(
				"configured collection-history-retention-days is %d, greater than the raw YouTube data limit of %d days",
				constants.CollectionHistoryRetentionDays,
				rawYouTubeDataRetentionDays,
			),
		)
	}
	if len(gcs.LifecycleRules) == 0 {
		report.Warnings = append(
			report.Warnings,
			"GCS export bucket has no lifecycle rule visible to this audit; historical Firestore exports may remain indefinitely",
		)
	}
	if gcs.SoftDeleteRetention != "" && gcs.SoftDeleteRetention != "0s" {
		report.Warnings = append(
			report.Warnings,
			"GCS soft delete retains deleted objects beyond their live-object deletion time; include that duration when evaluating effective retention",
		)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	return nil
}

func countOldFirestoreRawChat(
	ctx context.Context,
	client repository.DBClient,
	cutoff time.Time,
) (int64, error) {
	query := client.Collection(repository.LiveChatHistory).
		Where(repository.PublishedAtDocProperty, "<", cutoff)
	result, err := query.NewAggregationQuery().WithCount(firestoreCountAlias).Get(ctx)
	if err != nil {
		return 0, fmt.Errorf("run count aggregation: %w", err)
	}
	rawCount, ok := result[firestoreCountAlias]
	if !ok {
		return 0, fmt.Errorf("count aggregation missing alias %q", firestoreCountAlias)
	}
	countValue, ok := rawCount.(*firestorepb.Value)
	if !ok {
		return 0, fmt.Errorf("count aggregation has unexpected type %T", rawCount)
	}
	return countValue.GetIntegerValue(), nil
}

func inspectGCS(
	ctx context.Context,
	clientOption option.ClientOption,
	bucketName string,
) gcsAudit {
	audit := gcsAudit{BucketName: bucketName}
	if strings.TrimSpace(bucketName) == "" {
		audit.AttributesInspectionError = "GCS export bucket name is empty"
		return audit
	}

	client, err := storage.NewClient(ctx, clientOption)
	if err != nil {
		audit.AttributesInspectionError = fmt.Sprintf("initialize GCS client: %v", err)
		return audit
	}
	defer func() {
		if err := client.Close(); err != nil && audit.AttributesInspectionError == "" {
			audit.AttributesInspectionError = fmt.Sprintf("close GCS client: %v", err)
		}
	}()

	attrs, err := client.Bucket(bucketName).Attrs(ctx)
	if err != nil {
		audit.AttributesInspectionError = fmt.Sprintf("read bucket attributes: %v", err)
		return audit
	}

	audit.Location = attrs.Location
	audit.VersioningEnabled = attrs.VersioningEnabled
	audit.LifecycleRules = attrs.Lifecycle.Rules
	if attrs.RetentionPolicy != nil {
		audit.RetentionPeriod = attrs.RetentionPolicy.RetentionPeriod.String()
		audit.RetentionPolicyLocked = attrs.RetentionPolicy.IsLocked
	}
	if attrs.SoftDeletePolicy != nil {
		audit.SoftDeleteRetention = attrs.SoftDeletePolicy.RetentionDuration.String()
	}
	return audit
}
