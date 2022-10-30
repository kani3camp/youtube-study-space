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

type SetMaxSeatsResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func SetDesiredMaxSeats(request SetMaxSeatsParams) (SetMaxSeatsResponseStruct, error) {
	log.Println("SetDesiredMaxSeats()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return SetMaxSeatsResponseStruct{}, err
	}
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return SetMaxSeatsResponseStruct{}, err
	}
	defer sys.CloseFirestoreClient()
	
	if request.DesiredMaxSeats <= 0 || request.DesiredMemberMaxSeats <= 0 {
		return SetMaxSeatsResponseStruct{}, errors.New("invalid parameter")
	}
	
	// transaction not necessary
	err = sys.FirestoreController.UpdateDesiredMaxSeats(ctx, nil, request.DesiredMaxSeats)
	if err != nil {
		sys.MessageToOwnerWithError("failed UpdateDesiredMaxSeats", err)
		return SetMaxSeatsResponseStruct{}, err
	}
	err = sys.FirestoreController.UpdateDesiredMemberMaxSeats(ctx, nil, request.DesiredMemberMaxSeats)
	if err != nil {
		sys.MessageToOwnerWithError("failed UpdateDesiredMemberMaxSeats", err)
		return SetMaxSeatsResponseStruct{}, err
	}
	
	return SetMaxSeatsResponse(), nil
}

func SetMaxSeatsResponse() SetMaxSeatsResponseStruct {
	var apiResp SetMaxSeatsResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(SetDesiredMaxSeats)
}
