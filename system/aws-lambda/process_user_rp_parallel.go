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

func ProcessUserRPParallel(request lambdautils.UserRPParallelRequest) (ProcessUserRPParallelResponseStruct, error) {
	log.Println("ProcessUserRPParallel()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return ProcessUserRPParallelResponseStruct{}, err
	}
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return ProcessUserRPParallelResponseStruct{}, err
	}
	defer sys.CloseFirestoreClient()
	
	log.Println("process index: " + strconv.Itoa(request.ProcessIndex))
	remainingUserIds, err := sys.UpdateUserRPBatch(ctx, request.UserIds, lambdautils.InterruptTimeLimitSec)
	if err != nil {
		sys.MessageToOwnerWithError("failed to UpdateUserRPBatch", err)
		return ProcessUserRPParallelResponseStruct{}, err
	}
	
	// 残っているならば次を呼び出す
	if len(remainingUserIds) > 0 {
		sys.MessageToOwner(strconv.Itoa(len(remainingUserIds)) + "個のユーザーが未処理のため、次のlambdaを呼び出します。")
		
		sess, err := session.NewSession()
		if err != nil {
			sys.MessageToOwnerWithError("failed to session.NewSession()", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		svc := lambda2.New(sess)
		payload := lambdautils.UserRPParallelRequest{
			ProcessIndex: request.ProcessIndex,
			UserIds:      remainingUserIds,
		}
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			sys.MessageToOwnerWithError("failed to json.Marshal(payload)", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		input := lambda2.InvokeInput{
			FunctionName:   aws.String("process_user_rp_parallel"),
			InvocationType: aws.String(lambda2.InvocationTypeEvent),
			Payload:        jsonBytes,
		}
		resp, err := svc.Invoke(&input)
		if err != nil {
			sys.MessageToOwnerWithError("failed to svc.Invoke(&input)", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		log.Println(resp)
	} else {
		sys.MessageToOwner("batch process (index: " + strconv.Itoa(request.ProcessIndex) + ") completed.👍")
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
