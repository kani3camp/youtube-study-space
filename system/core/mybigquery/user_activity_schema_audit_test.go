package mybigquery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
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

type auditRoundTripper func(*http.Request) (*http.Response, error)

func (transport auditRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestAuditQueryWireBudgetAndReadOnlyScope(t *testing.T) {
	for _, mode := range []string{"legacy", "canonical", "budget rejected", "invalid budget"} {
		t.Run(mode, func(t *testing.T) {
			metadataCalls, queryCalls := 0, 0
			transport := auditRoundTripper(func(request *http.Request) (*http.Response, error) {
				status := http.StatusOK
				response := ""
				switch {
				case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/datasets/firestore_export/tables/user-activity-history"):
					metadataCalls++
					fields := []map[string]string{{"name": "taken_at", "type": "TIMESTAMP"}}
					if mode != "canonical" {
						fields = append(fields, map[string]string{"name": "timestamp", "type": "TIMESTAMP"})
					}
					body, err := json.Marshal(map[string]any{"type": "TABLE", "schema": map[string]any{"fields": fields}})
					if err != nil {
						t.Fatal(err)
					}
					response = string(body)
				case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/projects/synthetic-audit-project/queries"):
					queryCalls++
					var wire struct {
						Query              string `json:"query"`
						MaximumBytesBilled string `json:"maximumBytesBilled"`
						Location           string `json:"location"`
						UseLegacySQL       bool   `json:"useLegacySql"`
					}
					if err := json.NewDecoder(request.Body).Decode(&wire); err != nil {
						t.Fatal(err)
					}
					if wire.MaximumBytesBilled != "10485760" || wire.Location != "asia-southeast2" || wire.UseLegacySQL {
						t.Errorf("wire query must carry explicit bounded standard-SQL settings: %#v", wire)
					}
					if strings.Count(wire.Query, "COUNTIF(") != 4 || !strings.HasSuffix(wire.Query, "FROM `synthetic-audit-project.firestore_export.user-activity-history`") {
						t.Errorf("wire SQL must be the exact four aggregates on one table: %s", wire.Query)
					}
					if mode == "budget rejected" {
						status = http.StatusBadRequest
						response = `{"error":{"code":400,"message":"synthetic maximum bytes billed exceeded","errors":[{"reason":"billingTierLimitExceeded"}]}}`
					} else {
						response = `{"jobComplete":true,"totalRows":"1","schema":{"fields":[{"name":"legacy_non_null","type":"INTEGER"},{"name":"legacy_only","type":"INTEGER"},{"name":"both_equal","type":"INTEGER"},{"name":"both_different","type":"INTEGER"}]},"rows":[{"f":[{"v":"1"},{"v":"0"},{"v":"1"},{"v":"0"}]}]}`
					}
				default:
					t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
					status = http.StatusBadRequest
					response = `{"error":{"code":400,"message":"unexpected synthetic request"}}`
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(response)), Request: request}, nil
			})
			client, err := bigquery.NewClient(context.Background(), "synthetic-audit-project", option.WithHTTPClient(&http.Client{Transport: transport}))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := client.Close(); err != nil {
					t.Errorf("close synthetic client: %v", err)
				}
			}()
			controller := &BigqueryController{Client: client, WorkingRegion: "asia-southeast2"}
			budget := int64(10485760)
			if mode == "invalid budget" {
				budget = 0
			}
			audit, err := controller.InspectUserActivityLegacyTimestamp(context.Background(), budget)
			if mode == "budget rejected" || mode == "invalid budget" {
				if err == nil {
					t.Fatal("expected fail-closed budget error")
				}
			} else if err != nil {
				t.Fatalf("audit failed: %v", err)
			}
			wantMetadata, wantQueries := 1, 1
			if mode == "canonical" {
				wantQueries = 0
				if !audit.Canonical {
					t.Error("canonical metadata must skip query")
				}
			}
			if mode == "invalid budget" {
				wantMetadata, wantQueries = 0, 0
			}
			if metadataCalls != wantMetadata || queryCalls != wantQueries {
				t.Errorf("API calls metadata/query = %d/%d, want %d/%d; no retry or extra query allowed", metadataCalls, queryCalls, wantMetadata, wantQueries)
			}
		})
	}
}

func TestAuditBudgetBoundaries(t *testing.T) {
	for _, value := range []int64{-1, 0, MaxUserActivityAuditBytes + 1} {
		if ValidateUserActivityAuditBudget(value) == nil {
			t.Errorf("budget %d must fail", value)
		}
	}
	for _, value := range []int64{1, MaxUserActivityAuditBytes} {
		if err := ValidateUserActivityAuditBudget(value); err != nil {
			t.Errorf("budget %d must pass: %v", value, err)
		}
	}
}
