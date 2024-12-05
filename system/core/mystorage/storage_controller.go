package mystorage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/api/iterator"

	"app.modules/core/utils"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type StorageController struct {
	Client        *storage.Client
	WorkingRegion string
}

func NewStorageClient(ctx context.Context, clientOption option.ClientOption,
	workingRegion string) (*StorageController, error) {
	client, err := storage.NewClient(ctx, clientOption)
	if err != nil {
		return nil, fmt.Errorf("in storage.NewClient: %w", err)
	}
	return &StorageController{
		Client:        client,
		WorkingRegion: workingRegion,
	}, nil
}

func (controller *StorageController) CloseClient() {
	if err := controller.Client.Close(); err != nil {
		slog.Error("failed to close cloud storage client.")
	} else {
		slog.Info("successfully closed cloud storage client.")
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
	obj, err := it.Next()
	if errors.Is(err, iterator.Done) {
		return "", errors.New("there is no object whose name begins with " + searchPrefix)
	}
	if err != nil {
		return "", fmt.Errorf("in it.Next: %w", err)
	}
	return strings.Split(obj.Name, "/")[0], nil
}
