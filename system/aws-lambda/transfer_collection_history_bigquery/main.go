package main

import (
	"context"
	"log"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"github.com/aws/aws-lambda-go/lambda"
)

type TransferCollectionHistoryBigqueryResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func TransferCollectionHistoryBigquery() (TransferCollectionHistoryBigqueryResponse, error) {
	log.Println("TransferCollectionHistoryBigquery()")

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return TransferCollectionHistoryBigqueryResponse{}, nil
	}
	sys, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		return TransferCollectionHistoryBigqueryResponse{}, nil
	}
	defer sys.CloseFirestoreClient()

	err = sys.BackupCollectionHistoryFromGcsToBigquery(ctx, clientOption)
	if err != nil {
		sys.MessageToOwnerWithError("failed to transfer each collection history to bigquery", err)
		return TransferCollectionHistoryBigqueryResponse{}, nil
	}
	sys.MessageToOwner("successfully transferred each collection history to bigquery")

	return TransferCollectionHistoryBigqueryResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(TransferCollectionHistoryBigquery)
}
