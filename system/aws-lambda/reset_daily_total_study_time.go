package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type ResetDailyTotalStudyTimeResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func ResetDailyTotalStudyTime() (ResetDailyTotalStudyTimeResponseStruct, error) {
	log.Println("ResetDailyTotalStudyTime()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return ResetDailyTotalStudyTimeResponseStruct{}, err
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return ResetDailyTotalStudyTimeResponseStruct{}, err
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		_ = _system.MessageToLineBotWithError("failed to reset daily total time", err)
		return ResetDailyTotalStudyTimeResponseStruct{}, err
	}
	
	return ResetDailyTotalStudyTimeResponse(), nil
}

func ResetDailyTotalStudyTimeResponse() ResetDailyTotalStudyTimeResponseStruct {
	var apiResp ResetDailyTotalStudyTimeResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(ResetDailyTotalStudyTime)
}
