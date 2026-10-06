package main

import (
	"strings"
	"testing"

	"app.modules/core/mybigquery"
)

func TestBuildAuditTarget(t *testing.T) {
	t.Parallel()

	target, err := buildAuditTarget("development", "test-project", "test-project")
	if err != nil {
		t.Fatalf("buildAuditTarget() error = %v", err)
	}
	if target.Environment != "development" || target.ProjectID != "test-project" ||
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
		{name: "project mismatch", environment: "production", expected: "prod", actual: "dev", want: "GCP project mismatch"},
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
}
