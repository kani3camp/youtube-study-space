package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type OrganizeDatabaseResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func OrganizeDatabase() (OrganizeDatabaseResponse, error) {
	log.Println("OrganizeDatabase()")

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return OrganizeDatabaseResponse{}, nil
	}
	system, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		return OrganizeDatabaseResponse{}, nil
	}
	defer system.CloseFirestoreClient()

	err = system.OrganizeDB(ctx, true)
	if err != nil {
		system.MessageToOwnerWithError("failed to OrganizeDB", err)
		return OrganizeDatabaseResponse{}, nil
	}
	err = system.OrganizeDB(ctx, false)
	if err != nil {
		system.MessageToOwnerWithError("failed to OrganizeDB", err)
		return OrganizeDatabaseResponse{}, nil
	}

	return OrganizeDatabaseResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(OrganizeDatabase)
}
