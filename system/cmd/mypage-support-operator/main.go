// mypage-support-operatorは非公開の操作案をオフラインで検証する。
// --executeはowner-controlled GCP IAMを使い、fresh OAuth用の受付をprivate fileへ渡す。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"syscall"
	"time"

	"app.modules/core/mypage"
)

var (
	opaque  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	project = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
)

type proposal struct {
	Action          string
	Environment     string
	Project         string
	Purpose         mypage.SupportPurpose
	RequestRef      string
	Execute         bool
	Mock            bool
	RequestFile     string
	Handoff         string
	AllowProduction bool
}

type report struct {
	Mode            string `json:"mode"`
	Status          string `json:"status"`
	Code            string `json:"code"`
	ReadOnly        bool   `json:"readOnly"`
	ActualExecution bool   `json:"actualExecution"`
}

func parse(args []string, get func(string) string) (proposal, error) {
	var p proposal
	if get == nil {
		return proposal{}, fmt.Errorf("configuration")
	}
	values := map[string]string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--allow-production" {
			if p.AllowProduction {
				return proposal{}, fmt.Errorf("duplicate")
			}
			p.AllowProduction = true
			continue
		}
		if args[i] == "--mock" {
			if p.Mock {
				return proposal{}, fmt.Errorf("duplicate")
			}
			p.Mock = true
			continue
		}
		if args[i] == "--execute" {
			if p.Execute {
				return proposal{}, fmt.Errorf("duplicate")
			}
			p.Execute = true
			continue
		}
		if i+1 >= len(args) || len(args[i]) < 3 || args[i][:2] != "--" || values[args[i]] != "" || args[i+1] == "" {
			return proposal{}, fmt.Errorf("invalid")
		}
		values[args[i]] = args[i+1]
		i++
	}
	allowed := map[string]bool{"--action": true, "--environment": true, "--project": true, "--purpose": true, "--request-file": true, "--handoff-file": true}
	for key := range values {
		if !allowed[key] {
			return proposal{}, fmt.Errorf("unknown")
		}
	}
	p.Action, p.Environment, p.Project = values["--action"], values["--environment"], values["--project"]
	p.Purpose = mypage.SupportPurpose(values["--purpose"])
	p.RequestFile = values["--request-file"]
	p.Handoff = values["--handoff-file"]
	if !p.Execute && (p.Handoff != "" || p.AllowProduction) {
		return proposal{}, fmt.Errorf("mode")
	}
	if (p.Environment != "development" && p.Environment != "production") || !project.MatchString(p.Project) || p.Environment != get("MYPAGE_ENVIRONMENT") || p.Project != get("GOOGLE_CLOUD_PROJECT") || (p.Mock && p.Execute) {
		return proposal{}, fmt.Errorf("binding")
	}
	for _, alias := range []string{"GCLOUD_PROJECT", "GCP_PROJECT", "FIREBASE_PROJECT_ID", "CLOUDSDK_CORE_PROJECT"} {
		if value := get(alias); value != "" && value != p.Project {
			return proposal{}, fmt.Errorf("project alias")
		}
	}
	for _, key := range []string{"FIRESTORE_EMULATOR_HOST", "FIREBASE_AUTH_EMULATOR_HOST", "STORAGE_EMULATOR_HOST", "BIGQUERY_EMULATOR_HOST", "GOOGLE_APPLICATION_CREDENTIALS", "CREDENTIAL_FILE_LOCATION", "GCE_METADATA_HOST", "GCE_METADATA_IP"} {
		if get(key) != "" {
			return proposal{}, fmt.Errorf("live configuration")
		}
	}
	if humanAction(p.Action) {
		if p.RequestFile == "" || !intakePurpose(p.Purpose) {
			return proposal{}, fmt.Errorf("intake")
		}
		for key := range values {
			if key != "--action" && key != "--environment" && key != "--project" && key != "--purpose" && key != "--request-file" && key != "--handoff-file" {
				return proposal{}, fmt.Errorf("intake")
			}
		}
		return p, nil
	}
	return proposal{}, fmt.Errorf("action")
}

func privateFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() < 1 || info.Size() > limit {
		return nil, fmt.Errorf("private file")
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("private file")
	}
	defer f.Close() //nolint:errcheck // Read-only close; never expose filesystem diagnostics.
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0o600 || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("private file")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || len(data) < 1 || int64(len(data)) > limit {
		return nil, fmt.Errorf("private file")
	}
	return data, nil
}

func command(args []string, get func(string) string, out io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return commandWithIntake(ctx, args, get, out, mockIntakeFactory, liveIntakeFactory)
}

func commandWithIntake(ctx context.Context, args []string, get func(string) string, out io.Writer, factory intakeFactory, live ...intakeFactory) int {
	r := report{Mode: "preview", Status: "refused", Code: "INVALID_INPUT", ReadOnly: true, ActualExecution: false}
	finish := func(code int) int {
		if json.NewEncoder(out).Encode(r) != nil {
			return 1
		}
		return code
	}
	p, err := parse(args, get)
	if err != nil {
		return finish(2)
	}
	var input humanRequest
	if humanAction(p.Action) {
		input, err = readHumanRequest(p.RequestFile, p.Action)
		if err != nil {
			return finish(2)
		}
	}
	if p.Execute && len(live) == 1 && live[0] != nil && p.Handoff != "" && (p.Environment != "production" || p.AllowProduction) {
		r.Mode, r.Status, r.Code, r.ReadOnly, r.ActualExecution = "execute", "blocked", "INTAKE_INCOMPLETE", false, true
		code := runHumanIntake(ctx, p, input, live[0])
		if code == "MOCK_AWAITING_PROOF" {
			r.Status, r.Code = "awaiting-proof", "AWAITING_FRESH_OAUTH"
			return finish(0)
		}
		if code == "MOCK_RECEIPT_MATCHED" {
			r.Status, r.Code, r.ReadOnly = "receipt-matched", "RECEIPT_MATCHED", true
			return finish(0)
		}
		return finish(1)
	}
	if p.Execute {
		r.Mode, r.Code, r.Status = "execute", "TRUSTED_OPERATOR_ADAPTER_UNAVAILABLE", "blocked"
		return finish(1)
	}
	if p.Mock {
		r.Mode, r.Status, r.Code, r.ReadOnly = "mock", "blocked", "MOCK_TARGET_REQUIRED", false
		if !fixedMockIntake(p, input) {
			return finish(2)
		}
		r.Code = runHumanIntake(ctx, p, input, factory)
		if r.Code != "MOCK_AWAITING_PROOF" && r.Code != "MOCK_RECEIPT_MATCHED" {
			return finish(1)
		}
		r.Status = "mock-only"
		r.ReadOnly = p.Action == "inspect"
		return finish(0)
	}
	r.Status, r.Code = "proposal-valid", "READ_ONLY_PREVIEW"
	return finish(0)
}

func main() { os.Exit(command(os.Args[1:], os.Getenv, os.Stdout)) }
