package main

import (
	"app.modules/core/workspaceapp"
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
)

type DailyOrganizeDatabaseResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func DailyOrganizeDatabase() (DailyOrganizeDatabaseResponse, error) {
	slog.Info(utils.NameOf(DailyOrganizeDatabase))

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return DailyOrganizeDatabaseResponse{}, err
	}
	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		return DailyOrganizeDatabaseResponse{}, err
	}
	defer app.CloseFirestoreClient()

	userIdsToProcess, err := app.DailyOrganizeDB(ctx)
	if err != nil {
		app.MessageToOwnerWithError(ctx, "Failed to DailyOrganizeDB", err)
		return DailyOrganizeDatabaseResponse{}, err
	}

	sess, err := session.NewSession()
	if err != nil {
		app.MessageToOwnerWithError(ctx, "failed to lambda2.New(session.NewSession())", err)
		return DailyOrganizeDatabaseResponse{}, err
	}
	svc := lambda2.New(sess)

	allBatch := utils.DivideStringEqually(app.Configs.Constants.NumberOfParallelLambdaToProcessUserRP, userIdsToProcess)
	app.MessageToOwner(ctx, strconv.Itoa(len(userIdsToProcess))+"人のRP処理を"+strconv.Itoa(len(allBatch))+"つに分けて並行で処理。")
	for i, batch := range allBatch {
		slog.Info("batch No. "+strconv.Itoa(i+1)+".", "batch", batch)

		payload := lambdautils.UserRPParallelRequest{
			ProcessIndex: i,
			UserIds:      batch,
		}
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			app.MessageToOwnerWithError(ctx, "failed to json.Marshal(payload)", err)
			return DailyOrganizeDatabaseResponse{}, err
		}
		input := lambda2.InvokeInput{
			FunctionName:   aws.String("process_user_rp_parallel"),
			InvocationType: aws.String(lambda2.InvocationTypeEvent),
			Payload:        jsonBytes,
		}
		resp, err := svc.Invoke(&input)
		if err != nil {
			app.MessageToOwnerWithError(ctx, "failed to svc.Invoke(&input)", err)
			return DailyOrganizeDatabaseResponse{}, err
		}
		slog.Info("lambda invoked.", "output", resp)
	}

	return DailyOrganizeDatabaseResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(DailyOrganizeDatabase)
}
