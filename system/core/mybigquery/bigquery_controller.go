package mybigquery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"app.modules/core/repository"
	"app.modules/core/timeutil"
)

type BigqueryController struct {
	Client        *bigquery.Client
	WorkingRegion string
}

func NewBigqueryClient(ctx context.Context, projectID string, clientOption option.ClientOption,
	workingRegion string) (*BigqueryController,
	error,
) {
	client, err := bigquery.NewClient(ctx, projectID, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in bigquery.NewClient: %w", err)
	}

	return &BigqueryController{
		Client:        client,
		WorkingRegion: workingRegion,
	}, nil
}

func (c *BigqueryController) CloseClient() {
	if err := c.Client.Close(); err != nil {
		slog.Error("failed to close bigquery client.")
	} else {
		slog.Info("successfully closed bigquery client.")
	}
}

func shouldArchiveCollectionToBigQuery(collectionName string) bool {
	// Raw YouTube live chat is intentionally not a long-term analytics/archive
	// dataset. Firestore keeps it only for its short operational retention, and
	// historical BigQuery rows are cleaned up by the privacy retention operation.
	return collectionName != repository.LiveChatHistory
}

var retainedCollectionFields = map[string][]string{
	repository.UserActivities: {
		"seat_id",
		"taken_at",
		"user_id",
		"activity_type",
		"is_member_seat",
		"__key__",
		"__error__",
		"__has_error__",
	},
	repository.OrderHistory: {
		"seat_id",
		"ordered_at",
		"user_id",
		"menu_code",
		"is_member_seat",
		"__key__",
		"__error__",
		"__has_error__",
	},
}

func retainedCollectionProjection(collectionName string, schema bigquery.Schema) (string, error) {
	expected, ok := retainedCollectionFields[collectionName]
	if !ok {
		return "", fmt.Errorf("collection %q has no retained BigQuery schema contract", collectionName)
	}

	expectedSet := make(map[string]struct{}, len(expected))
	for _, name := range expected {
		expectedSet[name] = struct{}{}
	}

	seen := make(map[string]struct{}, len(schema))
	projection := make([]string, 0, len(schema))
	for _, field := range schema {
		if _, ok := expectedSet[field.Name]; !ok {
			return "", fmt.Errorf("collection %q temporary table has unexpected field %q", collectionName, field.Name)
		}
		if _, ok := seen[field.Name]; ok {
			return "", fmt.Errorf("collection %q temporary table has duplicate field %q", collectionName, field.Name)
		}
		seen[field.Name] = struct{}{}
		projection = append(projection, "`"+field.Name+"`")
	}

	missing := make([]string, 0)
	for _, name := range expected {
		if _, ok := seen[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("collection %q temporary table is missing required fields: %s", collectionName, strings.Join(missing, ", "))
	}

	return strings.Join(projection, ", "), nil
}

func (c *BigqueryController) ReadCollectionsFromGcs(ctx context.Context,
	gcsFolderName string, bucketName string,
	collections []string,
) error {
	for _, collectionName := range collections {
		if !shouldArchiveCollectionToBigQuery(collectionName) {
			slog.Info("skipping raw YouTube live chat BigQuery archive", "collection", collectionName)
			continue
		}

		// GCSからbigqueryの一時テーブルにデータをバッチ読込
		gcsRef := bigquery.NewGCSReference("gs://" + bucketName + "/" + gcsFolderName + "/all_namespaces/kind_" +
			"" + collectionName + "/all_namespaces_kind_" + collectionName + ".export_metadata")
		gcsRef.AllowJaggedRows = true
		gcsRef.SourceFormat = bigquery.DatastoreBackup

		dataset := c.Client.Dataset(DatasetName)
		loader := dataset.Table(TemporaryTableName).LoaderFrom(gcsRef)
		loader.WriteDisposition = bigquery.WriteTruncate // 上書き
		loader.Location = c.WorkingRegion
		job, err := loader.Run(ctx)
		if err != nil {
			return fmt.Errorf("in loader.Run: %w", err)
		}
		status, err := job.Wait(ctx)
		if err != nil {
			return fmt.Errorf("in job.Wait: %w", err)
		}
		if err = status.Err(); err != nil {
			return fmt.Errorf("load collection %q into BigQuery: %w", collectionName, err)
		}
		if status.State == bigquery.Done {
			slog.Info("GCSからbqの一時テーブルまでデータの読込が完了", "collection", collectionName)
		} else {
			slog.Info("GCSからbqの一時テーブルまでデータの読込", "state", status.State, "collection", collectionName)
			return fmt.Errorf("failed transfer data from gcs to bigquery temporary table. collection: %s", collectionName)
		}

		// 取得する始まりと終わりの日時を求める
		jstNow := timeutil.JstNow()
		yesterday := jstNow.AddDate(0, 0, -1)
		yesterdayStart := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		yesterdayEnd := time.Date(jstNow.Year(), jstNow.Month(), jstNow.Day(), 0, 0, 0, 0, jstNow.Location())

		// bigqueryにおいて一時テーブルから日時を指定してメインテーブルにデータを読込
		var query *bigquery.Query

		// 一時テーブルにロードされたデータが0件ならばここで終了。1件も読み込まれないと一時テーブルのスキーマが定義されないため、後続のクエリでエラーになる。
		query = c.Client.Query("SELECT * FROM `" + c.Client.Project() + "." + DatasetName + "." + TemporaryTableName + "` LIMIT 10")
		it, err := query.Read(ctx)
		if err != nil {
			return fmt.Errorf("in query.Read: %w", err)
		}
		numRows, err := iteratorSize(it)
		if err != nil {
			return fmt.Errorf("in iteratorSize: %w", err)
		}
		if numRows == 0 {
			slog.Info("number of loaded rows is zero.", "collection", collectionName)
			continue
		}

		tmpMetadata, err := dataset.Table(TemporaryTableName).Metadata(ctx)
		if err != nil {
			return fmt.Errorf("read temporary BigQuery table metadata for collection %q: %w", collectionName, err)
		}
		projection, err := retainedCollectionProjection(collectionName, tmpMetadata.Schema)
		if err != nil {
			return err
		}

		switch collectionName {
		case repository.LiveChatHistory:
			query = c.Client.Query("SELECT * FROM `" + c.Client.Project() + "." + DatasetName + "." +
				TemporaryTableName + "` WHERE FORMAT_TIMESTAMP('%F %T', published_at, '+09:00') " +
				"BETWEEN '" + yesterdayStart.Format("2006-01-02 15:04:05") + "' AND '" +
				yesterdayEnd.Format("2006-01-02 15:04:05") + "'")
		case repository.UserActivities:
			query = c.Client.Query("SELECT " + projection + " FROM `" + c.Client.Project() + "." + DatasetName + "." +
				TemporaryTableName + "` WHERE FORMAT_TIMESTAMP('%F %T', taken_at, '+09:00') " +
				"BETWEEN '" + yesterdayStart.Format("2006-01-02 15:04:05") + "' AND '" +
				yesterdayEnd.Format("2006-01-02 15:04:05") + "'")
		case repository.OrderHistory:
			query = c.Client.Query("SELECT " + projection + " FROM `" + c.Client.Project() + "." + DatasetName + "." +
				TemporaryTableName + "` WHERE FORMAT_TIMESTAMP('%F %T', ordered_at, '+09:00') " +
				"BETWEEN '" + yesterdayStart.Format("2006-01-02 15:04:05") + "' AND '" +
				yesterdayEnd.Format("2006-01-02 15:04:05") + "'")
		}
		query.Location = c.WorkingRegion
		query.WriteDisposition = bigquery.WriteAppend // 追加
		switch collectionName {
		case repository.LiveChatHistory:
			query.Dst = dataset.Table(LiveChatHistoryMainTableName)
		case repository.UserActivities:
			query.Dst = dataset.Table(UserActivityHistoryMainTableName)
		case repository.OrderHistory:
			query.Dst = dataset.Table(OrderHistoryMainTableName)
		}
		job, err = query.Run(ctx)
		if err != nil {
			return fmt.Errorf("in query.Run: %w", err)
		}
		status, err = job.Wait(ctx)
		if err != nil {
			return fmt.Errorf("in job.Wait: %w", err)
		}
		if err = status.Err(); err != nil {
			return fmt.Errorf("in status.Err: %w", err)
		}
		if status.State == bigquery.Done {
			slog.Info("bqの一時テーブルからメインテーブルまでデータの移行が完了", "collection", collectionName)
		} else {
			slog.Error("bqの一時テーブルからメインテーブルまでデータの移行結果", "state", status.State, "collection", collectionName)
			return fmt.Errorf("failed transfer data from bigquery temporary table to main table. collection: %s", collectionName)
		}
	}
	slog.Info("finished all collection's processes.", "collections", collections)
	return nil
}

func iteratorSize(it *bigquery.RowIterator) (int, error) {
	i := 0
	for {
		var row []bigquery.Value
		err := it.Next(&row)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return -1, fmt.Errorf("in it.Next: %w", err)
		}
		i++
	}
	return i, nil
}
