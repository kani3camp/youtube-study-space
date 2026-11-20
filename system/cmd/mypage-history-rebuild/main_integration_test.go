//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/internal/integrationtest"
)

func TestHistoryMigrationCLIRebuildCutoverAndRollback(t *testing.T) {
	integrationtest.ResetFirestore(t)
	repo := integrationtest.NewFirestoreController(t)
	ctx := context.Background()
	uid := "UCsynthetic0000000000001"
	_, err := repo.FirestoreClient().Collection(repository.USERS).Doc(uid).Set(ctx, repository.UserDoc{})
	require.NoError(t, err)
	base := []string{"--environment=development", "--project=demo-youtube-study-space-ci", "--max-documents=10"}
	invoke := func(selection migrationInput, operation string, execute bool) migrationReport {
		args := append([]string{}, base...)
		args = append(args, "--operation="+operation)
		if execute {
			args = append(args, "--execute")
		}
		body, err := json.Marshal(selection)
		require.NoError(t, err)
		var output bytes.Buffer
		require.NoError(t, run(ctx, args, bytes.NewReader(body), &output, os.Getenv))
		require.NotContains(t, output.String(), uid)
		var report migrationReport
		require.NoError(t, json.Unmarshal(output.Bytes(), &report))
		return report
	}
	seen := map[string]bool{}
	for i := 0; i < 7; i++ {
		month := time.Now().In(timeutil.JapanLocation()).AddDate(0, 0, -i).Format("2006-01")
		if seen[month] {
			continue
		}
		seen[month] = true
		selection := migrationInput{UID: uid, Month: month}
		dry := invoke(selection, "rebuild", false)
		require.True(t, dry.ReadOnly)
		require.False(t, dry.Applied)
		selection.Checkpoint = dry.Checkpoint
		selection.SourceVersion = dry.SourceVersion
		applied := invoke(selection, "rebuild", true)
		require.True(t, applied.Applied)
		require.Equal(t, 0, applied.Documents)
	}
	selection := migrationInput{UID: uid, Checkpoint: "absent"}
	dry := invoke(selection, "cutover", false)
	selection.UserRevision = dry.UserRevision
	require.True(t, invoke(selection, "cutover", true).Applied)
	user, err := repo.FirestoreClient().Collection(repository.USERS).Doc(uid).Get(ctx)
	require.NoError(t, err)
	require.Equal(t, repository.MonthlyWorkEpoch, user.Data()["work-history-epoch"])
	selection.UserRevision = time.Time{}
	dry = invoke(selection, "rollback", false)
	selection.UserRevision = dry.UserRevision
	require.True(t, invoke(selection, "rollback", true).Applied)
	user, err = repo.FirestoreClient().Collection(repository.USERS).Doc(uid).Get(ctx)
	require.NoError(t, err)
	require.NotContains(t, user.Data(), "work-history-epoch")
}
