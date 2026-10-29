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
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/supportdelete"
)

func TestLiveCLIRefusesProductionBeforeBootstrapAndKeepsFailuresPrivate(t *testing.T) {
	for _, allow := range []bool{false, true} {
		env := testEnvironment()
		env["MYPAGE_ENVIRONMENT"] = "production"
		input := requestJSON(t, func(v map[string]any) {
			objectField(t, v, "target")["environment"] = "production"
			objectField(t, v, "confirmation")["environment"] = "production"
		})
		args := []string{"--stdin", "--execute"}
		if allow {
			args = append(args, "--allow-production")
		}
		called := false
		var out bytes.Buffer
		code := command(context.Background(), args, func(k string) string { return env[k] }, strings.NewReader(input), &out, &bytes.Buffer{}, func(context.Context, request, options) (supportdelete.Outcome, error) {
			called = true
			return supportdelete.Outcome{Mode: "live", ActualExecution: true}, errors.New(privateError)
		})
		if allow {
			require.Equal(t, 1, code)
		} else {
			require.Equal(t, 2, code)
		}
		require.Equal(t, allow, called)
		require.NotContains(t, out.String(), privateError)
		require.NotContains(t, out.String(), testChannel)
	}
}

func TestLiveCLICannotTreatManualGateOrUnknownOutcomeAsCompletion(t *testing.T) {
	for _, result := range []supportdelete.Outcome{{Code: "MANUAL_GATE_REQUIRED", Mode: "live", ActualExecution: true}, {Code: "UNRECOGNIZED", Mode: "live", ActualExecution: true}} {
		var out bytes.Buffer
		code := command(context.Background(), []string{"--stdin", "--execute"}, func(k string) string { return testEnvironment()[k] }, strings.NewReader(requestJSON(t, nil)), &out, &bytes.Buffer{}, func(context.Context, request, options) (supportdelete.Outcome, error) { return result, nil })
		require.Equal(t, 1, code)
		var report report
		require.NoError(t, json.Unmarshal(out.Bytes(), &report))
		require.Equal(t, "blocked", report.Status)
	}
}

func TestPrivateCheckpointCannotOverwriteOtherTargetOrFollowSymlink(t *testing.T) {
	r, err := readRequest(strings.NewReader(requestJSON(t, nil)))
	require.NoError(t, err)
	e := supportdelete.Execution{Selector: r, Generation: 1, Cutoff: time.Now().UTC()}
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	require.NoError(t, writeCheckpoint(path, e))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	e.Cursor++
	require.NoError(t, writeCheckpoint(path, e))
	e.Selector.Target.ChannelID = "UCother000000000000000001"
	require.Error(t, writeCheckpoint(path, e))
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(path, link))
	require.Error(t, writeCheckpoint(link, e))
}

func TestLiveBootstrapRejectsUnboundEnvironmentWithoutCredentials(t *testing.T) {
	r, err := readRequest(strings.NewReader(requestJSON(t, nil)))
	require.NoError(t, err)
	_, err = executeCurrent(context.Background(), r, options{checkpoint: "private.json"})
	require.Error(t, err)
}
