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

func TestPrivateCheckpointSerializesInvocationAndKeepsDistinctStableOwner(t *testing.T) {
	r, err := readRequest(strings.NewReader(requestJSON(t, nil)))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	f, owner, err := openCheckpoint(path, r)
	require.NoError(t, err)
	t.Cleanup(func() { err := f.Close(); require.True(t, err == nil || errors.Is(err, os.ErrClosed)) })
	require.True(t, supportdelete.ValidRef(owner))
	info, err := f.Stat()
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	_, _, err = openCheckpoint(path, r)
	require.Error(t, err, "same checkpoint cannot execute concurrently")
	e := supportdelete.Execution{Selector: r, OwnerRef: owner, Generation: 1, Cutoff: time.Now().UTC()}
	require.NoError(t, writeCheckpoint(f, e))
	require.NoError(t, f.Close())
	reopened, resumedOwner, err := openCheckpoint(path, r)
	require.NoError(t, err)
	t.Cleanup(func() { err := reopened.Close(); require.True(t, err == nil || errors.Is(err, os.ErrClosed)) })
	require.Equal(t, owner, resumedOwner, "retry must keep the persisted owner")
	other, otherOwner, err := openCheckpoint(filepath.Join(t.TempDir(), "other.json"), r)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, other.Close()) })
	require.NotEqual(t, owner, otherOwner, "another checkpoint must encounter the store owner fence")
	require.NoError(t, reopened.Close())
	r.Target.ChannelID = "UCother000000000000000001"
	_, _, err = openCheckpoint(path, r)
	require.Error(t, err)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(path, link))
	_, _, err = openCheckpoint(link, e.Selector)
	require.Error(t, err)
}

func TestLiveBootstrapRejectsUnboundEnvironmentWithoutCredentials(t *testing.T) {
	r, err := readRequest(strings.NewReader(requestJSON(t, nil)))
	require.NoError(t, err)
	_, err = executeCurrent(context.Background(), r, options{checkpoint: "private.json"})
	require.Error(t, err)
}
