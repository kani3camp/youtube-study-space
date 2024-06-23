package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"app.modules/core/utils"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pkg/errors"
	"log/slog"
	"net/http"
)

type SetMaxSeatsParams struct {
	DesiredMaxSeats       int `json:"desired_max_seats"`
	DesiredMemberMaxSeats int `json:"desired_member_max_seats"`
}

type SetMaxSeatsResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func SetDesiredMaxSeats(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	slog.Info(utils.NameOf(SetDesiredMaxSeats))

	var params SetMaxSeatsParams
	err := json.Unmarshal([]byte(request.Body), &params)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	system, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	defer system.CloseFirestoreClient()

	if system.Configs.Constants.YoutubeMembershipEnabled {
		if params.DesiredMaxSeats <= 0 || params.DesiredMemberMaxSeats <= 0 {
			return events.APIGatewayProxyResponse{}, errors.New("invalid parameter")
		}
	} else {
		if params.DesiredMaxSeats <= 0 || params.DesiredMemberMaxSeats != 0 {
			return events.APIGatewayProxyResponse{}, errors.New("invalid parameter")
		}
	}

	// transaction not necessary
	err = system.FirestoreController.UpdateDesiredMaxSeats(ctx, nil, params.DesiredMaxSeats)
	if err != nil {
		system.MessageToOwnerWithError("failed UpdateDesiredMaxSeats", err)
		return events.APIGatewayProxyResponse{}, err
	}
	err = system.FirestoreController.UpdateDesiredMemberMaxSeats(ctx, nil, params.DesiredMemberMaxSeats)
	if err != nil {
		system.MessageToOwnerWithError("failed UpdateDesiredMemberMaxSeats", err)
		return events.APIGatewayProxyResponse{}, err
	}

	body, _ := json.Marshal(SetMaxSeatsResponse{
		Result:  lambdautils.OK,
		Message: "",
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
	lambda.Start(SetDesiredMaxSeats)
}
