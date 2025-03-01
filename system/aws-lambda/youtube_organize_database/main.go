package main

import (
	"context"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"app.modules/core/utils"
	"github.com/aws/aws-lambda-go/lambda"
)

type OrganizeDatabaseResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func OrganizeDatabase() (OrganizeDatabaseResponse, error) {
	slog.Info(utils.NameOf(OrganizeDatabase))

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

	if err := system.OrganizeDB(ctx, true); err != nil {
		system.MessageToOwnerWithError(ctx, "failed to OrganizeDB", err)
		return OrganizeDatabaseResponse{}, nil
	}
	if err := system.OrganizeDB(ctx, false); err != nil {
		system.MessageToOwnerWithError(ctx, "failed to OrganizeDB", err)
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
