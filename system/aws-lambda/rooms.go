package aws_lambda

import (
	"../app-engine/system"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"log"
)

type RoomsResponseStruct struct {
	Result  string       `json:"result"`
	Message string       `json:"message"`
	Rooms   []RoomStruct `json:"rooms"`
}

func Rooms(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println("Rooms()")
	ctx, client := InitializeHttpFuncWithFirestore()
	defer CloseFirestoreClient(client)

	var apiResp RoomsResponseStruct

	rooms, _ := RetrieveRooms(client, ctx)
	apiResp.Result = OK
	apiResp.Rooms = rooms

	jsonBytes, _ := json.Marshal(apiResp)
	return Response(jsonBytes)
}

func main() {
	lambda.Start(Rooms)
}
