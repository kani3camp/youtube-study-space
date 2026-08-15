package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/mybigquery"
)

func TestDeleteCandidateCount(t *testing.T) {
	audit := mybigquery.RawLiveChatRetentionAudit{
		RowsOlderThanCutoff: 47_451,
		UndatedRows:         3,
	}
	assert.EqualValues(t, 47_454, deleteCandidateCount(audit))
}

func TestBuildPurgeTarget(t *testing.T) {
	t.Parallel()

	target, err := buildPurgeTarget("production", "youtube-study-space-prod", "youtube-study-space-prod")
	require.NoError(t, err)
	assert.Equal(t, productionEnvironment, target.Environment)
	assert.Equal(t, "youtube-study-space-prod", target.ProjectID)
	assert.Equal(t, mybigquery.DatasetName, target.Dataset)
	assert.Equal(t, mybigquery.LiveChatHistoryMainTableName, target.Table)
	assert.Equal(t, "youtube-study-space-prod.firestore_export.live-chat-history", target.QualifiedTable)
}

func TestBuildPurgeTargetRejectsWrongProject(t *testing.T) {
	t.Parallel()

	_, err := buildPurgeTarget("development", "expected-dev", "different-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GCP project mismatch")
}

func TestBuildPurgeTargetRejectsUnknownEnvironment(t *testing.T) {
	t.Parallel()

	_, err := buildPurgeTarget("staging", "project", "project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment must be")
}

func TestValidateApplyTarget(t *testing.T) {
	t.Parallel()

	assert.NoError(t, validateApplyTarget(purgeTarget{Environment: developmentEnvironment}, false))
	assert.Error(t, validateApplyTarget(purgeTarget{Environment: developmentEnvironment}, true))
	assert.Error(t, validateApplyTarget(purgeTarget{Environment: productionEnvironment}, false))
	assert.NoError(t, validateApplyTarget(purgeTarget{Environment: productionEnvironment}, true))
}

func TestConfirmationToken(t *testing.T) {
	cutoff := time.Date(2026, 7, 15, 13, 52, 23, 0, time.FixedZone("JST-ish", 9*60*60))
	target := purgeTarget{
		Environment:    productionEnvironment,
		ProjectID:      "youtube-study-space-prod",
		Dataset:        mybigquery.DatasetName,
		Table:          mybigquery.LiveChatHistoryMainTableName,
		QualifiedTable: "youtube-study-space-prod.firestore_export.live-chat-history",
	}
	assert.Equal(
		t,
		"DELETE 47451 RAW YOUTUBE LIVE CHAT ROWS FROM PRODUCTION PROJECT youtube-study-space-prod TABLE youtube-study-space-prod.firestore_export.live-chat-history BEFORE 2026-07-15T04:52:23Z",
		confirmationToken(target, cutoff, 47_451),
	)
}
