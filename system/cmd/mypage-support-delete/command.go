package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"app.modules/core/supportdelete"
)

const (
	mockProject = "demo-youtube-study-space-ci"
	mockChannel = "UCsynthetic0000000000001"
)

type mockRun func(context.Context, supportdelete.Selector, bool) (supportdelete.Outcome, error)

// Reports contain only a fixed vocabulary. Selectors, references, paths,
// timestamps, input data and dependency errors are never report fields. A
// completed mock exercises synthetic fixtures and cannot claim real deletion.
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

func command(ctx context.Context, args []string, get func(string) string, stdin io.Reader, stdout, stderr io.Writer, run mockRun) int {
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
		// A plan validates private binding only, without reading support records,
		// discovering credentials or constructing an execution adapter.
		out.Status, out.Code, out.Stage = "planned", "PLAN_ONLY", "offline"
		return finish(0)
	}
	if o.mode == "execute" {
		// The command has no live adapter or live bootstrap. An execution flag
		// must never fall back to ADC, synthetic ports or an injected mock runner.
		out.Status, out.Code, out.Stage = "unavailable", "LIVE_ADAPTERS_UNAVAILABLE", "blocked"
		return finish(1)
	}
	if r.Target != (target{Environment: "development", ProjectID: mockProject, ChannelID: mockChannel}) {
		out.Code, out.Stage = "MOCK_TARGET_REQUIRED", "blocked"
		return finish(2)
	}
	out.Status, out.Code, out.Stage = "blocked", "WORKFLOW_BLOCKED", "blocked"
	if run == nil || ctx == nil || ctx.Err() != nil {
		return finish(1)
	}
	// Neither proof provenance, evidence, scope results nor checkpoint state
	// come from the manifest. Recovery remains a request to the trusted mock
	// runner, which must resolve its own independently seeded checkpoint.
	result, err := run(ctx, r, o.recover)
	if err != nil {
		return finish(1)
	}
	if result.Mode != "mock" || !result.Offline || result.ActualExecution || result.Code != "MOCK_COMPLETED" || result.Stage != "completed" {
		out.Code = "MOCK_RESULT_INVALID"
		return finish(1)
	}
	// Reconstruct the fixed report instead of forwarding even trusted output.
	out.Status, out.Code, out.Stage = "mock-only", "MOCK_COMPLETED", "completed"
	return finish(0)
}
