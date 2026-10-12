// mypage-support-operator validates a private operator proposal offline. No
// identity adapter or credentials are linked; --execute is always denied.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"app.modules/core/mypage"
)

var (
	opaque  = regexp.MustCompile(`^[a-f0-9]{64}$`)
	project = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
)

type proposal struct {
	Action           string
	Environment      string
	Project          string
	Purpose          mypage.SupportPurpose
	RequestRef       string
	ProofRef         string
	OperationID      string
	ExpectedRevision int64
	ReplyFile        string
	ReplyOperationID string
	ActionEvidence   string
	DeliveryAck      string
	Execute          bool
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
	allowed := map[string]bool{"--action": true, "--environment": true, "--project": true, "--purpose": true, "--request-ref": true, "--proof-ref": true, "--operation-id": true, "--expected-revision": true, "--reply-file": true, "--reply-operation-id": true, "--action-evidence": true, "--delivery-ack": true}
	for key := range values {
		if !allowed[key] {
			return proposal{}, fmt.Errorf("unknown")
		}
	}
	p.Action, p.Environment, p.Project = values["--action"], values["--environment"], values["--project"]
	p.Purpose = mypage.SupportPurpose(values["--purpose"])
	p.RequestRef, p.ProofRef, p.OperationID = values["--request-ref"], values["--proof-ref"], values["--operation-id"]
	p.ReplyFile, p.ReplyOperationID, p.ActionEvidence, p.DeliveryAck = values["--reply-file"], values["--reply-operation-id"], values["--action-evidence"], values["--delivery-ack"]
	revision, err := strconv.ParseInt(values["--expected-revision"], 10, 64)
	if err != nil || revision < 0 || (p.Environment != "development" && p.Environment != "production") || !project.MatchString(p.Project) || p.Environment != get("MYPAGE_ENVIRONMENT") || p.Project != get("GOOGLE_CLOUD_PROJECT") || !opaque.MatchString(p.RequestRef) || !opaque.MatchString(p.ProofRef) || !opaque.MatchString(p.OperationID) {
		return proposal{}, fmt.Errorf("binding")
	}
	p.ExpectedRevision = revision
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
	switch p.Action {
	case "reply":
		if p.Purpose != mypage.SupportDelete && p.Purpose != mypage.SupportRevoke && p.Purpose != mypage.SupportDisclosure {
			return proposal{}, fmt.Errorf("purpose")
		}
		if p.ReplyFile == "" || p.ReplyOperationID != "" || p.ActionEvidence != "" || p.DeliveryAck != "" {
			return proposal{}, fmt.Errorf("reply")
		}
	case "complete":
		if p.Purpose != mypage.SupportRevoke && p.Purpose != mypage.SupportDisclosure {
			return proposal{}, fmt.Errorf("completion purpose")
		}
		if p.ReplyFile != "" || !opaque.MatchString(p.ReplyOperationID) || !opaque.MatchString(p.ActionEvidence) || !opaque.MatchString(p.DeliveryAck) || p.ExpectedRevision < 1 {
			return proposal{}, fmt.Errorf("completion")
		}
	default:
		return proposal{}, fmt.Errorf("action")
	}
	return p, nil
}

func privateReply(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() < 1 || info.Size() > 2000 {
		return fmt.Errorf("private file")
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("private file")
	}
	defer f.Close() //nolint:errcheck // Read-only close; never expose filesystem diagnostics.
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0o600 || !os.SameFile(info, opened) {
		return fmt.Errorf("private file")
	}
	data, err := io.ReadAll(io.LimitReader(f, 2001))
	if err != nil || len(data) < 1 || len(data) > 2000 {
		return fmt.Errorf("private file")
	}
	// Validation stays local; neither the body nor raw errors reach output.
	body := string(data)
	if body != strings.TrimSpace(body) || !utf8.Valid(data) {
		return fmt.Errorf("reply content")
	}
	for _, b := range body {
		if unicode.IsControl(b) && b != '\n' {
			return fmt.Errorf("reply content")
		}
	}
	return nil
}

func command(args []string, get func(string) string, out io.Writer) int {
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
	if p.Action == "reply" && privateReply(p.ReplyFile) != nil {
		return finish(2)
	}
	if p.Execute {
		r.Mode, r.Code, r.Status = "execute", "TRUSTED_OPERATOR_ADAPTER_UNAVAILABLE", "blocked"
		return finish(1)
	}
	r.Status, r.Code = "proposal-valid", "READ_ONLY_PREVIEW"
	return finish(0)
}

func main() { os.Exit(command(os.Args[1:], os.Getenv, os.Stdout)) }
