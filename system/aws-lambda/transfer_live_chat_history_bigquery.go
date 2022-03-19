package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type TransferLiveChatHistoryBigqueryResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func TransferLiveChatHistoryBigquery() (TransferLiveChatHistoryBigqueryResponseStruct, error) {
	log.Println("TransferLiveChatHistoryBigquery()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return TransferLiveChatHistoryBigqueryResponseStruct{}, nil
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return TransferLiveChatHistoryBigqueryResponseStruct{}, nil
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.BackupLiveChatHistoryFromGcsToBigquery(ctx, clientOption)
	if err != nil {
		_ = _system.MessageToLineBotWithError("failed to transfer live chat history to bigquery", err)
		return TransferLiveChatHistoryBigqueryResponseStruct{}, nil
	}
	
	return TransferLiveChatHistoryBigqueryResponse(), nil
}

func TransferLiveChatHistoryBigqueryResponse() TransferLiveChatHistoryBigqueryResponseStruct {
	var apiResp TransferLiveChatHistoryBigqueryResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(TransferLiveChatHistoryBigquery)
}
