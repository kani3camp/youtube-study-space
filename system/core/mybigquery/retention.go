package mybigquery

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
)

// DeleteLiveChatHistoryBefore removes raw YouTube live chat rows older than cutoff.
// Rows without a published timestamp are also removed because their age cannot
// be verified against the retention limit.
func (c *BigqueryController) DeleteLiveChatHistoryBefore(ctx context.Context, cutoff time.Time) error {
	query := c.Client.Query(fmt.Sprintf(
		"DELETE FROM `%s.%s.%s` WHERE published_at IS NULL OR published_at < @cutoff",
		c.Client.Project(),
		DatasetName,
		LiveChatHistoryMainTableName,
	))
	query.Location = c.WorkingRegion
	query.Parameters = []bigquery.QueryParameter{
		{
			Name:  "cutoff",
			Value: cutoff,
		},
	}

	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("run live chat history retention query: %w", err)
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait live chat history retention query: %w", err)
	}
	if err := status.Err(); err != nil {
		return fmt.Errorf("live chat history retention query failed: %w", err)
	}
	if status.State != bigquery.Done {
		return fmt.Errorf("live chat history retention query did not finish: state=%v", status.State)
	}

	return nil
}
