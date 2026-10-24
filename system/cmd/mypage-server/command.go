package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// Configuration validation is not an infrastructure check or live E2E test.
// All report fields have bounded, fixed values; supplied configuration and
// dependency errors must never appear in this shareable preparation evidence.
type configurationReport struct {
	SchemaVersion               int    `json:"schemaVersion"`
	Mode                        string `json:"mode"`
	Offline                     bool   `json:"offline"`
	Configuration               string `json:"configuration"`
	StartupReadinessDeclaration string `json:"startupReadinessDeclaration"`
	InfrastructureReadiness     string `json:"infrastructureReadiness"`
	LiveE2E                     string `json:"liveE2E"`
}

// command dispatches before startup so check-only and invalid argument paths
// cannot reach ADC discovery, external clients, or the HTTP listener.
func command(args []string, get func(string) string, stdout, stderr io.Writer, start func() error) int {
	if len(args) == 0 {
		if err := start(); err != nil {
			fmt.Fprintln(stderr, startupDiagnostic(err)) //nolint:errcheck // Best-effort fixed diagnostic; output failure must not change the failure status or expose raw errors.
			return 1
		}
		return 0
	}
	if len(args) != 1 || args[0] != "--check-config" {
		fmt.Fprintln(stderr, "Usage: mypage-server [--check-config]") //nolint:errcheck // Best-effort fixed usage; preserve argument failure even when stderr cannot be written.
		return 2
	}
	report := configurationReport{
		SchemaVersion: 1, Mode: "check-config", Offline: true,
		Configuration: "valid", StartupReadinessDeclaration: "pending",
		InfrastructureReadiness: "pending", LiveE2E: "pending",
	}
	if get("MYPAGE_INFRASTRUCTURE_READY") == "true" {
		report.StartupReadinessDeclaration = "declared"
	}
	exitCode := 0
	if _, err := configurationValuesFrom(get); err != nil {
		report.Configuration = "invalid"
		exitCode = 1
	}
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		fmt.Fprintln(stderr, "MyPage configuration report unavailable") //nolint:errcheck // Best-effort fixed diagnostic; there is no safe fallback when the report and stderr writers fail.
		return 1
	}
	return exitCode
}

// Retain the existing safe startup stages for release diagnosis. This exact
// allowlist prevents unexpected dependency/callback errors from becoming logs.
func startupDiagnostic(err error) string {
	switch err.Error() {
	case "MyPage configuration unavailable":
		return "MyPage configuration unavailable"
	case "MyPage keyless bootstrap unavailable":
		return "MyPage keyless bootstrap unavailable"
	case "MyPage database bootstrap unavailable":
		return "MyPage database bootstrap unavailable"
	case "MyPage provider unavailable":
		return "MyPage provider unavailable"
	case "MyPage metadata provider unavailable":
		return "MyPage metadata provider unavailable"
	case "MyPage handler unavailable":
		return "MyPage handler unavailable"
	case "MyPage HTTP server unavailable":
		return "MyPage HTTP server unavailable"
	default:
		return "MyPage server unavailable"
	}
}
