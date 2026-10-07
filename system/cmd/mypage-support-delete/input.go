package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"syscall"

	"app.modules/core/supportdelete"
)

const maxInputBytes = 16 * 1024

var (
	errInput       = errors.New("invalid private input")
	projectPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	channelPattern = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
	refPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type target = supportdelete.Target

type request = supportdelete.Selector

type options struct {
	mode     string
	manifest string
	stdin    bool
	recover  bool
}

func parseOptions(args []string) (options, error) {
	o := options{mode: "plan"}
	modeSet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stdin":
			if o.stdin || o.manifest != "" {
				return options{}, errInput
			}
			o.stdin = true
		case "--manifest":
			if o.stdin || o.manifest != "" || i+1 >= len(args) || args[i+1] == "" {
				return options{}, errInput
			}
			i++
			o.manifest = args[i]
		case "--check-config", "--mock", "--execute":
			if modeSet {
				return options{}, errInput
			}
			modeSet = true
			o.mode = args[i][2:]
		case "--recover":
			if o.recover {
				return options{}, errInput
			}
			o.recover = true
		default:
			return options{}, errInput
		}
	}
	if (!o.stdin && o.manifest == "") || (o.recover && o.mode != "mock") {
		return options{}, errInput
	}
	return o, nil
}

func privateRegular(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Mode().Perm() == 0o600 && info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) == 0 && info.Size() <= maxInputBytes
}

// File input must stay an ordinary private file across the open. O_NOFOLLOW
// rejects a final-component symlink even if the path changes after Lstat, and
// O_NONBLOCK prevents a replacement FIFO from blocking before fstat rejects it.
// Neither file paths nor raw filesystem errors may become operator reports.
func inputFrom(o options, stdin io.Reader) (io.Reader, func(), error) {
	if o.stdin {
		if stdin == nil {
			return nil, nil, errInput
		}
		return stdin, func() {}, nil
	}
	info, err := os.Lstat(o.manifest)
	if err != nil || !privateRegular(info) {
		return nil, nil, errInput
	}
	file, err := os.OpenFile(o.manifest, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, errInput
	}
	opened, err := file.Stat()
	if err != nil || !privateRegular(opened) || !os.SameFile(info, opened) {
		file.Close() //nolint:errcheck // Best-effort read-only cleanup; raw filesystem details must remain private.
		return nil, nil, errInput
	}
	return file, func() {
		file.Close() //nolint:errcheck // Read-only cleanup cannot alter execution status or expose filesystem details.
	}, nil
}

// Strict token decoding rejects duplicate, case-variant, unknown and missing
// fields. Ordinary struct decoding accepts duplicate and case-variant names.
func strictObject(data []byte, fields ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, errInput
	}
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[field] = true
	}
	values := make(map[string]json.RawMessage, len(fields))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, errInput
		}
		key, ok := token.(string)
		if !ok || !allowed[key] || values[key] != nil {
			return nil, errInput
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, errInput
		}
		values[key] = value
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') || len(values) != len(fields) {
		return nil, errInput
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errInput
	}
	return values, nil
}

func stringValue(raw json.RawMessage) (string, error) {
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || data[0] != '"' {
		return "", errInput
	}
	var value string
	if json.Unmarshal(data, &value) != nil {
		return "", errInput
	}
	return value, nil
}

func targetFrom(values map[string]json.RawMessage) (target, error) {
	var t target
	for key, destination := range map[string]*string{"environment": &t.Environment, "projectID": &t.ProjectID, "channelID": &t.ChannelID} {
		value, err := stringValue(values[key])
		if err != nil {
			return target{}, errInput
		}
		*destination = value
	}
	if (t.Environment != "development" && t.Environment != "production") || !projectPattern.MatchString(t.ProjectID) || !channelPattern.MatchString(t.ChannelID) {
		return target{}, errInput
	}
	return t, nil
}

func readRequest(input io.Reader) (request, error) {
	data, err := io.ReadAll(io.LimitReader(input, maxInputBytes+1))
	if err != nil || len(data) > maxInputBytes {
		return request{}, errInput
	}
	fields, err := strictObject(data, "schemaVersion", "target", "operation", "requestRef", "executionRef", "proofRef", "confirmation")
	if err != nil || !bytes.Equal(bytes.TrimSpace(fields["schemaVersion"]), []byte("1")) {
		return request{}, errInput
	}
	operation, err := stringValue(fields["operation"])
	if err != nil || operation != "delete" {
		return request{}, errInput
	}
	targetFields, err := strictObject(fields["target"], "environment", "projectID", "channelID")
	if err != nil {
		return request{}, errInput
	}
	t, err := targetFrom(targetFields)
	if err != nil {
		return request{}, errInput
	}
	r := request{Target: t}
	for key, destination := range map[string]*string{"requestRef": &r.RequestRef, "executionRef": &r.ExecutionRef, "proofRef": &r.ProofRef} {
		value, err := stringValue(fields[key])
		if err != nil || !refPattern.MatchString(value) {
			return request{}, errInput
		}
		*destination = value
	}
	confirmation, err := strictObject(fields["confirmation"], "environment", "projectID", "channelID", "operation", "requestRef", "executionRef", "proofRef")
	if err != nil {
		return request{}, errInput
	}
	confirmedTarget, err := targetFrom(confirmation)
	if err != nil || confirmedTarget != r.Target {
		return request{}, errInput
	}
	for key, expected := range map[string]string{"operation": "delete", "requestRef": r.RequestRef, "executionRef": r.ExecutionRef, "proofRef": r.ProofRef} {
		actual, err := stringValue(confirmation[key])
		if err != nil || actual != expected {
			return request{}, errInput
		}
	}
	// Confirmation binds operator intent only. The proof reference is a selector
	// for independent trusted evidence; no manifest assertion can establish it.
	return r, nil
}

func configuredTarget(get func(string) string) (target, error) {
	if get == nil {
		return target{}, errInput
	}
	t := target{Environment: get("MYPAGE_ENVIRONMENT"), ProjectID: get("GOOGLE_CLOUD_PROJECT")}
	if (t.Environment != "development" && t.Environment != "production") || !projectPattern.MatchString(t.ProjectID) {
		return target{}, errInput
	}
	for _, key := range []string{"GCLOUD_PROJECT", "GCP_PROJECT", "FIREBASE_PROJECT_ID", "CLOUDSDK_CORE_PROJECT"} {
		if value := get(key); value != "" && value != t.ProjectID {
			return target{}, errInput
		}
	}
	for _, key := range []string{"GOOGLE_APPLICATION_CREDENTIALS", "CREDENTIAL_FILE_LOCATION", "FIRESTORE_EMULATOR_HOST", "FIREBASE_AUTH_EMULATOR_HOST", "STORAGE_EMULATOR_HOST", "BIGQUERY_EMULATOR_HOST", "GCE_METADATA_HOST", "GCE_METADATA_IP"} {
		if get(key) != "" {
			return target{}, errInput
		}
	}
	return t, nil
}
