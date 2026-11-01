package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"app.modules/core/supportdelete"

	"github.com/stretchr/testify/require"
)

func TestOfflineModesValidateTargetWithoutInvokingAnySDK(t *testing.T) {
	for _, mode := range []string{"plan", "check-config"} {
		args := []string{"--stdin"}
		if mode != "plan" {
			args = append(args, "--check-config")
		}
		var out bytes.Buffer
		code := command(context.Background(), args, func(k string) string { return testEnvironment()[k] }, strings.NewReader(requestJSON(t, nil)), &out, &bytes.Buffer{}, func(context.Context, request, options) (supportdelete.Outcome, error) {
			t.Fatal("offline mode invoked SDK")
			return supportdelete.Outcome{}, nil
		})
		require.Zero(t, code)
		var report report
		require.NoError(t, json.Unmarshal(out.Bytes(), &report))
		require.True(t, report.Offline)
		require.False(t, report.ActualExecution)
		require.NotContains(t, out.String(), testChannel)
	}
}

func TestInvalidSelectionCannotReachLiveBootstrap(t *testing.T) {
	for _, input := range []string{privateError, requestJSON(t, func(v map[string]any) { v["proofRef"] = "invalid" }), requestJSON(t, func(v map[string]any) { v["skipScopes"] = []string{"backups"} })} {
		var out bytes.Buffer
		code := command(context.Background(), []string{"--stdin", "--execute"}, func(k string) string { return testEnvironment()[k] }, strings.NewReader(input), &out, &bytes.Buffer{}, func(context.Context, request, options) (supportdelete.Outcome, error) {
			t.Fatal("invalid selection invoked SDK")
			return supportdelete.Outcome{}, nil
		})
		require.Equal(t, 2, code)
		require.NotContains(t, out.String(), privateError)
	}
}
