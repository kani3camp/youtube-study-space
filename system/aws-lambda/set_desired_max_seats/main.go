package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp"
	"app.modules/internal/logging"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	logging.InitLogger()
}

type SetMaxSeatsParams struct {
	DesiredMaxSeats       int `json:"desired_max_seats"`
	DesiredMemberMaxSeats int `json:"desired_member_max_seats"`
}

type SetMaxSeatsResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func SetDesiredMaxSeats(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	slog.Info(utils.NameOf(SetDesiredMaxSeats))

	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	var params SetMaxSeatsParams
	if err := json.Unmarshal([]byte(request.Body), &params); err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	app, err := workspaceapp.NewWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return events.APIGatewayProxyResponse{}, fmt.Errorf("timeout during NewWorkspaceApp: %w", err)
		}
		return events.APIGatewayProxyResponse{}, err
	}
	defer app.CloseFirestoreClient()

	if app.Configs.Constants.YoutubeMembershipEnabled {
		if params.DesiredMaxSeats <= 0 || params.DesiredMemberMaxSeats <= 0 {
			body, _ := json.Marshal(SetMaxSeatsResponse{Result: "error", Message: "invalid parameter"})
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Access-Control-Allow-Origin": "*"},
				Body:       string(body),
			}, nil
		}
	} else {
		if params.DesiredMaxSeats <= 0 || params.DesiredMemberMaxSeats != 0 {
			body, _ := json.Marshal(SetMaxSeatsResponse{Result: "error", Message: "invalid parameter"})
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Access-Control-Allow-Origin": "*"},
				Body:       string(body),
			}, nil
		}
	}

	// transaction not necessary
	if err := app.Repository.UpdateDesiredMaxSeats(gracefulCtx, nil, params.DesiredMaxSeats); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// NOTE: gracefulCtxは既にキャンセル済みのため、まだ残り時間のある元のctxを使用
			if notifyErr := app.NotifyTimeoutToOwner(ctx, fmt.Errorf("UpdateDesiredMaxSeatsでタイムアウト: %w", err)); notifyErr != nil {
				return events.APIGatewayProxyResponse{}, fmt.Errorf("timeout notification failed: %w", notifyErr)
			}
			body, _ := json.Marshal(SetMaxSeatsResponse{Result: "timeout_warning", Message: err.Error()})
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusOK,
				Headers:    map[string]string{"Access-Control-Allow-Origin": "*"},
				Body:       string(body),
			}, nil
		}
		app.MessageToOwnerWithError(ctx, "failed UpdateDesiredMaxSeats", err)
		return events.APIGatewayProxyResponse{}, err
	}

	if err := app.Repository.UpdateDesiredMemberMaxSeats(gracefulCtx, nil, params.DesiredMemberMaxSeats); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// NOTE: gracefulCtxは既にキャンセル済みのため、まだ残り時間のある元のctxを使用
			if notifyErr := app.NotifyTimeoutToOwner(ctx, fmt.Errorf("UpdateDesiredMemberMaxSeatsでタイムアウト: %w", err)); notifyErr != nil {
				return events.APIGatewayProxyResponse{}, fmt.Errorf("timeout notification failed: %w", notifyErr)
			}
			body, _ := json.Marshal(SetMaxSeatsResponse{Result: "timeout_warning", Message: err.Error()})
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusOK,
				Headers:    map[string]string{"Access-Control-Allow-Origin": "*"},
				Body:       string(body),
			}, nil
		}
		app.MessageToOwnerWithError(ctx, "failed UpdateDesiredMemberMaxSeats", err)
		return events.APIGatewayProxyResponse{}, err
	}

	body, _ := json.Marshal(SetMaxSeatsResponse{
		Result:  lambdautils.OK,
		Message: "",
	})

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Access-Control-Allow-Origin": "*",
		},
		Body:            string(body),
		IsBase64Encoded: false,
	}, nil
}

func main() {
	lambda.Start(SetDesiredMaxSeats)
}
