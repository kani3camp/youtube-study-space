package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app.modules/core/serviceaccess"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	testProject  = "synthetic-operator-project"
	testChannel  = "UCsynthetic0000000000001"
	testRef      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	privateError = "private-SDK-detail-channel-and-credential"
)

var testNow = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

func testEnvironment() map[string]string {
	return map[string]string{"MYPAGE_ENVIRONMENT": "development", "GOOGLE_CLOUD_PROJECT": testProject}
}

func requestJSON(t *testing.T, operation string, edit func(map[string]any)) string {
	t.Helper()
	reason := ""
	if operation == "block" {
		reason = "MODERATION"
	}
	v := map[string]any{
		"schemaVersion": 1,
		"target":        map[string]any{"environment": "development", "projectID": testProject, "channelID": testChannel},
		"operation":     operation, "actionRef": testRef, "reasonCode": reason,
		"confirmation": map[string]any{"environment": "development", "projectID": testProject, "channelID": testChannel, "operation": operation, "actionRef": testRef},
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
	require.True(t, ok, "synthetic object field must be an object")
	return field
}

type fakeStore struct {
	value   serviceaccess.Snapshot
	err     error
	events  []string
	changes []serviceaccess.Change
	reads   int
}

func (f *fakeStore) Read(_ context.Context, channel string) (serviceaccess.Snapshot, error) {
	f.reads++
	if channel != testChannel {
		return serviceaccess.Snapshot{}, errors.New(privateError)
	}
	return f.value, f.err
}

func (f *fakeStore) Change(_ context.Context, channel string, change serviceaccess.Change, now time.Time) (serviceaccess.Snapshot, error) {
	f.events = append(f.events, "commit")
	f.changes = append(f.changes, change)
	if f.err != nil || channel != testChannel {
		return serviceaccess.Snapshot{}, errors.New(privateError)
	}
	control, changed, err := serviceaccess.Transition(f.value, change, now)
	if err != nil {
		return serviceaccess.Snapshot{}, fmt.Errorf("synthetic transition: %w", err)
	}
	if changed {
		f.value = serviceaccess.Snapshot{Control: control, Exists: true, Revision: now.Add(time.Duration(control.Generation) * time.Microsecond)}
	}
	return f.value, nil
}

func runTest(t *testing.T, args []string, env map[string]string, data string, start bootstrap) (int, report, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := command(context.Background(), args, func(k string) string { return env[k] }, strings.NewReader(data), &stdout, &stderr, start, func() time.Time { return testNow })
	var value report
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &value), stdout.String())
	combined := stdout.String() + stderr.String()
	for _, private := range []string{testProject, testChannel, testRef, privateError} {
		require.NotContains(t, combined, private)
	}
	return code, value, combined
}

func TestOfflineModesNeverBootstrapOrReadControl(t *testing.T) {
	for _, operation := range []string{"block", "unblock", "inspect"} {
		for _, mode := range []string{"plan", "check-config"} {
			t.Run(operation+"/"+mode, func(t *testing.T) {
				args := []string{"--stdin"}
				if mode == "check-config" {
					args = append(args, "--check-config")
				}
				calls := 0
				code, out, _ := runTest(t, args, testEnvironment(), requestJSON(t, operation, nil), func(context.Context, target) (ports, error) {
					calls++
					return ports{}, errors.New(privateError)
				})
				require.Zero(t, code)
				require.Zero(t, calls)
				require.True(t, out.Offline)
				require.Equal(t, "unknown", out.Control)
				require.Equal(t, "not-run", out.Revoke)
			})
		}
	}
}

func TestRefusalBeforeBootstrap(t *testing.T) {
	base := requestJSON(t, "block", nil)
	for name, input := range map[string]string{
		"trailing JSON":                  base + `{}`,
		"oversized":                      strings.Repeat(" ", maxInputBytes+1),
		"unknown field":                  strings.Replace(base, `"schemaVersion":1`, `"secret":"`+privateError+`","schemaVersion":1`, 1),
		"duplicate field":                strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1),
		"case variant":                   strings.Replace(base, `"schemaVersion":1`, `"SchemaVersion":1`, 1),
		"duplicate target":               strings.Replace(base, `"target":{`, `"target":{},"target":{`, 1),
		"duplicate nested field":         strings.Replace(base, `"channelID":"`+testChannel+`"`, `"channelID":"`+testChannel+`","channelID":"`+testChannel+`"`, 1),
		"null reason":                    requestJSON(t, "unblock", func(v map[string]any) { v["reasonCode"] = nil }),
		"float schema":                   strings.Replace(base, `"schemaVersion":1`, `"schemaVersion":1.0`, 1),
		"invalid channel":                requestJSON(t, "block", func(v map[string]any) { objectField(t, v, "target")["channelID"] = "bad-channel" }),
		"wrong channel confirmation":     requestJSON(t, "block", func(v map[string]any) { objectField(t, v, "confirmation")["channelID"] = "UCsynthetic0000000000002" }),
		"wrong project confirmation":     requestJSON(t, "block", func(v map[string]any) { objectField(t, v, "confirmation")["projectID"] = "synthetic-wrong-project" }),
		"wrong environment confirmation": requestJSON(t, "block", func(v map[string]any) { objectField(t, v, "confirmation")["environment"] = "production" }),
		"wrong action confirmation":      requestJSON(t, "block", func(v map[string]any) { objectField(t, v, "confirmation")["operation"] = "unblock" }),
		"wrong ref confirmation":         requestJSON(t, "block", func(v map[string]any) { objectField(t, v, "confirmation")["actionRef"] = strings.Repeat("f", 64) }),
		"nonopaque ref":                  requestJSON(t, "block", func(v map[string]any) { v["actionRef"] = privateError }),
		"uppercase ref":                  requestJSON(t, "block", func(v map[string]any) { v["actionRef"] = strings.ToUpper(testRef) }),
		"free text reason":               requestJSON(t, "block", func(v map[string]any) { v["reasonCode"] = privateError }),
		"privacy mutation":               requestJSON(t, "privacyDeletion", nil),
		"unblock cannot persist code":    requestJSON(t, "unblock", func(v map[string]any) { v["reasonCode"] = "MODERATION" }),
		"inspect cannot persist code":    requestJSON(t, "inspect", func(v map[string]any) { v["reasonCode"] = "MODERATION" }),
		"missing ref":                    requestJSON(t, "block", func(v map[string]any) { delete(v, "actionRef") }),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), input, func(context.Context, target) (ports, error) {
				calls++
				return ports{}, nil
			})
			require.Equal(t, 2, code)
			require.Equal(t, "INVALID_INPUT", out.Code)
			require.Zero(t, calls)
		})
	}
}

func TestWrongConfiguredTargetAndUnsafeSDKConfigurationRefusedBeforeBootstrap(t *testing.T) {
	for _, edit := range []map[string]string{
		{"MYPAGE_ENVIRONMENT": "production"},
		{"GOOGLE_CLOUD_PROJECT": "synthetic-wrong-project"},
		{"GCLOUD_PROJECT": "synthetic-wrong-project"},
		{"GCP_PROJECT": "synthetic-wrong-project"},
		{"FIREBASE_PROJECT_ID": "synthetic-wrong-project"},
		{"GOOGLE_APPLICATION_CREDENTIALS": privateError},
		{"CREDENTIAL_FILE_LOCATION": privateError},
		{"FIRESTORE_EMULATOR_HOST": "127.0.0.1:8080"},
		{"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099"},
	} {
		t.Run(fmt.Sprint(edit), func(t *testing.T) {
			env := testEnvironment()
			for k, v := range edit {
				env[k] = v
			}
			calls := 0
			code, out, _ := runTest(t, []string{"--stdin", "--execute"}, env, requestJSON(t, "block", nil), func(context.Context, target) (ports, error) {
				calls++
				return ports{}, nil
			})
			require.Equal(t, 2, code)
			require.Contains(t, []string{"CONFIGURATION_INVALID", "TARGET_MISMATCH"}, out.Code)
			require.Zero(t, calls)
		})
	}
}

func TestInvalidOptionsRefusedWithoutInputOrBootstrap(t *testing.T) {
	for _, args := range [][]string{nil, {"--execute"}, {"--stdin", "--stdin"}, {"--stdin", "--manifest", privateError}, {"--manifest"}, {"--stdin", "--execute", "--check-config"}, {"--stdin", "--execute", "--execute"}, {"--stdin", "--allow-production"}, {"--stdin", privateError}} {
		code, out, output := runTest(t, args, testEnvironment(), privateError, func(context.Context, target) (ports, error) {
			t.Fatal("invalid options reached bootstrap")
			return ports{}, nil
		})
		require.Equal(t, 2, code)
		require.Equal(t, "INVALID_ARGUMENTS", out.Code)
		require.NotContains(t, output, privateError)
	}
}

func TestProductionExecutionRequiresMatchingExplicitAcknowledgement(t *testing.T) {
	env := testEnvironment()
	env["MYPAGE_ENVIRONMENT"] = "production"
	input := requestJSON(t, "inspect", func(v map[string]any) {
		objectField(t, v, "target")["environment"] = "production"
		objectField(t, v, "confirmation")["environment"] = "production"
	})
	calls := 0
	start := func(context.Context, target) (ports, error) {
		calls++
		return ports{Store: &fakeStore{}}, nil
	}
	code, out, _ := runTest(t, []string{"--stdin", "--execute"}, env, input, start)
	require.Equal(t, 2, code)
	require.Equal(t, "PRODUCTION_ACK_MISMATCH", out.Code)
	require.Zero(t, calls)
	code, _, _ = runTest(t, []string{"--stdin", "--execute", "--allow-production"}, env, input, start)
	require.Zero(t, code)
	require.Equal(t, 1, calls)
	code, out, _ = runTest(t, []string{"--stdin", "--execute", "--allow-production"}, testEnvironment(), requestJSON(t, "inspect", nil), start)
	require.Equal(t, 2, code)
	require.Equal(t, "PRODUCTION_ACK_MISMATCH", out.Code)
	require.Equal(t, 1, calls)
}

func TestBlockCommitsBeforeRevokeAndRetryIsIdempotent(t *testing.T) {
	store := &fakeStore{}
	closed := 0
	start := func(_ context.Context, selected target) (ports, error) {
		require.Equal(t, target{"development", testProject, testChannel}, selected)
		return ports{Store: store, Close: func() error { closed++; return nil }, Revoke: func(_ context.Context, channel string) error {
			require.Equal(t, testChannel, channel)
			require.True(t, store.value.Control.Moderation.Active, "revoke preceded guard commit")
			require.Equal(t, testRef, store.value.Control.Moderation.ActionRef)
			store.events = append(store.events, "revoke")
			return nil
		}}, nil
	}
	for i := 0; i < 2; i++ {
		code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "block", nil), start)
		require.Zero(t, code)
		require.Equal(t, "active", out.Moderation)
		require.Equal(t, "succeeded", out.Revoke)
		require.Equal(t, int64(1), out.Generation)
	}
	require.Equal(t, []string{"commit", "revoke", "commit", "revoke"}, store.events)
	require.Equal(t, 2, closed)
}

func TestFailedRevokeRetainsGuardAndReportsPartialFailure(t *testing.T) {
	store := &fakeStore{}
	code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "block", nil), func(context.Context, target) (ports, error) {
		return ports{Store: store, Revoke: func(context.Context, string) error { return errors.New(privateError) }}, nil
	})
	require.Equal(t, 3, code)
	require.Equal(t, "partial-failure", out.Status)
	require.Equal(t, "REVOKE_UNAVAILABLE", out.Code)
	require.Equal(t, "active", out.Moderation)
	require.Equal(t, "failed", out.Revoke)
	require.True(t, store.value.Control.Moderation.Active)
	require.Len(t, store.changes, 1, "revoke failure must not roll back the committed guard")
}

type responseStore struct{ value serviceaccess.Snapshot }

func (s responseStore) Read(context.Context, string) (serviceaccess.Snapshot, error) {
	return s.value, nil
}

func (s responseStore) Change(context.Context, string, serviceaccess.Change, time.Time) (serviceaccess.Snapshot, error) {
	return s.value, nil
}

func TestBlockDoesNotRevokeWithoutMatchingCommittedGuard(t *testing.T) {
	control, _, err := serviceaccess.Transition(serviceaccess.Snapshot{}, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "MODERATION", Reference: strings.Repeat("f", 64)}, testNow)
	require.NoError(t, err)
	for name, value := range map[string]serviceaccess.Snapshot{
		"no committed record": {},
		"malformed record":    {Exists: true},
		"different action":    {Control: control, Exists: true, Revision: testNow},
	} {
		t.Run(name, func(t *testing.T) {
			code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "block", nil), func(context.Context, target) (ports, error) {
				return ports{Store: responseStore{value}, Revoke: func(context.Context, string) error {
					t.Fatal("a mismatching guard reached revoke")
					return nil
				}}, nil
			})
			require.Equal(t, 1, code)
			require.Contains(t, []string{"CONTROL_CONFLICT", "CONTROL_UNAVAILABLE"}, out.Code)
			require.Equal(t, "unknown", out.Moderation)
			require.Equal(t, "not-run", out.Revoke)
		})
	}
}

func TestMissingRevokePortStillPersistsGuard(t *testing.T) {
	store := &fakeStore{}
	code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "block", nil), func(context.Context, target) (ports, error) { return ports{Store: store}, nil })
	require.Equal(t, 3, code)
	require.Equal(t, "partial-failure", out.Status)
	require.Equal(t, "active", out.Moderation)
	require.True(t, store.value.Control.Moderation.Active)
}

func TestFixedModerationReasonCodesAreSupported(t *testing.T) {
	for _, reason := range []string{"MODERATION", "SECURITY", "POLICY_VIOLATION", "LEGACY_COMPATIBILITY"} {
		t.Run(reason, func(t *testing.T) {
			store := &fakeStore{}
			input := requestJSON(t, "block", func(v map[string]any) { v["reasonCode"] = reason })
			code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), input, func(context.Context, target) (ports, error) {
				return ports{Store: store, Revoke: func(context.Context, string) error { return nil }}, nil
			})
			require.Zero(t, code)
			require.Equal(t, reason, out.ReasonCode)
			require.Equal(t, reason, store.value.Control.Moderation.ReasonCode)
		})
	}
}

func TestStoreFailureNeverRevokesAndCannotClaimRestriction(t *testing.T) {
	code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "block", nil), func(context.Context, target) (ports, error) {
		return ports{Store: &fakeStore{err: errors.New(privateError)}, Revoke: func(context.Context, string) error { t.Fatal("failed commit reached revoke"); return nil }}, nil
	})
	require.Equal(t, 1, code)
	require.Equal(t, "CONTROL_UNAVAILABLE", out.Code)
	require.Equal(t, "unknown", out.Moderation)
	require.Equal(t, "not-run", out.Revoke)
}

func TestUnblockPreservesPrivacyDeletionAndDoesNotRevoke(t *testing.T) {
	control, _, err := serviceaccess.Transition(serviceaccess.Snapshot{}, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: true, ReasonCode: "PRIVACY_DELETION", Reference: strings.Repeat("a", 64)}, testNow.Add(-time.Hour))
	require.NoError(t, err)
	store := &fakeStore{value: serviceaccess.Snapshot{Control: control, Exists: true, Revision: testNow.Add(-time.Hour)}}
	_, err = store.Change(context.Background(), testChannel, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "SECURITY", Reference: testRef}, testNow.Add(-time.Minute))
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "unblock", nil), func(context.Context, target) (ports, error) {
			return ports{Store: store, Revoke: func(context.Context, string) error { t.Fatal("unblock reached revoke"); return nil }}, nil
		})
		require.Zero(t, code)
		require.Equal(t, "inactive", out.Moderation)
		require.Equal(t, "active", out.PrivacyDeletion)
		require.Equal(t, "deletion", out.Control)
		require.Equal(t, int64(3), out.Generation)
	}
	require.Equal(t, "PRIVACY_DELETION", store.value.Control.PrivacyDeletion.ReasonCode)
	require.Equal(t, strings.Repeat("a", 64), store.value.Control.PrivacyDeletion.RequestRef)
	require.Equal(t, serviceaccess.Change{Reason: serviceaccess.Moderation}, store.changes[len(store.changes)-1])
}

func TestInspectUnavailableNeverReportsAllowed(t *testing.T) {
	for name, store := range map[string]*fakeStore{
		"SDK failure":   {err: errors.New(privateError)},
		"invalid state": {value: serviceaccess.Snapshot{Exists: true}},
	} {
		t.Run(name, func(t *testing.T) {
			code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "inspect", nil), func(context.Context, target) (ports, error) { return ports{Store: store}, nil })
			require.Equal(t, 1, code)
			require.Equal(t, "unavailable", out.Status)
			require.Equal(t, "unknown", out.Control)
			require.Equal(t, 1, store.reads)
			require.Empty(t, store.changes)
		})
	}
	store := &fakeStore{}
	code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "inspect", nil), func(context.Context, target) (ports, error) { return ports{Store: store}, nil })
	require.Zero(t, code)
	require.Equal(t, "absent", out.Control)
	require.Equal(t, "inactive", out.Moderation)
	require.Empty(t, store.changes)
}

func TestBootstrapFailureIsSanitized(t *testing.T) {
	code, out, _ := runTest(t, []string{"--stdin", "--execute"}, testEnvironment(), requestJSON(t, "inspect", nil), func(context.Context, target) (ports, error) { return ports{}, errors.New(privateError) })
	require.Equal(t, 1, code)
	require.Equal(t, "BOOTSTRAP_UNAVAILABLE", out.Code)
}

func TestManifestMustBePrivateRegularAndBounded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private-target.json")
	require.NoError(t, os.WriteFile(path, []byte(requestJSON(t, "inspect", nil)), 0o600))
	code, out, output := runTest(t, []string{"--manifest", path}, testEnvironment(), "", nil)
	require.Zero(t, code)
	require.Equal(t, "PLAN_ONLY", out.Code)
	require.NotContains(t, output, path)
	require.NoError(t, os.Chmod(path, 0o644))
	code, out, output = runTest(t, []string{"--manifest", path}, testEnvironment(), "", nil)
	require.Equal(t, 2, code)
	require.Equal(t, "INVALID_INPUT", out.Code)
	require.NotContains(t, output, path)
	require.NoError(t, os.Chmod(path, 0o600))
	link := filepath.Join(dir, "linked-target.json")
	require.NoError(t, os.Symlink(path, link))
	code, out, _ = runTest(t, []string{"--manifest", link}, testEnvironment(), "", nil)
	require.Equal(t, 2, code)
	require.Equal(t, "INVALID_INPUT", out.Code)
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte(" "), maxInputBytes+1), 0o600))
	code, out, _ = runTest(t, []string{"--manifest", path}, testEnvironment(), "", nil)
	require.Equal(t, 2, code)
	require.Equal(t, "INVALID_INPUT", out.Code)
}

type failingIO struct{}

func (failingIO) Read([]byte) (int, error)  { return 0, errors.New(privateError) }
func (failingIO) Write([]byte) (int, error) { return 0, errors.New(privateError) }

func TestInputAndOutputFailuresDoNotExposeRawErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := command(context.Background(), []string{"--stdin"}, func(k string) string { return testEnvironment()[k] }, failingIO{}, &stdout, &stderr, nil, func() time.Time { return testNow })
	require.Equal(t, 2, code)
	require.NotContains(t, stdout.String()+stderr.String(), privateError)
	stdout.Reset()
	code = command(context.Background(), []string{"--stdin"}, func(k string) string { return testEnvironment()[k] }, strings.NewReader(requestJSON(t, "inspect", nil)), failingIO{}, &stderr, nil, func() time.Time { return testNow })
	require.Equal(t, 1, code)
	require.Equal(t, "Service access control report unavailable\n", stderr.String())
}

func TestKeylessCredentialsRejectFilesKeysAndWrongProjectsWithoutTokenCalls(t *testing.T) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: privateError})
	for _, credentials := range []*google.Credentials{nil, {}, {TokenSource: tokenSource, JSON: []byte(`{"private_key":"private"}`)}, {TokenSource: tokenSource, ProjectID: "synthetic-wrong-project"}} {
		require.False(t, keylessCredentialsValid(credentials, testProject))
	}
	require.True(t, keylessCredentialsValid(&google.Credentials{TokenSource: tokenSource, ProjectID: testProject}, testProject))
	require.True(t, keylessCredentialsValid(&google.Credentials{TokenSource: tokenSource}, testProject))
}

var _ io.Reader = failingIO{}
