package main

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"log"
)


type ErrorResponseStruct struct {
	Result  string       `json:"result"`
	Message string       `json:"message"`
}

func Response(jsonBytes []byte) (events.APIGatewayProxyResponse, error) {
	log.Println("Response()")
	return events.APIGatewayProxyResponse{
		Body:       string(jsonBytes),
		StatusCode: 200, // これないとInternal Server Errorになる
		Headers: map[string]string{
			"Content-Type": "application/json",
			//"Access-Control-Allow-Origin": "*",
			//"Access-Control-Allow-Methods": "GET,POST,HEAD,OPTIONS",
			//"Access-Control-Allow-Headers": "Content-Type",
		},
	}, nil
}


func ErrorResponse(err error) (events.APIGatewayProxyResponse, error) {
	var apiResp ErrorResponseStruct
	fmt.Println(err.Error())
	apiResp.Result = ERROR
	apiResp.Message = err.Error()
	jsonBytes, _ := json.Marshal(apiResp)
	return Response(jsonBytes)
}
