package lambdautils

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
)

type ErrorResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func Response(jsonBytes []byte) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		Body:       string(jsonBytes),
		StatusCode: 200, // Necessary to avoid Internal Server Error on API Gateway
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}

// ResponseError creates an API Gateway proxy response for a given error.
func ResponseError(err error) events.APIGatewayProxyResponse {
	var apiResp ErrorResponse
	fmt.Println(err.Error())
	apiResp.Result = ERROR
	apiResp.Message = err.Error()
	jsonBytes, _ := json.Marshal(apiResp)
	return Response(jsonBytes)
}
