package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"app.modules/core/myfirestore"
	"cloud.google.com/go/firestore"
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
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return RoomsResponseStruct{}, err
	}
	defer _system.CloseFirestoreClient()
	
	var seats []myfirestore.SeatDoc
	var constants myfirestore.ConstantsConfigDoc
	err = _system.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var err error
		seats, err = _system.FirestoreController.ReadAllSeats(ctx)
		if err != nil {
			return err
		}
		
		constants, err = _system.FirestoreController.ReadSystemConstantsConfig(ctx, tx)
		if err != nil {
			return err
		}
		return nil
	})
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
