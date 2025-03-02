package main

import (
	"app.modules/core/workspaceapp"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pkg/errors"
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
	if err := json.Unmarshal([]byte(request.Body), &params); err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	system, err := workspaceapp.NewSystem(ctx, false, clientOption)
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
	if err := system.Repository.UpdateDesiredMaxSeats(ctx, nil, params.DesiredMaxSeats); err != nil {
		system.MessageToOwnerWithError(ctx, "failed UpdateDesiredMaxSeats", err)
		return events.APIGatewayProxyResponse{}, err
	}
	if err := system.Repository.UpdateDesiredMemberMaxSeats(ctx, nil, params.DesiredMemberMaxSeats); err != nil {
		system.MessageToOwnerWithError(ctx, "failed UpdateDesiredMemberMaxSeats", err)
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
