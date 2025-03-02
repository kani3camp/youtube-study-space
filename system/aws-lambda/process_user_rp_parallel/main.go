package main

import (
	"app.modules/core/workspaceapp"
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambda2 "github.com/aws/aws-sdk-go/service/lambda"
)

type ProcessUserRPParallelResponseStruct struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func ProcessUserRPParallel(request lambdautils.UserRPParallelRequest) (ProcessUserRPParallelResponseStruct, error) {
	slog.Info(utils.NameOf(ProcessUserRPParallel))

	ctx := context.Background()
	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return ProcessUserRPParallelResponseStruct{}, err
	}
	sys, err := workspaceapp.NewSystem(ctx, false, clientOption)
	if err != nil {
		return ProcessUserRPParallelResponseStruct{}, err
	}
	defer sys.CloseFirestoreClient()

	slog.Info("process index: " + strconv.Itoa(request.ProcessIndex))
	remainingUserIds := sys.UpdateUserRPBatch(ctx, request.UserIds, lambdautils.InterruptTimeLimitSec)

	// 残っているならば次を呼び出す
	if len(remainingUserIds) > 0 {
		sys.MessageToOwner(ctx, strconv.Itoa(len(remainingUserIds))+"個のユーザーが未処理のため、次のlambdaを呼び出します。")

		sess, err := session.NewSession()
		if err != nil {
			sys.MessageToOwnerWithError(ctx, "failed to session.NewSession()", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		svc := lambda2.New(sess)
		payload := lambdautils.UserRPParallelRequest{
			ProcessIndex: request.ProcessIndex,
			UserIds:      remainingUserIds,
		}
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			sys.MessageToOwnerWithError(ctx, "failed to json.Marshal(payload)", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		input := lambda2.InvokeInput{
			FunctionName:   aws.String("process_user_rp_parallel"),
			InvocationType: aws.String(lambda2.InvocationTypeEvent),
			Payload:        jsonBytes,
		}
		resp, err := svc.Invoke(&input)
		if err != nil {
			sys.MessageToOwnerWithError(ctx, "failed to svc.Invoke(&input)", err)
			return ProcessUserRPParallelResponseStruct{}, err
		}
		slog.Info("lambda invoked.", "output", resp)
	} else {
		sys.MessageToOwner(ctx, "batch process (index: "+strconv.Itoa(request.ProcessIndex)+") completed.👍")
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
