package main

import (
	"app.modules/aws-lambda/mydynamodb"
	"app.modules/system"
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
	credentialBytes, err := mydynamodb.RetrieveFirebaseCredentialInBytes()
	if err != nil {
		return ErrorResponse(err)
	}
	clientOption := FirestoreClientOption(credentialBytes)
	_system, err := system.NewSystem(ctx, clientOption)
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
