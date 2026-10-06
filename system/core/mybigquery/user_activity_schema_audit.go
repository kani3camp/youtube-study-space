package mybigquery

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

type UserActivitySchemaAudit struct {
	Canonical          bool  `json:"canonical"`
	HasTakenAt         bool  `json:"has_taken_at"`
	HasLegacyTimestamp bool  `json:"has_legacy_timestamp"`
	LegacyNonNull      int64 `json:"legacy_non_null"`
	LegacyOnly         int64 `json:"legacy_only"`
	BothEqual          int64 `json:"both_equal"`
	BothDifferent      int64 `json:"both_different"`
}

// MaxUserActivityAuditBytes is the fail-closed ceiling for this one-table audit.
// Raising it requires a separate reviewed source and cost approval.
const MaxUserActivityAuditBytes int64 = 1 << 30

func ValidateUserActivityAuditBudget(maxBytesBilled int64) error {
	if maxBytesBilled <= 0 || maxBytesBilled > MaxUserActivityAuditBytes {
		return fmt.Errorf("audit maximum bytes billed must be positive and at most 1 GiB")
	}
	return nil
}

func validateUserActivitySchema(schema bigquery.Schema) (hasTakenAt bool, hasLegacyTimestamp bool, err error) {
	var takenAt *bigquery.FieldSchema
	var legacyTimestamp *bigquery.FieldSchema

	for _, field := range schema {
		switch field.Name {
		case "taken_at":
			takenAt = field
		case "timestamp":
			legacyTimestamp = field
		}
	}

	if takenAt == nil {
		return false, legacyTimestamp != nil, fmt.Errorf("user-activity-history is missing canonical field %q", "taken_at")
	}
	if takenAt.Type != bigquery.TimestampFieldType {
		return true, legacyTimestamp != nil, fmt.Errorf(
			"user-activity-history field %q has type %q, want %q",
			"taken_at",
			takenAt.Type,
			bigquery.TimestampFieldType,
		)
	}
	if legacyTimestamp != nil && legacyTimestamp.Type != bigquery.TimestampFieldType {
		return true, true, fmt.Errorf(
			"user-activity-history legacy field %q has type %q, want %q",
			"timestamp",
			legacyTimestamp.Type,
			bigquery.TimestampFieldType,
		)
	}

	return true, legacyTimestamp != nil, nil
}

func (c *BigqueryController) InspectUserActivityLegacyTimestamp(ctx context.Context, maxBytesBilled int64) (UserActivitySchemaAudit, error) {
	if err := ValidateUserActivityAuditBudget(maxBytesBilled); err != nil {
		return UserActivitySchemaAudit{}, err
	}
	table := c.Client.Dataset(DatasetName).Table(UserActivityHistoryMainTableName)
	metadata, err := table.Metadata(ctx)
	if err != nil {
		return UserActivitySchemaAudit{}, fmt.Errorf("read user-activity-history metadata: %w", err)
	}

	hasTakenAt, hasLegacyTimestamp, err := validateUserActivitySchema(metadata.Schema)
	if err != nil {
		return UserActivitySchemaAudit{}, err
	}

	audit := UserActivitySchemaAudit{
		Canonical:          !hasLegacyTimestamp,
		HasTakenAt:         hasTakenAt,
		HasLegacyTimestamp: hasLegacyTimestamp,
	}
	if !hasLegacyTimestamp {
		return audit, nil
	}

	query := c.Client.Query(fmt.Sprintf(
		"SELECT\n"+
			"  COUNTIF(`timestamp` IS NOT NULL) AS legacy_non_null,\n"+
			"  COUNTIF(`timestamp` IS NOT NULL AND `taken_at` IS NULL) AS legacy_only,\n"+
			"  COUNTIF(`timestamp` IS NOT NULL AND `taken_at` IS NOT NULL AND `timestamp` = `taken_at`) AS both_equal,\n"+
			"  COUNTIF(`timestamp` IS NOT NULL AND `taken_at` IS NOT NULL AND `timestamp` != `taken_at`) AS both_different\n"+
			"FROM `%s.%s.%s`",
		c.Client.Project(),
		DatasetName,
		UserActivityHistoryMainTableName,
	))
	query.Location = c.WorkingRegion
	query.MaxBytesBilled = maxBytesBilled

	rows, err := query.Read(ctx)
	if err != nil {
		return UserActivitySchemaAudit{}, fmt.Errorf("run aggregate-only user-activity schema audit: %w", err)
	}

	var counts struct {
		LegacyNonNull int64 `bigquery:"legacy_non_null"`
		LegacyOnly    int64 `bigquery:"legacy_only"`
		BothEqual     int64 `bigquery:"both_equal"`
		BothDifferent int64 `bigquery:"both_different"`
	}
	if err := rows.Next(&counts); err != nil {
		return UserActivitySchemaAudit{}, fmt.Errorf("read aggregate-only user-activity schema audit: %w", err)
	}
	if err := rows.Next(&struct{}{}); err != iterator.Done {
		if err == nil {
			return UserActivitySchemaAudit{}, fmt.Errorf("aggregate-only user-activity schema audit returned more than one row")
		}
		return UserActivitySchemaAudit{}, fmt.Errorf("finish aggregate-only user-activity schema audit: %w", err)
	}

	audit.LegacyNonNull = counts.LegacyNonNull
	audit.LegacyOnly = counts.LegacyOnly
	audit.BothEqual = counts.BothEqual
	audit.BothDifferent = counts.BothDifferent
	return audit, nil
}
