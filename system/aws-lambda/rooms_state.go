package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"app.modules/core/myfirestore"
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type RoomsResponseStruct struct {
	Result         string                `json:"result"`
	Message        string                `json:"message"`
	Seats          []myfirestore.SeatDoc `json:"seats"`
	MaxSeats       int                   `json:"max_seats"`
	MinVacancyRate float32               `json:"min_vacancy_rate"`
}

func Rooms() (RoomsResponseStruct, error) {
	log.Println("Rooms()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return RoomsResponseStruct{}, err
	}
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return RoomsResponseStruct{}, err
	}
	defer sys.CloseFirestoreClient()
	
	var constants myfirestore.ConstantsConfigDoc
	seats, err := sys.FirestoreController.ReadAllSeats(ctx)
	if err != nil {
		return RoomsResponseStruct{}, err
	}
	constants, err = sys.FirestoreController.ReadSystemConstantsConfig(ctx, nil)
	if err != nil {
		return RoomsResponseStruct{}, err
	}
	
	return RoomsResponse(seats, constants.MaxSeats, constants.MinVacancyRate), nil
}

func RoomsResponse(seats []myfirestore.SeatDoc, maxSeats int, minVacancyRate float32) RoomsResponseStruct {
	var apiResp RoomsResponseStruct
	apiResp.Result = lambdautils.OK
	apiResp.Seats = seats
	apiResp.MaxSeats = maxSeats
	apiResp.MinVacancyRate = minVacancyRate
	return apiResp
}

func main() {
	lambda.Start(Rooms)
}
