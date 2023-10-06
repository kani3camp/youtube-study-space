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

type DailyOrganizeDatabaseResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func DailyOrganizeDatabase() (DailyOrganizeDatabaseResponse, error) {
	log.Println("DailyOrganizeDatabase()")

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return DailyOrganizeDatabaseResponse{}, err
	}
	system, err := core.NewSystem(ctx, false, clientOption)
	if err != nil {
		return DailyOrganizeDatabaseResponse{}, err
	}
	defer system.CloseFirestoreClient()

	userIdsToProcess, err := system.DailyOrganizeDB(ctx)
	if err != nil {
		system.MessageToOwnerWithError("Failed to DailyOrganizeDB", err)
		return DailyOrganizeDatabaseResponse{}, err
	}

	sess, err := session.NewSession()
	if err != nil {
		system.MessageToOwnerWithError("failed to lambda2.New(session.NewSession())", err)
		return DailyOrganizeDatabaseResponse{}, err
	}
	svc := lambda2.New(sess)

	allBatch := utils.DivideStringEqually(system.Configs.Constants.NumberOfParallelLambdaToProcessUserRP, userIdsToProcess)
	system.MessageToOwner(strconv.Itoa(len(userIdsToProcess)) + "人のRP処理を" + strconv.Itoa(len(allBatch)) + "つに分けて並行で処理。")
	for i, batch := range allBatch {
		log.Println("batch No. " + strconv.Itoa(i+1))
		log.Println(batch)

		payload := lambdautils.UserRPParallelRequest{
			ProcessIndex: i,
			UserIds:      batch,
		}
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			system.MessageToOwnerWithError("failed to json.Marshal(payload)", err)
			return DailyOrganizeDatabaseResponse{}, err
		}
		input := lambda2.InvokeInput{
			FunctionName:   aws.String("process_user_rp_parallel"),
			InvocationType: aws.String(lambda2.InvocationTypeEvent),
			Payload:        jsonBytes,
		}
		resp, err := svc.Invoke(&input)
		if err != nil {
			system.MessageToOwnerWithError("failed to svc.Invoke(&input)", err)
			return DailyOrganizeDatabaseResponse{}, err
		}
		log.Println(resp)
	}

	return DailyOrganizeDatabaseResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(DailyOrganizeDatabase)
}
