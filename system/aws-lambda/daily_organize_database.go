package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type DailyOrganizeDatabaseResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func DailyOrganizeDatabase() (DailyOrganizeDatabaseResponseStruct, error) {
	log.Println("ResetDailyTotalStudyTime()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return DailyOrganizeDatabaseResponseStruct{}, err
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return DailyOrganizeDatabaseResponseStruct{}, err
	}
	defer _system.CloseFirestoreClient()
	
	err = _system.DailyOrganizeDatabase(ctx)
	if err != nil {
		_ = _system.MessageToLineBotWithError("failed to DailyOrganizeDatabase", err)
		return DailyOrganizeDatabaseResponseStruct{}, err
	}
	
	return DailyOrganizeDatabaseResponse(), nil
}

func DailyOrganizeDatabaseResponse() DailyOrganizeDatabaseResponseStruct {
	var apiResp DailyOrganizeDatabaseResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(DailyOrganizeDatabase)
}
