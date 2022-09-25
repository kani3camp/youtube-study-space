package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type TransferCollectionHistoryBigqueryResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func TransferCollectionHistoryBigquery() (TransferCollectionHistoryBigqueryResponseStruct, error) {
	log.Println("TransferCollectionHistoryBigquery()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return TransferCollectionHistoryBigqueryResponseStruct{}, nil
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return TransferCollectionHistoryBigqueryResponseStruct{}, nil
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.BackupCollectionHistoryFromGcsToBigquery(ctx, clientOption)
	if err != nil {
		_system.MessageToOwnerWithError("failed to transfer each collection history to bigquery", err)
		return TransferCollectionHistoryBigqueryResponseStruct{}, nil
	}
	
	return TransferCollectionHistoryBigqueryResponse(), nil
}

func TransferCollectionHistoryBigqueryResponse() TransferCollectionHistoryBigqueryResponseStruct {
	var apiResp TransferCollectionHistoryBigqueryResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(TransferCollectionHistoryBigquery)
}
