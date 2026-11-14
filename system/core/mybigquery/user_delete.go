package mybigquery

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

// DeleteUserData removes rows attributable to a YouTube channel ID from the
// Study Space BigQuery history tables. GCS source snapshots are intentionally
// out of scope and must be handled by the backup-retention policy separately.
func (c *BigqueryController) DeleteUserData(ctx context.Context, youtubeChannelID string) error {
	youtubeChannelID = strings.TrimSpace(youtubeChannelID)
	if youtubeChannelID == "" {
		return fmt.Errorf("youtube channel id is empty")
	}

	deletions := []struct {
		tableName string
		fieldName string
	}{
		{tableName: LiveChatHistoryMainTableName, fieldName: "author_channel_id"},
		{tableName: UserActivityHistoryMainTableName, fieldName: "user_id"},
		{tableName: OrderHistoryMainTableName, fieldName: "user_id"},
	}

	for _, deletion := range deletions {
		if err := c.deleteRowsByField(
			ctx,
			deletion.tableName,
			deletion.fieldName,
			youtubeChannelID,
		); err != nil {
			return fmt.Errorf("delete user rows from %s: %w", deletion.tableName, err)
		}
	}

	return nil
}

func (c *BigqueryController) deleteRowsByField(
	ctx context.Context,
	tableName string,
	fieldName string,
	value string,
) error {
	query := c.Client.Query(fmt.Sprintf(
		"DELETE FROM `%s.%s.%s` WHERE %s = @value",
		c.Client.Project(),
		DatasetName,
		tableName,
		fieldName,
	))
	query.Location = c.WorkingRegion
	query.Parameters = []bigquery.QueryParameter{{Name: "value", Value: value}}

	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("run delete query: %w", err)
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait delete query: %w", err)
	}
	if err := status.Err(); err != nil {
		return fmt.Errorf("delete query failed: %w", err)
	}
	if status.State != bigquery.Done {
		return fmt.Errorf("delete query did not finish: state=%v", status.State)
	}

	return nil
}
