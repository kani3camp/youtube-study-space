package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationRefusesUnsafeTargetBeforeReadingInputOrDiscoveringCredentials(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"--environment=production", "--project=youtube-study-space"},
		{"--environment=development", "--project=youtube-study-space"},
		{"--environment=development", "--project=test-youtube-study-space", "--max-documents=10001"},
		{"--environment=development", "--project=test-youtube-study-space", "--operation=unknown"},
	} {
		err := run(context.Background(), args, strings.NewReader("not-json"), new(bytes.Buffer), func(string) string { return "" })
		require.ErrorIs(t, err, errMigration)
	}
}

func TestMigrationRejectsUnreviewedExecutionAndUnsafeSDKConfiguration(t *testing.T) {
	args := []string{"--environment=development", "--project=test-youtube-study-space", "--execute"}
	input := `{"uid":"UCsynthetic0000000000001","month":"2026-10"}`
	for _, settings := range []map[string]string{
		{},
		{"FIRESTORE_EMULATOR_HOST": "remote.example:8080"},
		{"GOOGLE_CLOUD_PROJECT": "youtube-study-space"},
		{"GOOGLE_APPLICATION_CREDENTIALS": "private-file"},
		{"FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099"},
	} {
		out := new(bytes.Buffer)
		err := run(context.Background(), args, strings.NewReader(input), out, func(key string) string { return settings[key] })
		require.ErrorIs(t, err, errMigration)
		require.Empty(t, out.String())
	}
}
