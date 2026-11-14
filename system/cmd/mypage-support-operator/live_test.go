package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"app.modules/core/mypage"
)

func TestPrivateIntakeHandoffIsExclusive0600AndNeverFollowsSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := intakeReceipt{RequestRef: "synthetic-receipt", Challenge: "synthetic-private-challenge"}
	require.NoError(t, writePrivateHandoff(path, receipt))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	require.Error(t, writePrivateHandoff(path, receipt))
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(path, link))
	require.Error(t, writePrivateHandoff(link, receipt))
}

func TestOwnerIntakeNeedsProductionConfirmationBeforeLiveFactory(t *testing.T) {
	input := mockHumanRequest(mypage.SupportDelete)
	path := humanFile(t, input)
	for _, allow := range []bool{false, true} {
		args := humanArgs(path, mypage.SupportDelete)
		args[3] = "production"
		args[5] = "youtube-study-space"
		args = append(args, "--execute", "--handoff-file", filepath.Join(t.TempDir(), "handoff.json"))
		if allow {
			args = append(args, "--allow-production")
		}
		get := func(k string) string {
			if k == "MYPAGE_ENVIRONMENT" {
				return "production"
			}
			if k == "GOOGLE_CLOUD_PROJECT" {
				return "youtube-study-space"
			}
			return ""
		}
		called := false
		var out bytes.Buffer
		code := commandWithIntake(context.Background(), args, get, &out, mockIntakeFactory, func(context.Context, proposal) (*intakeRuntime, error) { called = true; return nil, errIntake })
		require.Equal(t, 1, code)
		require.Equal(t, allow, called)
		require.NotContains(t, out.String(), input.Channel)
	}
}
