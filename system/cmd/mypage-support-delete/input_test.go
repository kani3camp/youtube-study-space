package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testProject  = "demo-youtube-study-space-ci"
	testChannel  = "UCsynthetic0000000000001"
	testRequest  = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testExecute  = "1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	testProof    = "2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	privateError = "private-operation-data-and-credential-detail"
)

func testEnvironment() map[string]string {
	return map[string]string{"MYPAGE_ENVIRONMENT": "development", "GOOGLE_CLOUD_PROJECT": testProject}
}

func requestJSON(t *testing.T, edit func(map[string]any)) string {
	t.Helper()
	v := map[string]any{
		"schemaVersion": 1,
		"target":        map[string]any{"environment": "development", "projectID": testProject, "channelID": testChannel},
		"operation":     "delete", "requestRef": testRequest, "executionRef": testExecute, "proofRef": testProof,
		"confirmation": map[string]any{"environment": "development", "projectID": testProject, "channelID": testChannel, "operation": "delete", "requestRef": testRequest, "executionRef": testExecute, "proofRef": testProof},
	}
	if edit != nil {
		edit(v)
	}
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func objectField(t *testing.T, value map[string]any, key string) map[string]any {
	t.Helper()
	field, ok := value[key].(map[string]any)
	require.True(t, ok)
	return field
}

func TestStrictPrivateRequestRequiresExactSchemaAndConfirmation(t *testing.T) {
	base := requestJSON(t, nil)
	valid, err := readRequest(strings.NewReader(base))
	require.NoError(t, err)
	require.Equal(t, request{Target: target{Environment: "development", ProjectID: testProject, ChannelID: testChannel}, RequestRef: testRequest, ExecutionRef: testExecute, ProofRef: testProof}, valid)

	for name, input := range map[string]string{
		"trailing JSON":          base + `{}`,
		"oversized":              strings.Repeat(" ", maxInputBytes+1),
		"empty":                  "",
		"array":                  `[]`,
		"null":                   `null`,
		"malformed":              `{`,
		"unknown field":          strings.Replace(base, `"schemaVersion":1`, `"private":"`+privateError+`","schemaVersion":1`, 1),
		"duplicate field":        strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1),
		"case variant":           strings.Replace(base, `"schemaVersion":1`, `"SchemaVersion":1`, 1),
		"float schema":           strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":1.0`, 1),
		"exponent schema":        strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":1e0`, 1),
		"string schema":          strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":"1"`, 1),
		"wrong schema":           strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		"duplicate target":       strings.Replace(base, `"target":{`, `"target":{},"target":{`, 1),
		"duplicate nested field": strings.Replace(base, `"channelID":"`+testChannel+`"`, `"channelID":"`+testChannel+`","channelID":"`+testChannel+`"`, 1),
		"read operation":         requestJSON(t, func(v map[string]any) { v["operation"] = "inspect" }),
		"revoke operation":       requestJSON(t, func(v map[string]any) { v["operation"] = "revoke" }),
		"disclosure operation":   requestJSON(t, func(v map[string]any) { v["operation"] = "disclosure" }),
		"wrong channel":          requestJSON(t, func(v map[string]any) { objectField(t, v, "target")["channelID"] = "wrong-channel" }),
		"unknown environment":    requestJSON(t, func(v map[string]any) { objectField(t, v, "target")["environment"] = "staging" }),
		"invalid project":        requestJSON(t, func(v map[string]any) { objectField(t, v, "target")["projectID"] = "invalid_project" }),
		"uppercase ref":          requestJSON(t, func(v map[string]any) { v["requestRef"] = strings.ToUpper(testRequest) }),
		"short ref":              requestJSON(t, func(v map[string]any) { v["proofRef"] = strings.Repeat("a", 63) }),
		"raw ref":                requestJSON(t, func(v map[string]any) { v["executionRef"] = privateError }),
		"scope assertion":        requestJSON(t, func(v map[string]any) { v["scopes"] = []string{"users"} }),
		"proof assertion":        requestJSON(t, func(v map[string]any) { v["proofConfirmed"] = true }),
		"evidence assertion":     requestJSON(t, func(v map[string]any) { v["evidence"] = map[string]any{"drained": true} }),
		"provenance assertion":   requestJSON(t, func(v map[string]any) { v["provenance"] = "live" }),
		"recovery assertion":     requestJSON(t, func(v map[string]any) { v["recoverable"] = true }),
		"skip assertion":         requestJSON(t, func(v map[string]any) { v["skipScopes"] = []string{"backups"} }),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := readRequest(strings.NewReader(input))
			require.ErrorIs(t, err, errInput)
		})
	}

	for _, scope := range []string{"root", "target", "confirmation"} {
		var fields []string
		switch scope {
		case "root":
			fields = []string{"schemaVersion", "target", "operation", "requestRef", "executionRef", "proofRef", "confirmation"}
		case "target":
			fields = []string{"environment", "projectID", "channelID"}
		case "confirmation":
			fields = []string{"environment", "projectID", "channelID", "operation", "requestRef", "executionRef", "proofRef"}
		}
		for _, field := range fields {
			for _, invalid := range []string{"null", "missing", "wrong type", "case variant"} {
				t.Run(scope+"/"+field+"/"+invalid, func(t *testing.T) {
					input := requestJSON(t, func(v map[string]any) {
						object := v
						if scope != "root" {
							object = objectField(t, v, scope)
						}
						switch invalid {
						case "null":
							object[field] = nil
						case "missing":
							delete(object, field)
						case "wrong type":
							object[field] = []any{}
						case "case variant":
							object[strings.ToUpper(field)] = object[field]
							delete(object, field)
						}
					})
					_, err := readRequest(strings.NewReader(input))
					require.ErrorIs(t, err, errInput)
				})
			}
		}
	}
	for field, value := range map[string]string{"environment": "production", "projectID": "synthetic-wrong-project", "channelID": "UCsynthetic0000000000002", "operation": "revoke", "requestRef": strings.Repeat("a", 64), "executionRef": strings.Repeat("b", 64), "proofRef": strings.Repeat("c", 64)} {
		t.Run("confirmation mismatch/"+field, func(t *testing.T) {
			input := requestJSON(t, func(v map[string]any) { objectField(t, v, "confirmation")[field] = value })
			_, err := readRequest(strings.NewReader(input))
			require.ErrorIs(t, err, errInput)
		})
	}
}

func TestInputBoundAppliesIncludingWhitespace(t *testing.T) {
	base := requestJSON(t, nil)
	padded := base + strings.Repeat(" ", maxInputBytes-len(base))
	_, err := readRequest(strings.NewReader(padded))
	require.NoError(t, err)
	_, err = readRequest(strings.NewReader(padded + " "))
	require.ErrorIs(t, err, errInput)
}

func TestOptionsRejectUnboundOrUnsupportedExecutionModes(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--execute"},
		{"--stdin", "--stdin"},
		{"--stdin", "--manifest", privateError},
		{"--manifest"},
		{"--manifest", ""},
		{"--stdin", "--mock", "--execute"},
		{"--stdin", "--check-config", "--mock"},
		{"--stdin", "--mock", "--mock"},
		{"--stdin", "--allow-production"},
		{"--stdin", "--recover"},
		{"--stdin", "--execute", "--recover"},
		{"--stdin", "--check-config", "--recover"},
		{"--stdin", "--mock", "--recover", "--recover"},
		{"--stdin", "--skip", "users"},
		{"--stdin", "--channel-id", testChannel},
		{"--stdin", "--request-ref", testRequest},
		{"--stdin", privateError},
	} {
		_, err := parseOptions(args)
		require.ErrorIs(t, err, errInput)
	}
	for _, args := range [][]string{{"--stdin"}, {"--manifest", privateError}, {"--stdin", "--check-config"}, {"--stdin", "--mock"}, {"--stdin", "--execute"}, {"--stdin", "--mock", "--recover"}} {
		_, err := parseOptions(args)
		require.NoError(t, err)
	}
}

func TestManifestMustBeExact0600RegularNonSymlinkAndBounded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private-target.json")
	require.NoError(t, os.WriteFile(path, []byte(requestJSON(t, nil)), 0o600))
	input, closeInput, err := inputFrom(options{manifest: path}, nil)
	require.NoError(t, err)
	_, err = readRequest(input)
	closeInput()
	require.NoError(t, err)
	for _, mode := range []os.FileMode{0o400, 0o640, 0o644, 0o700, 0o600 | os.ModeSetuid, 0o600 | os.ModeSetgid, 0o600 | os.ModeSticky} {
		require.NoError(t, os.Chmod(path, mode))
		info, err := os.Lstat(path)
		require.NoError(t, err)
		// Some filesystems clear setuid/setgid on chmod. In that case the
		// ordinary 0600 file is not a valid negative fixture.
		if special := mode & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky); special != 0 && info.Mode()&special != special {
			t.Logf("filesystem cleared requested special mode %v", special)
			continue
		}
		_, _, err = inputFrom(options{manifest: path}, nil)
		require.ErrorIs(t, err, errInput)
	}
	require.NoError(t, os.Chmod(path, 0o600))
	link := filepath.Join(dir, "linked-target.json")
	require.NoError(t, os.Symlink(path, link))
	_, _, err = inputFrom(options{manifest: link}, nil)
	require.ErrorIs(t, err, errInput)
	_, _, err = inputFrom(options{manifest: dir}, nil)
	require.ErrorIs(t, err, errInput)
	_, _, err = inputFrom(options{manifest: filepath.Join(dir, privateError)}, nil)
	require.ErrorIs(t, err, errInput)
	fifo := filepath.Join(dir, "private-pipe")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	_, _, err = inputFrom(options{manifest: fifo}, nil)
	require.ErrorIs(t, err, errInput)
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte(" "), maxInputBytes+1), 0o600))
	_, _, err = inputFrom(options{manifest: path}, nil)
	require.ErrorIs(t, err, errInput)
}

type failingIO struct{}

func (failingIO) Read([]byte) (int, error)  { return 0, errors.New(privateError) }
func (failingIO) Write([]byte) (int, error) { return 0, errors.New(privateError) }

func TestInputReaderFailureStaysPrivate(t *testing.T) {
	_, err := readRequest(failingIO{})
	require.ErrorIs(t, err, errInput)
	require.NotContains(t, err.Error(), privateError)
	_, _, err = inputFrom(options{stdin: true}, nil)
	require.ErrorIs(t, err, errInput)
}

func TestConfigurationRejectsAliasesCredentialsAndEndpointOverrides(t *testing.T) {
	for key, value := range map[string]string{
		"MYPAGE_ENVIRONMENT": "staging", "GOOGLE_CLOUD_PROJECT": "invalid_project",
		"GCLOUD_PROJECT": "synthetic-wrong-project", "GCP_PROJECT": "synthetic-wrong-project",
		"FIREBASE_PROJECT_ID": "synthetic-wrong-project", "CLOUDSDK_CORE_PROJECT": "synthetic-wrong-project",
		"GOOGLE_APPLICATION_CREDENTIALS": privateError, "CREDENTIAL_FILE_LOCATION": privateError,
		"FIRESTORE_EMULATOR_HOST": "127.0.0.1:8080", "FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099",
		"STORAGE_EMULATOR_HOST": "127.0.0.1:8081", "BIGQUERY_EMULATOR_HOST": "127.0.0.1:8082",
		"GCE_METADATA_HOST": privateError, "GCE_METADATA_IP": "127.0.0.1",
	} {
		t.Run(key, func(t *testing.T) {
			env := testEnvironment()
			env[key] = value
			_, err := configuredTarget(func(k string) string { return env[k] })
			require.ErrorIs(t, err, errInput)
		})
	}
	for _, environment := range []string{"development", "production"} {
		env := testEnvironment()
		env["MYPAGE_ENVIRONMENT"] = environment
		for _, alias := range []string{"GCLOUD_PROJECT", "GCP_PROJECT", "FIREBASE_PROJECT_ID", "CLOUDSDK_CORE_PROJECT"} {
			env[alias] = testProject
		}
		_, err := configuredTarget(func(k string) string { return env[k] })
		require.NoError(t, err)
	}
	_, err := configuredTarget(nil)
	require.ErrorIs(t, err, errInput)
}

var _ io.ReadWriter = failingIO{}
