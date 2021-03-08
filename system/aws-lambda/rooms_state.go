package main

import (
	"app.modules/aws-lambda/mydynamodb"
	"app.modules/system"
	"app.modules/system/myfirestore"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type RoomsResponseStruct struct {
	Result  string       `json:"result"`
	Message string       `json:"message"`
	DefaultRoom   myfirestore.DefaultRoomDoc `json:"default_room"`
	NoSeatRoom myfirestore.NoSeatRoomDoc `json:"no_seat_room"`
	DefaultRoomLayout myfirestore.RoomLayoutDoc `json:"default_room_layout"`
}

func Rooms(_ events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Rooms()")
	
	ctx := context.Background()
	credentialBytes, err := mydynamodb.RetrieveFirebaseCredentialInBytes()
	if err != nil {
		return ErrorResponse(err)
	}
	clientOption := FirestoreClientOption(credentialBytes)
	_system, err := system.NewSystem(ctx, clientOption)
	if err != nil {
		return ErrorResponse(err)
	}
	defer _system.CloseFirestoreClient()
	
	defaultRoom, err := _system.FirestoreController.RetrieveDefaultRoom(ctx)
	if err != nil {
		return ErrorResponse(err)
	}
	
	noSeatRoom, err := _system.FirestoreController.RetrieveNoSeatRoom(ctx)
	if err != nil {
		return ErrorResponse(err)
	}
	
	defaultRoomLayout, err := _system.FirestoreController.RetrieveDefaultRoomLayout(ctx)
	if err != nil {
		return ErrorResponse(err)
	}
	
	return RoomsResponse(defaultRoom, noSeatRoom, defaultRoomLayout)
}

func RoomsResponse(defaultRoom myfirestore.DefaultRoomDoc, noSeatRoom myfirestore.NoSeatRoomDoc, defaultRoomLayout myfirestore.RoomLayoutDoc) (events.APIGatewayProxyResponse, error) {
	var apiResp RoomsResponseStruct
	apiResp.Result = OK
	apiResp.DefaultRoom = defaultRoom
	apiResp.NoSeatRoom = noSeatRoom
	apiResp.DefaultRoomLayout = defaultRoomLayout
	jsonBytes, _ := json.Marshal(apiResp)
	return Response(jsonBytes)
}

func main() {
	lambda.Start(Rooms)
}
