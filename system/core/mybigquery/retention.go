package mybigquery

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
)

// DeleteLiveChatHistoryBefore removes raw YouTube live chat rows older than cutoff.
// Raw YouTube API data is intentionally kept separate from study-space-owned
// activity data so that its retention policy can be enforced independently.
func (c *BigqueryController) DeleteLiveChatHistoryBefore(ctx context.Context, cutoff time.Time) error {
	query := c.Client.Query(fmt.Sprintf(
		"DELETE FROM `%s.%s.%s` WHERE published_at < @cutoff",
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
