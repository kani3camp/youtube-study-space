package main

import (
	"github.com/aws/aws-lambda-go/events"
	"log"
)

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
