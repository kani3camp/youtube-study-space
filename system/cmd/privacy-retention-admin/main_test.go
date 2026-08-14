package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"app.modules/core/mybigquery"
)

func TestDeleteCandidateCount(t *testing.T) {
	audit := mybigquery.RawLiveChatRetentionAudit{
		RowsOlderThanCutoff: 47_451,
		UndatedRows:         3,
	}
	assert.EqualValues(t, 47_454, deleteCandidateCount(audit))
}

func TestConfirmationToken(t *testing.T) {
	cutoff := time.Date(2026, 7, 15, 13, 52, 23, 0, time.FixedZone("JST-ish", 9*60*60))
	assert.Equal(
		t,
		"DELETE 47451 RAW YOUTUBE LIVE CHAT ROWS BEFORE 2026-07-15T04:52:23Z",
		confirmationToken(cutoff, 47_451),
	)
}
