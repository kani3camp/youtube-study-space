package mybigquery

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRawLiveChatPurgeQuery(t *testing.T) {
	const table = "`project.dataset.live-chat-history`"
	query := buildRawLiveChatPurgeQuery(table)

	assert.Contains(t, query, "BEGIN TRANSACTION;")
	assert.Contains(t, query, "@expected_rows")
	assert.Contains(t, query, "@cutoff")
	assert.Contains(t, query, "ERROR('raw live chat purge candidate count changed; rerun preview')")
	assert.Contains(t, query, "DELETE FROM "+table)
	assert.Contains(t, query, "published_at IS NULL OR published_at < @cutoff")
	assert.Contains(t, query, "COMMIT TRANSACTION;")
	assert.Equal(t, 2, strings.Count(query, table))
}
