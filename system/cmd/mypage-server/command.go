package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// 設定検証は基盤検査や実環境のE2Eではない。報告項目はすべて範囲を限定した固定値とする。
// 共有可能な準備記録へ、入力設定や依存先エラーを出さない。
type configurationReport struct {
	SchemaVersion               int    `json:"schemaVersion"`
	Mode                        string `json:"mode"`
	Offline                     bool   `json:"offline"`
	Configuration               string `json:"configuration"`
	StartupReadinessDeclaration string `json:"startupReadinessDeclaration"`
	InfrastructureReadiness     string `json:"infrastructureReadiness"`
	LiveE2E                     string `json:"liveE2E"`
}

// commandは起動前に処理を分岐し、検査専用モードや不正引数が
// ADC探索・外部クライアント・HTTP待ち受けへ到達しないようにする。
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

// 配備診断用に、既存の安全な起動段階を維持する。
// 厳密な許可リストで、予期しない依存先やコールバックのエラーをログに出さない。
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
