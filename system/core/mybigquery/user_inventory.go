package mybigquery

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

type UserDataInventory struct {
	LiveChatHistoryRows int64 `json:"live_chat_history_rows"`
	UserActivityRows    int64 `json:"user_activity_rows"`
	OrderHistoryRows    int64 `json:"order_history_rows"`
}

type countRow struct {
	RowCount int64 `bigquery:"row_count"`
}

// InspectUserData returns counts of BigQuery rows attributable to a YouTube
// channel ID. It does not modify any table.
func (c *BigqueryController) InspectUserData(
	ctx context.Context,
	youtubeChannelID string,
) (UserDataInventory, error) {
	youtubeChannelID = strings.TrimSpace(youtubeChannelID)
	if youtubeChannelID == "" {
		return UserDataInventory{}, fmt.Errorf("youtube channel id is empty")
	}

	liveChatRows, err := c.countRowsByField(
		ctx,
		LiveChatHistoryMainTableName,
		"author_channel_id",
		youtubeChannelID,
	)
	if err != nil {
		return UserDataInventory{}, fmt.Errorf("count live chat history rows: %w", err)
	}

	userActivityRows, err := c.countRowsByField(
		ctx,
		UserActivityHistoryMainTableName,
		"user_id",
		youtubeChannelID,
	)
	if err != nil {
		return UserDataInventory{}, fmt.Errorf("count user activity rows: %w", err)
	}

	orderHistoryRows, err := c.countRowsByField(
		ctx,
		OrderHistoryMainTableName,
		"user_id",
		youtubeChannelID,
	)
	if err != nil {
		return UserDataInventory{}, fmt.Errorf("count order history rows: %w", err)
	}

	return UserDataInventory{
		LiveChatHistoryRows: liveChatRows,
		UserActivityRows:    userActivityRows,
		OrderHistoryRows:    orderHistoryRows,
	}, nil
}

func (c *BigqueryController) countRowsByField(
	ctx context.Context,
	tableName string,
	fieldName string,
	value string,
) (int64, error) {
	query := c.Client.Query(fmt.Sprintf(
		"SELECT COUNT(*) AS row_count FROM `%s.%s.%s` WHERE %s = @value",
		c.Client.Project(),
		DatasetName,
		tableName,
		fieldName,
	))
	query.Location = c.WorkingRegion
	query.Parameters = []bigquery.QueryParameter{{Name: "value", Value: value}}

	iterator, err := query.Read(ctx)
	if err != nil {
		return 0, fmt.Errorf("read count query: %w", err)
	}

	var row countRow
	if err := iterator.Next(&row); err != nil {
		return 0, fmt.Errorf("read count query row: %w", err)
	}
	return row.RowCount, nil
}
