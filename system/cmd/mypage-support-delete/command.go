package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// 報告には固定の語彙だけを使う。対象指定・参照・パス・時刻・入力データ・依存先エラーは含めない。
type report struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Mode            string `json:"mode"`
	Operation       string `json:"operation"`
	Offline         bool   `json:"offline"`
	ActualExecution bool   `json:"actualExecution"`
	Status          string `json:"status"`
	Code            string `json:"code"`
	Stage           string `json:"stage"`
}

func initialReport() report {
	return report{SchemaVersion: 1, Mode: "plan", Operation: "delete", Offline: true, ActualExecution: false, Status: "refused", Code: "INVALID_ARGUMENTS", Stage: "input"}
}

func command(ctx context.Context, args []string, get func(string) string, stdin io.Reader, stdout, stderr io.Writer, live ...liveRun) int {
	out := initialReport()
	finish := func(exitCode int) int {
		if json.NewEncoder(stdout).Encode(out) != nil {
			fmt.Fprintln(stderr, "MyPage support deletion report unavailable") //nolint:errcheck // Best-effort fixed diagnostic; raw errors and private input must never become output.
			return 1
		}
		return exitCode
	}
	o, err := parseOptions(args)
	if err != nil {
		return finish(2)
	}
	out.Mode = o.mode
	config, err := configuredTarget(get)
	if err != nil {
		out.Code, out.Stage = "CONFIGURATION_INVALID", "configuration"
		return finish(2)
	}
	input, closeInput, err := inputFrom(o, stdin)
	if err != nil {
		out.Code = "INVALID_INPUT"
		return finish(2)
	}
	r, err := readRequest(input)
	closeInput()
	if err != nil {
		out.Code = "INVALID_INPUT"
		return finish(2)
	}
	if r.Target.Environment != config.Environment || r.Target.ProjectID != config.ProjectID {
		out.Code, out.Stage = "TARGET_MISMATCH", "configuration"
		return finish(2)
	}
	if o.mode == "check-config" {
		out.Status, out.Code, out.Stage = "configuration-valid", "CONFIGURATION_VALID", "offline"
		return finish(0)
	}
	if o.mode == "plan" {
		// 計画では非公開の対応情報だけを検証し、サポート記録の読み取り・
		// 認証情報の探索・実行アダプターの構築は行わない。
		out.Status, out.Code, out.Stage = "planned", "PLAN_ONLY", "offline"
		return finish(0)
	}
	if o.mode == "execute" {
		if len(live) != 1 || live[0] == nil {
			out.Status, out.Code, out.Stage = "unavailable", "LIVE_ADAPTERS_UNAVAILABLE", "blocked"
			return finish(1)
		}
		if r.Target.Environment == "production" && !o.allowProduction {
			out.Code = "PRODUCTION_CONFIRMATION_REQUIRED"
			return finish(2)
		}
		result, err := live[0](ctx, r, o)
		out.Offline = false
		out.ActualExecution = result.ActualExecution
		out.Status, out.Code, out.Stage = "blocked", "DELETION_INCOMPLETE", "blocked"
		if result.Code == "MANUAL_GATE_REQUIRED" {
			out.Code = "MANUAL_GATE_REQUIRED"
		}
		if err != nil {
			return finish(1)
		}
		if result.Code != "COMPLETED" || result.Mode != "live" {
			return finish(1)
		}
		out.Status, out.Code, out.Stage = "completed", "COMPLETED", "completed"
		return finish(0)
	}

	return finish(2)
}
