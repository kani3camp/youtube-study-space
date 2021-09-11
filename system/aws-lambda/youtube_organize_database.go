package main

import (
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
	clientOption, err := FirestoreClientOption()
	if err != nil {
		return ErrorResponse(err)
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return ErrorResponse(err)
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.OrganizeDatabase(ctx)
	if err != nil {
		_ = _system.LineBot.SendMessageWithError("failed to organize database", err)
		return ErrorResponse(err)
	}
	
	return OrganizeDatabaseResponse()
}

func OrganizeDatabaseResponse() (events.APIGatewayProxyResponse, error) {
	var apiResp OrganizeDatabaseResponseStruct
	apiResp.Result = OK
	jsonBytes, _ := json.Marshal(apiResp)
	return Response(jsonBytes)
}

func main() {
	lambda.Start(OrganizeDatabase)
}
