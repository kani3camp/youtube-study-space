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

type OrganizeDatabaseResponseStruct struct {
	Result  string       `json:"result"`
	Message string       `json:"message"`
}

func OrganizeDatabase(_ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("OrganizeDatabase()")
	
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
	
	err = _system.OrganizeDatabase(ctx)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed to organize database", err)
		return lambdautils.ErrorResponse(err)
	}
	
	return OrganizeDatabaseResponse()
}

func OrganizeDatabaseResponse() (events.APIGatewayProxyResponse, error) {
	var apiResp OrganizeDatabaseResponseStruct
	apiResp.Result = lambdautils.OK
	jsonBytes, _ := json.Marshal(apiResp)
	return lambdautils.Response(jsonBytes)
}

func main() {
	lambda.Start(OrganizeDatabase)
}
