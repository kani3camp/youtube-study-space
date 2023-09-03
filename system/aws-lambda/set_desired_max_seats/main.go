package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pkg/errors"
	"log"
)

type SetMaxSeatsParams struct {
	DesiredMaxSeats       int `json:"desired_max_seats"`
	DesiredMemberMaxSeats int `json:"desired_member_max_seats"`
}

type SetMaxSeatsResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func SetDesiredMaxSeats(request SetMaxSeatsParams) (SetMaxSeatsResponse, error) {
	log.Println("SetDesiredMaxSeats()")

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return SetMaxSeatsResponse{}, err
	}
	system, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		return SetMaxSeatsResponse{}, err
	}
	defer system.CloseFirestoreClient()

	if system.Configs.Constants.YoutubeMembershipEnabled {
		if request.DesiredMaxSeats <= 0 || request.DesiredMemberMaxSeats <= 0 {
			return SetMaxSeatsResponse{}, errors.New("invalid parameter")
		}
	} else {
		if request.DesiredMaxSeats <= 0 || request.DesiredMemberMaxSeats != 0 {
			return SetMaxSeatsResponse{}, errors.New("invalid parameter")
		}
	}

	// transaction not necessary
	err = system.FirestoreController.UpdateDesiredMaxSeats(ctx, nil, request.DesiredMaxSeats)
	if err != nil {
		system.MessageToOwnerWithError("failed UpdateDesiredMaxSeats", err)
		return SetMaxSeatsResponse{}, err
	}
	err = system.FirestoreController.UpdateDesiredMemberMaxSeats(ctx, nil, request.DesiredMemberMaxSeats)
	if err != nil {
		system.MessageToOwnerWithError("failed UpdateDesiredMemberMaxSeats", err)
		return SetMaxSeatsResponse{}, err
	}

	return SetMaxSeatsResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(SetDesiredMaxSeats)
}
