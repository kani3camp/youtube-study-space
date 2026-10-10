package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const syntheticPrivate = "synthetic-private-reply-should-never-print"

func testArgs(path string) []string {
	return []string{"--action", "reply", "--environment", "development", "--project", "demo-youtube-study-space-ci", "--purpose", "disclosure", "--request-ref", strings.Repeat("a", 64), "--proof-ref", strings.Repeat("b", 64), "--operation-id", strings.Repeat("c", 64), "--expected-revision", "0", "--reply-file", path}
}

func testGet(key string) string {
	switch key {
	case "MYPAGE_ENVIRONMENT":
		return "development"
	case "GOOGLE_CLOUD_PROJECT":
		return "demo-youtube-study-space-ci"
	}
	return ""
}

func TestOfflineOperatorCLIRejectsUnsafeInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reply.txt")
	require.NoError(t, os.WriteFile(path, []byte(syntheticPrivate), 0o600))
	for name, modify := range map[string]func([]string) []string{
		"unknown":           func(a []string) []string { return append(a, "--operator", "caller-name") },
		"duplicate":         func(a []string) []string { return append(a, "--purpose", "delete") },
		"wrong environment": func(a []string) []string { a[3] = "production"; return a },
		"wrong project":     func(a []string) []string { a[5] = "demo-other-project"; return a },
		"invalid purpose":   func(a []string) []string { a[7] = "login"; return a },
		"invalid ref":       func(a []string) []string { a[9] = "short"; return a },
		"negative revision": func(a []string) []string { a[15] = "-1"; return a },
		"delete completion": func(a []string) []string { a[1] = "complete"; a[7] = "delete"; return a },
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			require.Equal(t, 2, command(modify(testArgs(path)), testGet, &out))
			require.NotContains(t, out.String(), syntheticPrivate)
		})
	}
	var out bytes.Buffer
	require.Equal(t, 0, command(testArgs(path), testGet, &out))
	var report report
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))
	require.True(t, report.ReadOnly)
	require.False(t, report.ActualExecution)
	require.Equal(t, "READ_ONLY_PREVIEW", report.Code)
	out.Reset()
	require.Equal(t, 1, command(append(testArgs(path), "--execute"), testGet, &out))
	require.Contains(t, out.String(), "TRUSTED_OPERATOR_ADAPTER_UNAVAILABLE")
	require.NotContains(t, out.String(), syntheticPrivate)
	require.NoError(t, os.Chmod(path, 0o644))
	out.Reset()
	require.Equal(t, 2, command(testArgs(path), testGet, &out))
	require.NoError(t, os.Chmod(path, 0o600))
	link := filepath.Join(t.TempDir(), "reply-link")
	require.NoError(t, os.Symlink(path, link))
	out.Reset()
	require.Equal(t, 2, command(testArgs(link), testGet, &out))
}

func TestCompletionPreviewNeedsEvidenceAndAck(t *testing.T) {
	args := testArgs("")
	args[1] = "complete"
	args = args[:len(args)-2]
	args = append(args, "--reply-operation-id", strings.Repeat("d", 64), "--action-evidence", strings.Repeat("e", 64), "--delivery-ack", strings.Repeat("f", 64))
	args[15] = "1"
	var out bytes.Buffer
	require.Equal(t, 0, command(args, testGet, &out))
	args = args[:len(args)-2]
	out.Reset()
	require.Equal(t, 2, command(args, testGet, &out))
}
