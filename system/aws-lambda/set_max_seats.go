package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pkg/errors"
	"log"
)

type SetMaxSeatsParams struct {
	MaxSeats int `json:"max_seats"`
}

type SetMaxSeatsResponseStruct struct {
	Result  string       `json:"result"`
	Message string                    `json:"message"`
}

func SetMaxSeats(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
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
	
	// リクエストパラメータ読み込み
	params := SetMaxSeatsParams{}
	_ = json.Unmarshal([]byte(request.Body), &params)
	
	// 有効な値かチェック
	if params.MaxSeats >= 0 {
		err = _system.FirestoreController.SetMaxSeats(params.MaxSeats, ctx)
		if err != nil {
			return lambdautils.ErrorResponse(err)
		}
	} else {
		return lambdautils.ErrorResponse(errors.New("invalid parameter"))
	}
	
	return SetMaxSeatsResponse()
}

func SetMaxSeatsResponse() (events.APIGatewayProxyResponse, error) {
	var apiResp SetMaxSeatsResponseStruct
	apiResp.Result = lambdautils.OK
	jsonBytes, _ := json.Marshal(apiResp)
	return lambdautils.Response(jsonBytes)
}

func main() {
	lambda.Start(SetMaxSeats)
}
