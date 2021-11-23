package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"app.modules/core/myfirestore"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type RoomsResponseStruct struct {
	Result  string       `json:"result"`
	Message string                    `json:"message"`
	DefaultRoom   myfirestore.RoomDoc `json:"default_room"`
}

func Rooms(_ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Rooms()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return lambdautils.ErrorResponse(err)
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return lambdautils.ErrorResponse(err)
	}
	defer _system.CloseFirestoreClient()
	
	defaultRoom, err := _system.FirestoreController.RetrieveRoom(ctx)
	if err != nil {
		return lambdautils.ErrorResponse(err)
	}
	
	return RoomsResponse(defaultRoom)
}

func RoomsResponse(defaultRoom myfirestore.RoomDoc) (events.APIGatewayProxyResponse, error) {
	var apiResp RoomsResponseStruct
	apiResp.Result = lambdautils.OK
	apiResp.DefaultRoom = defaultRoom
	jsonBytes, _ := json.Marshal(apiResp)
	return lambdautils.Response(jsonBytes)
}

func main() {
	lambda.Start(Rooms)
}
