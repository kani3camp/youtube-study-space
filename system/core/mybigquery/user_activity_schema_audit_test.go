package mybigquery

import (
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
)

func TestValidateUserActivitySchema(t *testing.T) {
	t.Parallel()

	t.Run("canonical schema", func(t *testing.T) {
		t.Parallel()
		hasTakenAt, hasLegacy, err := validateUserActivitySchema(bigquery.Schema{
			&bigquery.FieldSchema{Name: "taken_at", Type: bigquery.TimestampFieldType},
		})
		if err != nil {
			t.Fatalf("validateUserActivitySchema() error = %v", err)
		}
		if !hasTakenAt || hasLegacy {
			t.Fatalf("validateUserActivitySchema() = taken_at:%t legacy:%t, want true false", hasTakenAt, hasLegacy)
		}
	})

	t.Run("legacy timestamp is detected", func(t *testing.T) {
		t.Parallel()
		hasTakenAt, hasLegacy, err := validateUserActivitySchema(bigquery.Schema{
			&bigquery.FieldSchema{Name: "taken_at", Type: bigquery.TimestampFieldType},
			&bigquery.FieldSchema{Name: "timestamp", Type: bigquery.TimestampFieldType},
		})
		if err != nil {
			t.Fatalf("validateUserActivitySchema() error = %v", err)
		}
		if !hasTakenAt || !hasLegacy {
			t.Fatalf("validateUserActivitySchema() = taken_at:%t legacy:%t, want true true", hasTakenAt, hasLegacy)
		}
	})

	t.Run("missing taken_at fails closed", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateUserActivitySchema(bigquery.Schema{
			&bigquery.FieldSchema{Name: "timestamp", Type: bigquery.TimestampFieldType},
		})
		if err == nil || !strings.Contains(err.Error(), "missing canonical field") {
			t.Fatalf("validateUserActivitySchema() error = %v, want missing canonical field", err)
		}
	})

	t.Run("wrong legacy type fails closed", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateUserActivitySchema(bigquery.Schema{
			&bigquery.FieldSchema{Name: "taken_at", Type: bigquery.TimestampFieldType},
			&bigquery.FieldSchema{Name: "timestamp", Type: bigquery.StringFieldType},
		})
		if err == nil || !strings.Contains(err.Error(), "legacy field") {
			t.Fatalf("validateUserActivitySchema() error = %v, want legacy type error", err)
		}
	})
}
