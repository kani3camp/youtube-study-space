package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"app.modules/core/utils"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log/slog"
	"net/http"
)

type DisplayShoutMessageResponse struct {
	UserId          string `json:"user_id"`
	ShoutMessage    string `json:"shout_message"`
	UserDisplayName string `json:"user_display_name"`
}

func DisplayShoutMessage(ctx context.Context, _ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	slog.Info(utils.NameOf(DisplayShoutMessage))

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	system, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	defer system.CloseFirestoreClient()

	shout, err := system.GetShoutMessage(ctx)
	if err != nil {
		system.MessageToOwnerWithError("failed GetShoutMessage", err)
		return events.APIGatewayProxyResponse{}, err
	}
	userDisplayName, err := system.GetYoutubeUserDisplayName(ctx, shout.DocId)
	if err != nil {
		system.MessageToOwnerWithError("failed GetYoutubeUserDisplayName", err)
		return events.APIGatewayProxyResponse{}, err
	}

	body, _ := json.Marshal(DisplayShoutMessageResponse{
		UserId:          shout.DocId,
		ShoutMessage:    shout.Doc.Message,
		UserDisplayName: userDisplayName,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Access-Control-Allow-Origin": "*",
		},
		Body:            string(body),
		IsBase64Encoded: false,
	}, nil
}

func main() {
	lambda.Start(DisplayShoutMessage)
}
