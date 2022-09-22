package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type OrganizeDatabaseResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func OrganizeDatabase() (OrganizeDatabaseResponseStruct, error) {
	log.Println("OrganizeDatabase()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return OrganizeDatabaseResponseStruct{}, nil
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return OrganizeDatabaseResponseStruct{}, nil
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.OrganizeDB(ctx)
	if err != nil {
		_system.MessageToLineBotWithError("failed to OrganizeDB", err)
		return OrganizeDatabaseResponseStruct{}, nil
	}
	
	return OrganizeDatabaseResponse(), nil
}

func OrganizeDatabaseResponse() OrganizeDatabaseResponseStruct {
	var apiResp OrganizeDatabaseResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(OrganizeDatabase)
}
