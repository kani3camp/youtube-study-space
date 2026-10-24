package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"app.modules/core/supportdelete"

	"github.com/stretchr/testify/require"
)

func completedMock() supportdelete.Outcome {
	return supportdelete.Outcome{Code: "MOCK_COMPLETED", Stage: "completed", Mode: "mock", Offline: true, ActualExecution: false}
}

func runTest(t *testing.T, args []string, env map[string]string, data string, run mockRun) (int, report, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := command(context.Background(), args, func(k string) string { return env[k] }, strings.NewReader(data), &stdout, &stderr, run)
	var value report
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &value), stdout.String())
	combined := stdout.String() + stderr.String()
	for _, private := range []string{testProject, testChannel, testRequest, testExecute, testProof, privateError} {
		require.NotContains(t, combined, private)
	}
	require.True(t, value.Offline)
	require.False(t, value.ActualExecution)
	return code, value, combined
}

func TestOfflineModesDoNotInvokeRunnerAndCannotEstablishProofOrDeletion(t *testing.T) {
	for _, mode := range []string{"plan", "check-config"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"--stdin"}
			if mode == "check-config" {
				args = append(args, "--check-config")
			}
			// Format-valid selectors are accepted without pretending they resolve
			// to trusted support records or independently verified proof.
			input := requestJSON(t, func(v map[string]any) {
				v["proofRef"] = strings.Repeat("a", 64)
				objectField(t, v, "confirmation")["proofRef"] = strings.Repeat("a", 64)
			})
			code, out, _ := runTest(t, args, testEnvironment(), input, func(context.Context, request, bool) (supportdelete.Outcome, error) {
				t.Fatal("offline validation invoked the workflow runner")
				return supportdelete.Outcome{}, nil
			})
			require.Zero(t, code)
			require.Equal(t, mode, out.Mode)
			require.Equal(t, "offline", out.Stage)
			require.Contains(t, []string{"PLAN_ONLY", "CONFIGURATION_VALID"}, out.Code)
		})
	}
}

func TestLiveExecutionAlwaysRefusesWithoutInvokingAnyRunner(t *testing.T) {
	for _, environment := range []string{"development", "production"} {
		t.Run(environment, func(t *testing.T) {
			env := testEnvironment()
			env["MYPAGE_ENVIRONMENT"] = environment
			input := requestJSON(t, func(v map[string]any) {
				objectField(t, v, "target")["environment"] = environment
				objectField(t, v, "confirmation")["environment"] = environment
			})
			code, out, _ := runTest(t, []string{"--stdin", "--execute"}, env, input, func(context.Context, request, bool) (supportdelete.Outcome, error) {
				t.Fatal("live execution invoked a mock or bootstrap")
				return completedMock(), nil
			})
			require.Equal(t, 1, code)
			require.Equal(t, "LIVE_ADAPTERS_UNAVAILABLE", out.Code)
			require.Equal(t, "blocked", out.Stage)
			require.Equal(t, "execute", out.Mode)
		})
	}
}

func TestMockRequiresFixedSyntheticTargetBeforeRunner(t *testing.T) {
	for field, value := range map[string]string{"environment": "production", "projectID": "synthetic-wrong-project", "channelID": "UCsynthetic0000000000002"} {
		t.Run(field, func(t *testing.T) {
			env := testEnvironment()
			if field == "environment" {
				env["MYPAGE_ENVIRONMENT"] = value
			}
			if field == "projectID" {
				env["GOOGLE_CLOUD_PROJECT"] = value
			}
			input := requestJSON(t, func(v map[string]any) {
				objectField(t, v, "target")[field] = value
				objectField(t, v, "confirmation")[field] = value
			})
			code, out, _ := runTest(t, []string{"--stdin", "--mock"}, env, input, func(context.Context, request, bool) (supportdelete.Outcome, error) {
				t.Fatal("non-synthetic mock target reached runner")
				return completedMock(), nil
			})
			require.Equal(t, 2, code)
			require.Equal(t, "MOCK_TARGET_REQUIRED", out.Code)
		})
	}
}

func TestConfiguredTargetMismatchCannotInvokeRunner(t *testing.T) {
	for key, value := range map[string]string{"MYPAGE_ENVIRONMENT": "production", "GOOGLE_CLOUD_PROJECT": "synthetic-wrong-project"} {
		t.Run(key, func(t *testing.T) {
			env := testEnvironment()
			env[key] = value
			code, out, _ := runTest(t, []string{"--stdin", "--mock"}, env, requestJSON(t, nil), func(context.Context, request, bool) (supportdelete.Outcome, error) {
				t.Fatal("configuration target mismatch reached runner")
				return completedMock(), nil
			})
			require.Equal(t, 2, code)
			require.Equal(t, "TARGET_MISMATCH", out.Code)
		})
	}
}

func TestInvalidInputsAndArgumentsCannotInvokeRunnerOrExposePrivateValues(t *testing.T) {
	for _, data := range []string{privateError, requestJSON(t, func(v map[string]any) { v["provenance"] = "live" }), requestJSON(t, func(v map[string]any) { v["skipScopes"] = []string{"backups"} })} {
		code, out, _ := runTest(t, []string{"--stdin", "--mock"}, testEnvironment(), data, func(context.Context, request, bool) (supportdelete.Outcome, error) {
			t.Fatal("invalid private input reached runner")
			return completedMock(), nil
		})
		require.Equal(t, 2, code)
		require.Equal(t, "INVALID_INPUT", out.Code)
	}
	code, out, _ := runTest(t, []string{"--stdin", privateError}, testEnvironment(), privateError, nil)
	require.Equal(t, 2, code)
	require.Equal(t, "INVALID_ARGUMENTS", out.Code)
	env := testEnvironment()
	env["GOOGLE_APPLICATION_CREDENTIALS"] = privateError
	code, out, _ = runTest(t, []string{"--stdin", "--mock"}, env, requestJSON(t, nil), func(context.Context, request, bool) (supportdelete.Outcome, error) {
		t.Fatal("unsafe SDK configuration reached runner")
		return completedMock(), nil
	})
	require.Equal(t, 2, code)
	require.Equal(t, "CONFIGURATION_INVALID", out.Code)
}

func TestMockRunnerReceivesOnlyBoundSelectorAndRecoveryIntent(t *testing.T) {
	for _, recover := range []bool{false, true} {
		args := []string{"--stdin", "--mock"}
		if recover {
			args = append(args, "--recover")
		}
		calls := 0
		code, out, _ := runTest(t, args, testEnvironment(), requestJSON(t, nil), func(_ context.Context, selector request, recovery bool) (supportdelete.Outcome, error) {
			calls++
			require.Equal(t, request{Target: target{Environment: "development", ProjectID: testProject, ChannelID: testChannel}, RequestRef: testRequest, ExecutionRef: testExecute, ProofRef: testProof}, selector)
			require.Equal(t, recover, recovery)
			return completedMock(), nil
		})
		require.Zero(t, code)
		require.Equal(t, 1, calls)
		require.Equal(t, "mock-only", out.Status)
		require.Equal(t, "MOCK_COMPLETED", out.Code)
		require.Equal(t, "mock", out.Mode)
	}
}

func TestMockCannotClaimActualExecutionOrForwardUnknownOutcome(t *testing.T) {
	for name, result := range map[string]supportdelete.Outcome{
		"actual execution":  {Code: "MOCK_COMPLETED", Stage: "completed", Mode: "mock", Offline: true, ActualExecution: true},
		"network execution": {Code: "MOCK_COMPLETED", Stage: "completed", Mode: "mock", Offline: false},
		"live provenance":   {Code: "MOCK_COMPLETED", Stage: "completed", Mode: "live", Offline: true},
		"unknown code":      {Code: privateError, Stage: "completed", Mode: "mock", Offline: true},
		"unknown stage":     {Code: "MOCK_COMPLETED", Stage: privateError, Mode: "mock", Offline: true},
		"unproven state":    {Code: "MOCK_COMPLETED", Stage: "unknown", Mode: "mock", Offline: true},
		"empty outcome":     {},
	} {
		t.Run(name, func(t *testing.T) {
			code, out, _ := runTest(t, []string{"--stdin", "--mock"}, testEnvironment(), requestJSON(t, nil), func(context.Context, request, bool) (supportdelete.Outcome, error) {
				return result, nil
			})
			require.Equal(t, 1, code)
			require.Equal(t, "MOCK_RESULT_INVALID", out.Code)
			require.Equal(t, "blocked", out.Stage)
		})
	}
}

func TestRunnerFailureKeepsAllDependencyDetailsPrivate(t *testing.T) {
	code, out, _ := runTest(t, []string{"--stdin", "--mock"}, testEnvironment(), requestJSON(t, nil), func(context.Context, request, bool) (supportdelete.Outcome, error) {
		return supportdelete.Outcome{Code: privateError, Stage: privateError, Mode: privateError}, errors.New(privateError)
	})
	require.Equal(t, 1, code)
	require.Equal(t, "WORKFLOW_BLOCKED", out.Code)
	require.Equal(t, "blocked", out.Status)
	code, out, _ = runTest(t, []string{"--stdin", "--mock"}, testEnvironment(), requestJSON(t, nil), nil)
	require.Equal(t, 1, code)
	require.Equal(t, "WORKFLOW_BLOCKED", out.Code)
}

func TestCancelledMockNeverInvokesRunner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := command(ctx, []string{"--stdin", "--mock"}, func(k string) string { return testEnvironment()[k] }, strings.NewReader(requestJSON(t, nil)), &stdout, &stderr, func(context.Context, request, bool) (supportdelete.Outcome, error) {
		t.Fatal("cancelled command invoked runner")
		return completedMock(), nil
	})
	require.Equal(t, 1, code)
	require.NotContains(t, stdout.String()+stderr.String(), privateError)
}

func TestManifestAndIOErrorsNeverExposePathsOrRawErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), privateError+".json")
	require.NoError(t, os.WriteFile(path, []byte(requestJSON(t, nil)), 0o600))
	code, out, output := runTest(t, []string{"--manifest", path}, testEnvironment(), "", nil)
	require.Zero(t, code)
	require.Equal(t, "PLAN_ONLY", out.Code)
	require.NotContains(t, output, path)
	require.NoError(t, os.Chmod(path, 0o644))
	code, out, output = runTest(t, []string{"--manifest", path}, testEnvironment(), "", nil)
	require.Equal(t, 2, code)
	require.Equal(t, "INVALID_INPUT", out.Code)
	require.NotContains(t, output, path)
	var stdout, stderr bytes.Buffer
	code = command(context.Background(), []string{"--stdin"}, func(k string) string { return testEnvironment()[k] }, failingIO{}, &stdout, &stderr, nil)
	require.Equal(t, 2, code)
	require.NotContains(t, stdout.String()+stderr.String(), privateError)
	code = command(context.Background(), []string{"--stdin"}, func(k string) string { return testEnvironment()[k] }, strings.NewReader(requestJSON(t, nil)), failingIO{}, &stderr, nil)
	require.Equal(t, 1, code)
	require.Equal(t, "MyPage support deletion report unavailable\n", stderr.String())
}

func TestDefaultRunnerRejectsWrongOpaqueReferences(t *testing.T) {
	for _, key := range []string{"requestRef", "executionRef", "proofRef"} {
		t.Run(key, func(t *testing.T) {
			input := requestJSON(t, func(v map[string]any) {
				v[key] = strings.Repeat("f", 64)
				objectField(t, v, "confirmation")[key] = strings.Repeat("f", 64)
			})
			code, out, _ := runTest(t, []string{"--stdin", "--mock"}, testEnvironment(), input, supportdelete.RunMock)
			require.Equal(t, 1, code)
			require.Equal(t, "WORKFLOW_BLOCKED", out.Code)
		})
	}
}

func TestDefaultRunnerUsesIndependentSyntheticEvidence(t *testing.T) {
	for _, recover := range []bool{false, true} {
		args := []string{"--stdin", "--mock"}
		if recover {
			args = append(args, "--recover")
		}
		code, out, _ := runTest(t, args, testEnvironment(), requestJSON(t, nil), supportdelete.RunMock)
		require.Zero(t, code)
		require.Equal(t, "MOCK_COMPLETED", out.Code)
		require.Equal(t, "mock-only", out.Status)
	}
}
