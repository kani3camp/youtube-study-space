package mybigquery

import (
	"app.modules/core/myfirestore"
	"app.modules/core/utils"
	"cloud.google.com/go/bigquery"
	"context"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"log"
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
		return nil, err
	}
	
	return &BigqueryController{
		Client:        client,
		WorkingRegion: workingRegion,
	}, nil
}

func (c *BigqueryController) CloseClient() {
	err := c.Client.Close()
	if err != nil {
		log.Println("failed to close bigquery client.")
	} else {
		log.Println("successfully closed bigquery client.")
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
			return err
		}
		status, err := job.Wait(ctx)
		if err != nil {
			return err
		}
		if err = status.Err(); err != nil {
			return err
		}
		if status.State == bigquery.Done {
			log.Println("GCSからbqの一時テーブルまでデータの読込が完了")
		} else {
			log.Println("GCSからbqの一時テーブルまでデータの読込: ")
			log.Println(status.State)
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
			return err
		}
		numRows, err := iteratorSize(it)
		if err != nil {
			return err
		}
		if numRows == 0 {
			log.Println("number of loaded rows is zero.")
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
			return err
		}
		status, err = job.Wait(ctx)
		if err != nil {
			return err
		}
		if err = status.Err(); err != nil {
			return err
		}
		if status.State == bigquery.Done {
			log.Println("bqの一時テーブルからメインテーブルまでデータの移行が完了")
		} else {
			log.Println("bqの一時テーブルからメインテーブルまでデータの移行: ")
			log.Println(status.State)
			return errors.New("failed transfer data from bigquery temporary table to main table.")
		}
	}
	log.Println("finished all collection's processes.")
	return nil
}

func iteratorSize(it *bigquery.RowIterator) (int, error) {
	i := 0
	for {
		var row []bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return -1, err
		}
		i++
	}
	return i, nil
}
