package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"cloud.google.com/go/firestore"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pkg/errors"
	"log"
)

type SetMaxSeatsParams struct {
	DesiredMaxSeats int `json:"desired_max_seats"`
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
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return SetMaxSeatsResponseStruct{}, err
	}
	defer _system.CloseFirestoreClient()
	
	if request.DesiredMaxSeats <= 0 {
		return SetMaxSeatsResponseStruct{}, errors.New("invalid parameter")
	}
	
	err = _system.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		err := _system.FirestoreController.SetDesiredMaxSeats(tx, request.DesiredMaxSeats)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
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
