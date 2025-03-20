package main

import (
	"app.modules/core/workspaceapp"
	"context"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"github.com/aws/aws-lambda-go/lambda"
)

type CheckLiveStreamResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

// CheckLiveStream checks the live stream status and, in case of an error, sends a message to the owner.
func CheckLiveStream() (CheckLiveStreamResponse, error) {
	slog.Info(utils.NameOf(CheckLiveStream))

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return CheckLiveStreamResponse{}, err
	}
	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		return CheckLiveStreamResponse{}, err
	}
	defer app.CloseFirestoreClient()

	if err := app.CheckLiveStreamStatus(ctx); err != nil {
		app.MessageToOwnerWithError(ctx, "failed to check live stream status", err)
		return CheckLiveStreamResponse{}, err
	}

	return CheckLiveStreamResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(CheckLiveStream)
}
