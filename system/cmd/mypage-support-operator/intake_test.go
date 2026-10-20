package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
)

func humanArgs(path string, purpose mypage.SupportPurpose) []string {
	return []string{"--action", "create", "--environment", "development", "--project", humanMockProject, "--purpose", string(purpose), "--request-file", path}
}

func humanFile(t *testing.T, input humanRequest) string {
	t.Helper()
	fields := map[string]string{"requestId": input.RequestID, "targetChannel": input.Channel, "acceptedAt": input.AcceptedAt.Format(time.RFC3339Nano)}
	if !input.ExpectedRevision.IsZero() {
		fields["expectedReceiptVersion"] = input.ExpectedRevision.Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(fields)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "human-request.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func privateReport(t *testing.T, output string) report {
	t.Helper()
	var r report
	require.NoError(t, json.Unmarshal([]byte(output), &r))
	for _, private := range []string{humanMockProject, humanMockChannel, strings.Repeat("a", 64), strings.Repeat("b", 64), "2026-", "synthetic-private", "human-request.json"} {
		require.NotContains(t, output, private)
	}
	require.False(t, r.ActualExecution)
	return r
}

type rejectingIntakeTransport struct{ t *testing.T }

func (r rejectingIntakeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.t.Fatal("offline intake must not make an HTTP request")
	return nil, errors.New("synthetic-private")
}

func TestHumanIntakeCLIUsesFixedOfflineMockAndNeverExecutes(t *testing.T) {
	transport := http.DefaultTransport
	http.DefaultTransport = rejectingIntakeTransport{t: t}
	t.Cleanup(func() { http.DefaultTransport = transport })
	for _, purpose := range []mypage.SupportPurpose{mypage.SupportDelete, mypage.SupportDisclosure, mypage.SupportRevoke} {
		t.Run(string(purpose), func(t *testing.T) {
			path := humanFile(t, mockHumanRequest(purpose))
			args := humanArgs(path, purpose)
			for _, mode := range []string{"preview", "mock", "execute"} {
				var out bytes.Buffer
				options := append([]string(nil), args...)
				if mode != "preview" {
					options = append(options, "--"+mode)
				}
				code := command(options, testGet, &out)
				r := privateReport(t, out.String())
				switch mode {
				case "preview":
					require.Zero(t, code)
					require.Equal(t, "READ_ONLY_PREVIEW", r.Code)
					require.True(t, r.ReadOnly)
				case "mock":
					require.Zero(t, code)
					require.Equal(t, "MOCK_AWAITING_PROOF", r.Code)
					require.Equal(t, "mock-only", r.Status)
					require.False(t, r.ReadOnly)
				case "execute":
					require.Equal(t, 1, code)
					require.Equal(t, "TRUSTED_OPERATOR_ADAPTER_UNAVAILABLE", r.Code)
				}
			}
			for _, mode := range []string{"preview", "execute"} {
				var out bytes.Buffer
				options := append([]string(nil), args...)
				if mode == "execute" {
					options = append(options, "--execute")
				}
				commandWithIntake(context.Background(), options, testGet, &out, func(context.Context, proposal) (*intakeRuntime, error) {
					t.Fatal("preview/execute must never reach a factory")
					return nil, nil
				})
			}
		})
	}
}

func TestHumanIntakeCLIRejectsUnsafeInputBeforeFactory(t *testing.T) {
	input := mockHumanRequest(mypage.SupportDelete)
	valid := humanFile(t, input)
	for name, change := range map[string]func([]string) []string{
		"duplicate flag":                func(a []string) []string { return append(a, "--action", "create") },
		"mixed modes":                   func(a []string) []string { return append(a, "--execute") },
		"duplicate mock":                func(a []string) []string { return append(a, "--mock") },
		"create cannot assert proof":    func(a []string) []string { return append(a, "--proof-ref", strings.Repeat("a", 64)) },
		"create cannot assert revision": func(a []string) []string { return append(a, "--expected-revision", "0") },
		"invalid purpose":               func(a []string) []string { a[7] = "login"; return a },
		"production mock":               func(a []string) []string { a[3] = "production"; return a },
		"mock project":                  func(a []string) []string { a[5] = "demo-other-project"; return a },
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			require.Equal(t, 2, commandWithIntake(context.Background(), change(append(humanArgs(valid, mypage.SupportDelete), "--mock")), testGet, &out, func(context.Context, proposal) (*intakeRuntime, error) {
				t.Fatal("invalid input reached factory")
				return nil, nil
			}))
			privateReport(t, out.String())
		})
	}
	data, err := os.ReadFile(valid)
	require.NoError(t, err)
	// Even a syntactically valid, consistently configured target must stay inside
	// the fixed mock fixture, before any injected factory is considered.
	for _, field := range []string{"request", "channel", "accepted", "environment", "project"} {
		t.Run("fixed fixture "+field, func(t *testing.T) {
			changed := input
			environment, projectID := "development", humanMockProject
			switch field {
			case "request":
				changed.RequestID = strings.Repeat("c", 64)
			case "channel":
				changed.Channel = "UCsynthetic0000000000002"
			case "accepted":
				changed.AcceptedAt = changed.AcceptedAt.Add(time.Minute)
			case "environment":
				environment = "production"
			case "project":
				projectID = "demo-other-project"
			}
			args := append(humanArgs(humanFile(t, changed), mypage.SupportDelete), "--mock")
			args[3], args[5] = environment, projectID
			get := func(key string) string {
				if key == "MYPAGE_ENVIRONMENT" {
					return environment
				}
				if key == "GOOGLE_CLOUD_PROJECT" {
					return projectID
				}
				return ""
			}
			var out bytes.Buffer
			require.Equal(t, 2, commandWithIntake(context.Background(), args, get, &out, func(context.Context, proposal) (*intakeRuntime, error) {
				t.Fatal("mock target mismatch reached factory")
				return nil, nil
			}))
			require.Equal(t, "MOCK_TARGET_REQUIRED", privateReport(t, out.String()).Code)
		})
	}
	for name, value := range map[string][]byte{
		"duplicate field": []byte(strings.TrimSuffix(string(data), "}") + `,"requestId":"` + input.RequestID + `"}`),
		"unknown field":   []byte(strings.TrimSuffix(string(data), "}") + `,"body":"synthetic-private"}`),
		"case variant":    bytes.Replace(data, []byte("requestId"), []byte("RequestId"), 1),
		"missing field":   []byte(`{"requestId":"` + input.RequestID + `"}`),
		"null":            []byte("null"), "null value": bytes.Replace(data, []byte(`"`+input.RequestID+`"`), []byte("null"), 1),
		"trailing input":  append(append([]byte(nil), data...), []byte(" []")...),
		"oversized":       bytes.Repeat([]byte(" "), 1025),
		"invalid channel": bytes.Replace(data, []byte(humanMockChannel), []byte("synthetic-private"), 1),
		"zero time":       bytes.Replace(data, []byte("2026-10-06T00:00:00Z"), []byte("0001-01-01T00:00:00Z"), 1),
		"future time":     bytes.Replace(data, []byte("2026-10-06T00:00:00Z"), []byte(time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)), 1),
		"nanosecond loss": bytes.Replace(data, []byte("2026-10-06T00:00:00Z"), []byte("2026-10-06T00:00:00.000000001Z"), 1),
		"non UTC":         bytes.Replace(data, []byte("2026-10-06T00:00:00Z"), []byte("2026-10-06T09:00:00+09:00"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.json")
			require.NoError(t, os.WriteFile(path, value, 0o600))
			var out bytes.Buffer
			require.Equal(t, 2, commandWithIntake(context.Background(), append(humanArgs(path, mypage.SupportDelete), "--mock"), testGet, &out, func(context.Context, proposal) (*intakeRuntime, error) {
				t.Fatal("invalid input reached factory")
				return nil, nil
			}))
			privateReport(t, out.String())
		})
	}
	for _, key := range []string{"GCLOUD_PROJECT", "GCP_PROJECT", "FIREBASE_PROJECT_ID", "CLOUDSDK_CORE_PROJECT", "GOOGLE_APPLICATION_CREDENTIALS", "FIRESTORE_EMULATOR_HOST", "GCE_METADATA_HOST"} {
		t.Run(key, func(t *testing.T) {
			var out bytes.Buffer
			get := func(k string) string {
				if k == key {
					return "synthetic-private"
				}
				return testGet(k)
			}
			require.Equal(t, 2, commandWithIntake(context.Background(), append(humanArgs(valid, mypage.SupportDelete), "--mock"), get, &out, func(context.Context, proposal) (*intakeRuntime, error) {
				t.Fatal("unsafe configuration reached factory")
				return nil, nil
			}))
			privateReport(t, out.String())
		})
	}
	for _, mode := range []os.FileMode{0o644, 0o400} {
		require.NoError(t, os.Chmod(valid, mode))
		var out bytes.Buffer
		require.Equal(t, 2, command(humanArgs(valid, mypage.SupportDelete), testGet, &out))
	}
	require.NoError(t, os.Chmod(valid, 0o600))
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(valid, link))
	var out bytes.Buffer
	require.Equal(t, 2, command(humanArgs(link, mypage.SupportDelete), testGet, &out))
}

type createFunc func(context.Context, string, string, mypage.SupportPurpose, time.Time, time.Time) (string, string, error)

func TestHumanIntakeAssemblyDeniesMissingOrUnboundClientWithoutIO(t *testing.T) {
	p := proposal{Action: "create", Environment: "development", Project: humanMockProject, Purpose: mypage.SupportDelete}
	authorize := func(context.Context, intakeIntent) (intakeGrant, error) {
		t.Fatal("unbound client must be rejected before authority")
		return intakeGrant{}, nil
	}
	deliver := func(context.Context, intakeIntent, intakeReceipt) error {
		t.Fatal("unbound client must never receive a challenge")
		return nil
	}
	for _, client := range []*firestore.Client{nil, {}} {
		runtime, err := newFirestoreIntakeRuntime(p, client, authorize, deliver, time.Now)
		require.ErrorIs(t, err, errIntake)
		require.Nil(t, runtime)
	}
}

func (f createFunc) Create(ctx context.Context, id, channel string, purpose mypage.SupportPurpose, accepted, now time.Time) (string, string, error) {
	return f(ctx, id, channel, purpose, accepted, now)
}

func TestHumanIntakeAuthorityBindingAndUncertainResultsFailClosed(t *testing.T) {
	p := proposal{Action: "create", Environment: "development", Project: humanMockProject, Purpose: mypage.SupportDelete, Mock: true}
	input := mockHumanRequest(p.Purpose)
	for name, change := range map[string]func(*intakeRuntime){
		"no authority":        func(r *intakeRuntime) { r.authorize = nil },
		"no private delivery": func(r *intakeRuntime) { r.deliver = nil },
		"no creator":          func(r *intakeRuntime) { r.creator = nil },
		"no clock":            func(r *intakeRuntime) { r.clock = nil },
		"wrong environment":   func(r *intakeRuntime) { r.environment = "production" },
		"wrong project":       func(r *intakeRuntime) { r.project = "demo-other-project" },
		"zero clock":          func(r *intakeRuntime) { r.clock = func() time.Time { return time.Time{} } },
		"future receipt":      func(r *intakeRuntime) { r.clock = func() time.Time { return input.AcceptedAt.Add(-time.Hour) } },
		"authority denied": func(r *intakeRuntime) {
			r.authorize = func(context.Context, intakeIntent) (intakeGrant, error) {
				return intakeGrant{}, errors.New("synthetic-private")
			}
		},
		"changed bound receipt": func(r *intakeRuntime) {
			r.authorize = func(_ context.Context, i intakeIntent) (intakeGrant, error) {
				i.Request.Channel = "UCsynthetic0000000000002"
				return intakeGrant{Actor: "synthetic-operator", Intent: i}, nil
			}
		},
		"raw actor": func(r *intakeRuntime) {
			r.authorize = func(_ context.Context, i intakeIntent) (intakeGrant, error) {
				return intakeGrant{Actor: "synthetic@example.invalid", Intent: i}, nil
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := mockIntakeFactory(context.Background(), p)
			require.NoError(t, err)
			r.creator = createFunc(func(context.Context, string, string, mypage.SupportPurpose, time.Time, time.Time) (string, string, error) {
				t.Fatal("denial reached Create")
				return "", "", nil
			})
			change(r)
			code := runHumanIntake(context.Background(), p, input, func(context.Context, proposal) (*intakeRuntime, error) { return r, nil })
			require.NotEqual(t, "MOCK_AWAITING_PROOF", code)
			require.NotContains(t, code, "synthetic-private")
		})
	}
	for _, stage := range []string{"create ACK loss", "delivery failure", "invalid result", "success"} {
		t.Run(stage, func(t *testing.T) {
			r, err := mockIntakeFactory(context.Background(), p)
			require.NoError(t, err)
			calls, deliveries := 0, 0
			r.creator = createFunc(func(_ context.Context, id, channel string, purpose mypage.SupportPurpose, accepted, now time.Time) (string, string, error) {
				calls++
				require.Equal(t, input.RequestID, id)
				require.Equal(t, input.Channel, channel)
				require.Equal(t, p.Purpose, purpose)
				require.Equal(t, input.AcceptedAt, accepted)
				require.Equal(t, input.AcceptedAt.Add(time.Hour), now)
				if stage == "create ACK loss" {
					return "", "", errors.New("synthetic-private")
				}
				if stage == "invalid result" {
					return "synthetic-private", "", nil
				}
				return strings.Repeat("a", 64), strings.Repeat("b", 64), nil
			})
			r.deliver = func(_ context.Context, i intakeIntent, receipt intakeReceipt) error {
				deliveries++
				require.Equal(t, input, i.Request)
				require.Len(t, receipt.Challenge, 64)
				if stage == "delivery failure" {
					return errors.New("synthetic-private")
				}
				return nil
			}
			code := runHumanIntake(context.Background(), p, input, func(context.Context, proposal) (*intakeRuntime, error) { return r, nil })
			require.Equal(t, 1, calls, "never retry Create after uncertain acknowledgement")
			switch stage {
			case "create ACK loss", "invalid result":
				require.Equal(t, "INTAKE_RESULT_UNCERTAIN", code)
				require.Zero(t, deliveries)
			case "delivery failure":
				require.Equal(t, "INTAKE_HANDOFF_PENDING", code)
				require.Equal(t, 1, deliveries)
			case "success":
				require.Equal(t, "MOCK_AWAITING_PROOF", code)
				require.Equal(t, 1, deliveries)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Equal(t, "TRUSTED_INTAKE_UNAVAILABLE", runHumanIntake(ctx, p, input, func(context.Context, proposal) (*intakeRuntime, error) {
		t.Fatal("cancelled request reached factory")
		return nil, nil
	}))
	require.Equal(t, "TRUSTED_INTAKE_UNAVAILABLE", runHumanIntake(context.Background(), p, input, nil))
}
