package mybigquery

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/googleapi"

	"app.modules/core/serviceaccess"
)

// ApplyChannelDeletion handles only the current fixed primary archive tables.
// Export snapshots, backups and other copies require the separate owner Runbook gate.
// Stable job IDs reconcile a lost DML acknowledgement without inventing completion.
func (c *BigqueryController) ApplyChannelDeletion(ctx context.Context, channel, operation string, inspect bool) error {
	if c == nil || c.Client == nil || !serviceaccess.ValidChannel(channel) || len(operation) != 64 {
		return errors.New("channel deletion unavailable")
	}
	dataset := c.Client.Dataset(DatasetName)
	metadata, err := dataset.Metadata(ctx)
	if err != nil || metadata.Location == "" {
		return errors.New("archive inventory unavailable")
	}
	for _, table := range []struct{ name, field string }{
		{LiveChatHistoryMainTableName, "author_channel_id"}, {UserActivityHistoryMainTableName, "user_id"}, {OrderHistoryMainTableName, "user_id"}, {TemporaryTableName, "user_id"},
	} {
		schema, err := dataset.Table(table.name).Metadata(ctx)
		var api *googleapi.Error
		if errors.As(err, &api) && api.Code == 404 {
			continue
		}
		if err != nil {
			return errors.New("archive table inventory unavailable")
		}
		found := false
		for _, f := range schema.Schema {
			if f.Name == table.field && f.Type == bigquery.StringFieldType && !f.Repeated {
				found = true
			}
		}
		if !found {
			return errors.New("archive ownership field unavailable")
		}
		if !inspect {
			sql, err := TargetDeletionSQL(c.Client.Project(), DatasetName, table.name, table.field)
			if err != nil {
				return err
			}
			q := c.Client.Query(sql)
			q.Location = metadata.Location
			q.Parameters = []bigquery.QueryParameter{{Name: "channel", Value: channel}}
			q.JobID = "mypage_delete_" + operation + "_" + strings.ReplaceAll(table.name, "-", "_")
			job, err := q.Run(ctx)
			if errors.As(err, &api) && api.Code == 409 {
				job, err = c.Client.JobFromIDLocation(ctx, q.JobID, metadata.Location)
				if err == nil {
					config, configErr := job.Config()
					query, ok := config.(*bigquery.QueryConfig)
					if configErr != nil || !ok || query.Q != sql || query.UseLegacySQL || len(query.Parameters) != 1 || query.Parameters[0].Name != "channel" || query.Parameters[0].Value != channel {
						return errors.New("archive job binding unavailable")
					}
				}
			}
			if err != nil {
				return errors.New("archive deletion unavailable")
			}
			result, err := job.Wait(ctx)
			if err != nil || result.Err() != nil {
				return errors.New("archive deletion incomplete")
			}
		}
		q := c.Client.Query(fmt.Sprintf("SELECT COUNT(*) AS row_count FROM `%s.%s.%s` WHERE %s = @channel", c.Client.Project(), DatasetName, table.name, table.field))
		q.Location = metadata.Location
		q.Parameters = []bigquery.QueryParameter{{Name: "channel", Value: channel}}
		rows, err := q.Read(ctx)
		if err != nil {
			return errors.New("archive reread unavailable")
		}
		var row countRow
		if rows.Next(&row) != nil || row.RowCount != 0 {
			return errors.New("archive target remains")
		}
	}
	return nil
}
