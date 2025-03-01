package main

import (
	"context"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
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
	system, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		return CheckLiveStreamResponse{}, err
	}
	defer system.CloseFirestoreClient()

	if err := system.CheckLiveStreamStatus(ctx); err != nil {
		system.MessageToOwnerWithError(ctx, "failed to check live stream status", err)
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
