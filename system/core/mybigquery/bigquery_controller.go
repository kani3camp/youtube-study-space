package mybigquery

import (
	"app.modules/core/myfirestore"
	"app.modules/core/utils"
	"cloud.google.com/go/bigquery"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"log/slog"
	"time"
)

type BigqueryController struct {
	Client        *bigquery.Client
	WorkingRegion string
}

func NewBigqueryClient(ctx context.Context, projectId string, clientOption option.ClientOption,
	workingRegion string) (*BigqueryController,
	error) {
	client, err := bigquery.NewClient(ctx, projectId, clientOption)
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

func (c *BigqueryController) ReadCollectionsFromGcs(ctx context.Context,
	gcsFolderName string, bucketName string,
	collections []string) error {
	for _, collectionName := range collections {
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
			return err
		}
		if status.State == bigquery.Done {
			slog.Info("GCSからbqの一時テーブルまでデータの読込が完了")
		} else {
			slog.Info("GCSからbqの一時テーブルまでデータの読込: %v", "state", status.State)
			return errors.New("failed transfer data from gcs to bigquery temporary table.")
		}

		// 取得する始まりと終わりの日時を求める
		jstNow := utils.JstNow()
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
			slog.Info("number of loaded rows is zero.")
			continue
		}

		switch collectionName {
		case myfirestore.LiveChatHistory:
			query = c.Client.Query("SELECT * FROM `" + c.Client.Project() + "." + DatasetName + "." +
				TemporaryTableName + "` WHERE FORMAT_TIMESTAMP('%F %T', published_at, '+09:00') " +
				"BETWEEN '" + yesterdayStart.Format("2006-01-02 15:04:05") + "' AND '" +
				yesterdayEnd.Format("2006-01-02 15:04:05") + "'")
		case myfirestore.UserActivities:
			query = c.Client.Query("SELECT * FROM `" + c.Client.Project() + "." + DatasetName + "." +
				TemporaryTableName + "` WHERE FORMAT_TIMESTAMP('%F %T', taken_at, '+09:00') " +
				"BETWEEN '" + yesterdayStart.Format("2006-01-02 15:04:05") + "' AND '" +
				yesterdayEnd.Format("2006-01-02 15:04:05") + "'")
		}
		query.Location = c.WorkingRegion
		query.WriteDisposition = bigquery.WriteAppend // 追加
		switch collectionName {
		case myfirestore.LiveChatHistory:
			query.QueryConfig.Dst = dataset.Table(LiveChatHistoryMainTableName)
		case myfirestore.UserActivities:
			query.QueryConfig.Dst = dataset.Table(UserActivityHistoryMainTableName)
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
			slog.Info("bqの一時テーブルからメインテーブルまでデータの移行が完了")
		} else {
			slog.Error("bqの一時テーブルからメインテーブルまでデータの移行結果", "state", status.State)
			return errors.New("failed transfer data from bigquery temporary table to main table.")
		}
	}
	slog.Info("finished all collection's processes.")
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
