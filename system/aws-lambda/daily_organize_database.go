package main

import (
	"app.modules/aws-lambda/lambdautils"
	"app.modules/core"
	"app.modules/core/utils"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
	"log"
	"strconv"
)

type DailyOrganizeDatabaseResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func DailyOrganizeDatabase() (DailyOrganizeDatabaseResponseStruct, error) {
	log.Println("DailyOrganizeDatabase()")
	
	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return DailyOrganizeDatabaseResponseStruct{}, err
	}
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return DailyOrganizeDatabaseResponseStruct{}, err
	}
	defer sys.CloseFirestoreClient()
	
	userIdsToProcess, err := sys.DailyOrganizeDB(ctx)
	if err != nil {
		sys.MessageToLineBotWithError("failed to DailyOrganizeDB", err)
		return DailyOrganizeDatabaseResponseStruct{}, err
	}
	
	sess, err := session.NewSession()
	if err != nil {
		sys.MessageToLineBotWithError("failed to lambda2.New(session.NewSession())", err)
		return DailyOrganizeDatabaseResponseStruct{}, err
	}
	svc := lambda2.New(sess)
	
	allBatch := utils.DivideStringEqually(sys.Configs.Constants.NumberOfParallelLambdaToProcessUserRP, userIdsToProcess)
	sys.MessageToLineBot(strconv.Itoa(len(userIdsToProcess)) + "人のRP処理を" + strconv.Itoa(len(allBatch)) + "つに分けて並行で処理。")
	for i, batch := range allBatch {
		log.Println("batch No. " + strconv.Itoa(i+1))
		log.Println(batch)
		payload := lambdautils.ProcessUserRPParallelRequestStruct{
			ProcessIndex: i,
			UserIds:      batch,
		}
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			sys.MessageToLineBotWithError("failed to json.Marshal(payload)", err)
			return DailyOrganizeDatabaseResponseStruct{}, err
		}
		input := lambda2.InvokeInput{
			FunctionName:   aws.String("process_user_rp_parallel"),
			InvocationType: aws.String(lambda2.InvocationTypeEvent),
			Payload:        jsonBytes,
		}
		resp, err := svc.Invoke(&input)
		if err != nil {
			sys.MessageToLineBotWithError("failed to svc.Invoke(&input)", err)
			return DailyOrganizeDatabaseResponseStruct{}, err
		}
		log.Println(resp)
	}
	
	return DailyOrganizeDatabaseResponse(), nil
}

func DailyOrganizeDatabaseResponse() DailyOrganizeDatabaseResponseStruct {
	var apiResp DailyOrganizeDatabaseResponseStruct
	apiResp.Result = lambdautils.OK
	return apiResp
}

func main() {
	lambda.Start(DailyOrganizeDatabase)
}
