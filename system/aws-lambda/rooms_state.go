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
	Result      string              `json:"result"`
	Message     string              `json:"message"`
	DefaultRoom myfirestore.RoomDoc `json:"default_room"`
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
	
	defaultRoom, err := _system.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return RoomsResponseStruct{}, err
	}
	
	return RoomsResponse(defaultRoom), nil
}

func RoomsResponse(defaultRoom myfirestore.RoomDoc) RoomsResponseStruct {
	var apiResp RoomsResponseStruct
	apiResp.Result = lambdautils.OK
	apiResp.DefaultRoom = defaultRoom
	return apiResp
}

func main() {
	lambda.Start(Rooms)
}
