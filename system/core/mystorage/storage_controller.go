package mystorage

import (
	"app.modules/core/utils"
	"cloud.google.com/go/storage"
	"context"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"log"
	"strings"
)

type StorageController struct {
	Client        *storage.Client
	WorkingRegion string
}

func NewStorageClient(ctx context.Context, clientOption option.ClientOption,
	workingRegion string) (*StorageController, error) {
	client, err := storage.NewClient(ctx, clientOption)
	if err != nil {
		return nil, err
	}
	return &StorageController{
		Client:        client,
		WorkingRegion: workingRegion,
	}, nil
}

func (controller *StorageController) CloseClient() {
	err := controller.Client.Close()
	if err != nil {
		log.Println("failed to close cloud storage client.")
	} else {
		log.Println("successfully closed cloud storage client.")
	}
}

func (controller *StorageController) GetGcsYesterdayExportFolderName(ctx context.Context, bucketName string) (string,
	error) {
	jstNow := utils.JstNow()
	yesterday := jstNow.AddDate(0, 0, -1)
	searchPrefix := yesterday.Format("2006-01-02")
	query := &storage.Query{
		Prefix: searchPrefix,
	}
	bucket := controller.Client.Bucket(bucketName)
	it := bucket.Objects(ctx, query)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", err
		}
		return strings.Split(obj.Name, "/")[0], nil
	}
	return "", errors.New("there is no object whose name begins with " + searchPrefix)
}
