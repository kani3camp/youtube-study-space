package mybigquery

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
)

type RawLiveChatRetentionAudit struct {
	TotalRows          int64  `json:"total_rows"`
	RowsOlderThanCutoff int64  `json:"rows_older_than_cutoff"`
	UndatedRows        int64  `json:"undated_rows"`
	OldestPublishedAt  string `json:"oldest_published_at,omitempty"`
	NewestPublishedAt  string `json:"newest_published_at,omitempty"`
}

type rawLiveChatRetentionAuditRow struct {
	TotalRows           int64  `bigquery:"total_rows"`
	RowsOlderThanCutoff int64  `bigquery:"rows_older_than_cutoff"`
	UndatedRows         int64  `bigquery:"undated_rows"`
	OldestPublishedAt   string `bigquery:"oldest_published_at"`
	NewestPublishedAt   string `bigquery:"newest_published_at"`
}

// InspectRawLiveChatRetention reports the current age distribution of raw
// live-chat rows without modifying the table.
func (c *BigqueryController) InspectRawLiveChatRetention(
	ctx context.Context,
	cutoff time.Time,
) (RawLiveChatRetentionAudit, error) {
	query := c.Client.Query(fmt.Sprintf(`
SELECT
  COUNT(*) AS total_rows,
  COUNTIF(published_at < @cutoff) AS rows_older_than_cutoff,
  COUNTIF(published_at IS NULL) AS undated_rows,
  COALESCE(FORMAT_TIMESTAMP('%%Y-%%m-%%dT%%H:%%M:%%SZ', MIN(published_at)), '') AS oldest_published_at,
  COALESCE(FORMAT_TIMESTAMP('%%Y-%%m-%%dT%%H:%%M:%%SZ', MAX(published_at)), '') AS newest_published_at
FROM %s
`, bqTableIdentifier(c.Client.Project(), DatasetName, LiveChatHistoryMainTableName)))
	query.Location = c.WorkingRegion
	query.Parameters = []bigquery.QueryParameter{{Name: "cutoff", Value: cutoff}}

	iterator, err := query.Read(ctx)
	if err != nil {
		return RawLiveChatRetentionAudit{}, fmt.Errorf("read raw live chat retention audit: %w", err)
	}

	var row rawLiveChatRetentionAuditRow
	if err := iterator.Next(&row); err != nil {
		return RawLiveChatRetentionAudit{}, fmt.Errorf("read raw live chat retention audit row: %w", err)
	}

	return RawLiveChatRetentionAudit{
		TotalRows:           row.TotalRows,
		RowsOlderThanCutoff: row.RowsOlderThanCutoff,
		UndatedRows:         row.UndatedRows,
		OldestPublishedAt:   row.OldestPublishedAt,
		NewestPublishedAt:   row.NewestPublishedAt,
	}, nil
}
