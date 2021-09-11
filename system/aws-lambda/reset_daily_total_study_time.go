package main

import (
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
	clientOption, err := FirestoreClientOption()
	if err != nil {
		return ErrorResponse(err)
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return ErrorResponse(err)
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.ResetDailyTotalStudyTime(ctx)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed to reset daily total time", err)
		return ErrorResponse(err)
	}
	
	return ResetDailyTotalStudyTimeResponse()
}

func ResetDailyTotalStudyTimeResponse() (events.APIGatewayProxyResponse, error) {
	var apiResp ResetDailyTotalStudyTimeResponseStruct
	apiResp.Result = OK
	jsonBytes, _ := json.Marshal(apiResp)
	return Response(jsonBytes)
}

func main() {
	lambda.Start(ResetDailyTotalStudyTime)
}
