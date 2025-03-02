package main

import (
	"app.modules/core/workspaceapp"
	"context"
	"log/slog"

	"app.modules/core/utils"

	"app.modules/aws-lambda/lambdautils"
	"github.com/aws/aws-lambda-go/lambda"
)

type TransferCollectionHistoryBigqueryResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func TransferCollectionHistoryBigquery() (TransferCollectionHistoryBigqueryResponse, error) {
	slog.Info(utils.NameOf(TransferCollectionHistoryBigquery))

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return TransferCollectionHistoryBigqueryResponse{}, nil
	}
	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		return TransferCollectionHistoryBigqueryResponse{}, nil
	}
	defer app.CloseFirestoreClient()

	if err := app.BackupCollectionHistoryFromGcsToBigquery(ctx, clientOption); err != nil {
		app.MessageToOwnerWithError(ctx, "failed to transfer each collection history to bigquery", err)
		return TransferCollectionHistoryBigqueryResponse{}, nil
	}
	app.MessageToOwner(ctx, "successfully transferred each collection history to bigquery")

	return TransferCollectionHistoryBigqueryResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(TransferCollectionHistoryBigquery)
}
