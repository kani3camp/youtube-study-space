package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"

	"app.modules/core/serviceaccess"
)

const maxInputBytes = 16 * 1024

var (
	errInput       = errors.New("invalid private input")
	projectPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	actionPattern  = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type target struct {
	Environment string
	ProjectID   string
	ChannelID   string
}

type request struct {
	Target     target
	Operation  string
	ActionRef  string
	ReasonCode string
}

type options struct {
	mode            string
	manifest        string
	stdin           bool
	allowProduction bool
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
		case "--execute", "--check-config":
			if modeSet {
				return options{}, errInput
			}
			modeSet = true
			if args[i] == "--execute" {
				o.mode = "execute"
			} else {
				o.mode = "check-config"
			}
		case "--allow-production":
			if o.allowProduction {
				return options{}, errInput
			}
			o.allowProduction = true
		default:
			return options{}, errInput
		}
	}
	if (!o.stdin && o.manifest == "") || (o.allowProduction && o.mode != "execute") {
		return options{}, errInput
	}
	return o, nil
}

// Manifests must be ordinary private files. Neither file paths nor filesystem
// errors can become reports. Stdin remains useful for an approved secret pipe.
func inputFrom(o options, stdin io.Reader) (io.Reader, func(), error) {
	if o.stdin {
		return stdin, func() {}, nil
	}
	info, err := os.Lstat(o.manifest)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > maxInputBytes {
		return nil, nil, errInput
	}
	file, err := os.Open(o.manifest)
	if err != nil {
		return nil, nil, errInput
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm()&0o077 != 0 {
		file.Close() //nolint:errcheck // Best-effort closing on a refused input path; no filesystem detail may be exposed.
		return nil, nil, errInput
	}
	return file, func() {
		file.Close() //nolint:errcheck // Read-only input cleanup cannot change operation status or expose filesystem detail.
	}, nil
}

// strictObject rejects unknown, case-variant, duplicate and missing fields.
// encoding/json's ordinary struct decoding accepts duplicate and case-variant
// names, which is unsafe at the private operator boundary.
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
	if (t.Environment != "development" && t.Environment != "production") || !projectPattern.MatchString(t.ProjectID) || !serviceaccess.ValidChannel(t.ChannelID) {
		return target{}, errInput
	}
	return t, nil
}

func readRequest(input io.Reader) (request, error) {
	data, err := io.ReadAll(io.LimitReader(input, maxInputBytes+1))
	if err != nil || len(data) > maxInputBytes {
		return request{}, errInput
	}
	fields, err := strictObject(data, "schemaVersion", "target", "operation", "actionRef", "reasonCode", "confirmation")
	if err != nil || !bytes.Equal(bytes.TrimSpace(fields["schemaVersion"]), []byte("1")) {
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
	for key, destination := range map[string]*string{"operation": &r.Operation, "actionRef": &r.ActionRef, "reasonCode": &r.ReasonCode} {
		value, err := stringValue(fields[key])
		if err != nil {
			return request{}, errInput
		}
		*destination = value
	}
	if !actionPattern.MatchString(r.ActionRef) {
		return request{}, errInput
	}
	switch r.Operation {
	case "block":
		switch r.ReasonCode {
		case "MODERATION", "SECURITY", "POLICY_VIOLATION", "LEGACY_COMPATIBILITY":
		default:
			return request{}, errInput
		}
	case "unblock", "inspect":
		if r.ReasonCode != "" {
			return request{}, errInput
		}
	default:
		return request{}, errInput
	}
	confirmation, err := strictObject(fields["confirmation"], "environment", "projectID", "channelID", "operation", "actionRef")
	if err != nil {
		return request{}, errInput
	}
	confirmedTarget, err := targetFrom(confirmation)
	if err != nil || confirmedTarget != r.Target {
		return request{}, errInput
	}
	for key, expected := range map[string]string{"operation": r.Operation, "actionRef": r.ActionRef} {
		actual, err := stringValue(confirmation[key])
		if err != nil || actual != expected {
			return request{}, errInput
		}
	}
	return r, nil
}

func configuredTarget(get func(string) string) (target, error) {
	t := target{Environment: get("MYPAGE_ENVIRONMENT"), ProjectID: get("GOOGLE_CLOUD_PROJECT")}
	if (t.Environment != "development" && t.Environment != "production") || !projectPattern.MatchString(t.ProjectID) {
		return target{}, errInput
	}
	for _, key := range []string{"GCLOUD_PROJECT", "GCP_PROJECT", "FIREBASE_PROJECT_ID"} {
		if value := get(key); value != "" && value != t.ProjectID {
			return target{}, errInput
		}
	}
	for _, key := range []string{"GOOGLE_APPLICATION_CREDENTIALS", "CREDENTIAL_FILE_LOCATION", "FIRESTORE_EMULATOR_HOST", "FIREBASE_AUTH_EMULATOR_HOST"} {
		if get(key) != "" {
			return target{}, errInput
		}
	}
	return t, nil
}
