package main

import (
	"app.modules/core/workspaceapp"
	"context"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	lambdautils.InitLogger()
}

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
	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		return OrganizeDatabaseResponse{}, nil
	}
	defer app.CloseFirestoreClient()

	if err := app.OrganizeDB(ctx, true); err != nil {
		app.MessageToOwnerWithError(ctx, "failed to OrganizeDB", err)
		return OrganizeDatabaseResponse{}, nil
	}
	if err := app.OrganizeDB(ctx, false); err != nil {
		app.MessageToOwnerWithError(ctx, "failed to OrganizeDB", err)
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
