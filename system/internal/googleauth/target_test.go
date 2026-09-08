package googleauth

import (
	"strings"
	"testing"
)

func TestProjectIDForEnvironment(t *testing.T) {
	tests := []struct {
		environment string
		want        string
	}{
		{environment: "development", want: DevelopmentProjectID},
		{environment: "production", want: ProductionProjectID},
	}
	for _, tt := range tests {
		t.Run(tt.environment, func(t *testing.T) {
			got, err := ProjectIDForEnvironment(tt.environment)
			if err != nil {
				t.Fatalf("ProjectIDForEnvironment returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("project ID = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProjectIDForEnvironmentRejectsUnknownEnvironment(t *testing.T) {
	_, err := ProjectIDForEnvironment("staging")
	if err == nil || !strings.Contains(err.Error(), "development or production") {
		t.Fatalf("expected environment error, got %v", err)
	}
}
