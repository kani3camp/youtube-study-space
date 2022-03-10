package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type CheckLiveStreamResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func CheckLiveStream() (CheckLiveStreamResponseStruct, error) {
	log.Println("CheckLiveStream()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return CheckLiveStreamResponseStruct{}, err
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return CheckLiveStreamResponseStruct{}, err
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.CheckLiveStreamStatus(ctx)
	if err != nil {
		_ = _system.MessageToLineBotWithError("failed to check live stream", err)
		return CheckLiveStreamResponseStruct{}, err
	}
	
	return CheckLiveStreamResponse(), nil
}

func CheckLiveStreamResponse() CheckLiveStreamResponseStruct {
	var apiResp CheckLiveStreamResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(CheckLiveStream)
}
