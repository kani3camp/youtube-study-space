package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type CheckLiveStreamResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

// CheckLiveStream checks the live stream status and, in case of an error, sends a message to the owner.
func CheckLiveStream() (CheckLiveStreamResponse, error) {
	log.Println("CheckLiveStream()")

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

	err = system.CheckLiveStreamStatus(ctx)
	if err != nil {
		system.MessageToOwnerWithError("failed to check live stream status", err)
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
