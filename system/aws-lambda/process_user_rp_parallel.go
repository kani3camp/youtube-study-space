package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	"log"
	"strconv"
)

type ProcessUserRPParallelResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func ProcessUserRPParallel(request lambdautils.ProcessUserRPParallelRequestStruct) (ProcessUserRPParallelResponseStruct, error) {
	log.Println("ProcessUserRPParallel()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return ProcessUserRPParallelResponseStruct{}, err
	}
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return ProcessUserRPParallelResponseStruct{}, err
	}
	defer _system.CloseFirestoreClient()
	
	remainingUserIds, err := _system.UpdateUserRPBatch(ctx, request.UserIds, lambdautils.InterruptionTimeLimitSeconds)
	if err != nil {
		_system.MessageToLineBotWithError("failed to UpdateUserRPBatch", err)
		return ProcessUserRPParallelResponseStruct{}, err
	}
	
	// 残っているならば次を呼び出す
	if len(remainingUserIds) > 0 {
		log.Println(strconv.Itoa(len(remainingUserIds)) + "個のユーザーが未処理のため、次のlambdaを呼び出します。")
		
		sess, err := session.NewSession()
		if err != nil {
			_system.MessageToLineBotWithError("failed to lambda2.New(session.NewSession())", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		svc := lambda2.New(sess)
		payload := lambdautils.ProcessUserRPParallelRequestStruct{
			UserIds: remainingUserIds,
		}
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return ProcessUserRPParallelResponseStruct{}, err
		}
		input := lambda2.InvokeInput{
			FunctionName:   aws.String("process_user_rp_parallel"),
			InvocationType: aws.String(lambda2.InvocationTypeEvent),
			Payload:        jsonBytes,
		}
		resp, err := svc.Invoke(&input)
		if err != nil {
			_system.MessageToLineBotWithError("failed to svc.Invoke(&input)", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		log.Println(resp)
	} else {
		log.Println("all user's processes in this batch completed.")
	}
	
	return ProcessUserRPParallelResponse(), nil
}

func ProcessUserRPParallelResponse() ProcessUserRPParallelResponseStruct {
	var apiResp ProcessUserRPParallelResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(ProcessUserRPParallel)
}
