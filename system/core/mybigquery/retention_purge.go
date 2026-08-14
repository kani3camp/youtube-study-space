package mybigquery

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
)

// PurgeRawLiveChatBefore deletes raw YouTube live-chat rows before cutoff only
// when the candidate count still equals expectedRows. The count check and DELETE
// run in one BigQuery transaction so the operator's preview cannot silently
// expand to a different snapshot at apply time.
func (c *BigqueryController) PurgeRawLiveChatBefore(
	ctx context.Context,
	cutoff time.Time,
	expectedRows int64,
) error {
	if expectedRows <= 0 {
		return fmt.Errorf("expected rows must be positive: %d", expectedRows)
	}

	tableIdentifier := fmt.Sprintf(
		"`%s.%s.%s`",
		c.Client.Project(),
		DatasetName,
		LiveChatHistoryMainTableName,
	)
	query := c.Client.Query(buildRawLiveChatPurgeQuery(tableIdentifier))
	query.Location = c.WorkingRegion
	query.Parameters = []bigquery.QueryParameter{
		{Name: "cutoff", Value: cutoff},
		{Name: "expected_rows", Value: expectedRows},
	}

	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("run raw live chat purge transaction: %w", err)
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait raw live chat purge transaction: %w", err)
	}
	if err := status.Err(); err != nil {
		return fmt.Errorf("raw live chat purge transaction failed: %w", err)
	}
	if status.State != bigquery.Done {
		return fmt.Errorf("raw live chat purge transaction did not finish: state=%v", status.State)
	}
	return nil
}

func buildRawLiveChatPurgeQuery(tableIdentifier string) string {
	return fmt.Sprintf(`
BEGIN TRANSACTION;

SELECT IF(
  (SELECT COUNT(*) FROM %s WHERE published_at IS NULL OR published_at < @cutoff) = @expected_rows,
  TRUE,
  ERROR('raw live chat purge candidate count changed; rerun preview')
) AS candidate_count_verified;

DELETE FROM %s
WHERE published_at IS NULL OR published_at < @cutoff;

COMMIT TRANSACTION;
`, tableIdentifier, tableIdentifier)
}
