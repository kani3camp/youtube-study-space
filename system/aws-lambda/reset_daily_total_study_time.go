package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type ResetDailyTotalStudyTimeResponseStruct struct {
	Result  string       `json:"result"`
	Message string       `json:"message"`
}

func ResetDailyTotalStudyTime(_ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("ResetDailyTotalStudyTime()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return lambdautils.ErrorResponse(err)
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return lambdautils.ErrorResponse(err)
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed to reset daily total time", err)
		return lambdautils.ErrorResponse(err)
	}
	
	return ResetDailyTotalStudyTimeResponse()
}

func ResetDailyTotalStudyTimeResponse() (events.APIGatewayProxyResponse, error) {
	var apiResp ResetDailyTotalStudyTimeResponseStruct
	apiResp.Result = lambdautils.OK
	jsonBytes, _ := json.Marshal(apiResp)
	return lambdautils.Response(jsonBytes)
}

func main() {
	lambda.Start(ResetDailyTotalStudyTime)
}
