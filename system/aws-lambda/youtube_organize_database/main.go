package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp"
	"github.com/aws/aws-lambda-go/lambda"
)

func init() {
	lambdautils.InitLogger()
}

type OrganizeDatabaseResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

func OrganizeDatabase(ctx context.Context) (OrganizeDatabaseResponse, error) {
	slog.Info(utils.NameOf(OrganizeDatabase))

	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return OrganizeDatabaseResponse{}, fmt.Errorf("in FirestoreClientOption: %w", err)
	}

	app, err := workspaceapp.NewWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// 初期化中のタイムアウト（appがないのでDiscord通知不可）
			return OrganizeDatabaseResponse{}, fmt.Errorf("timeout during NewWorkspaceApp: %w", err)
		}
		return OrganizeDatabaseResponse{}, fmt.Errorf("in NewWorkspaceApp: %w", err)
	}
	defer app.CloseFirestoreClient()

	if err := app.OrganizeDB(gracefulCtx, true); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			if notifyErr := app.NotifyTimeoutToOwner(gracefulCtx, fmt.Errorf("OrganizeDB (member room)でタイムアウト: %w", err)); notifyErr != nil {
				return OrganizeDatabaseResponse{}, fmt.Errorf("timeout notification failed: %w", notifyErr)
			}
			return OrganizeDatabaseResponse{Result: "timeout_warning", Message: err.Error()}, nil
		}
		app.MessageToOwnerWithError(gracefulCtx, "failed to OrganizeDB (member room)", err)
		return OrganizeDatabaseResponse{}, nil
	}

	if err := app.OrganizeDB(gracefulCtx, false); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			if notifyErr := app.NotifyTimeoutToOwner(gracefulCtx, fmt.Errorf("OrganizeDB (general room)でタイムアウト: %w", err)); notifyErr != nil {
				return OrganizeDatabaseResponse{}, fmt.Errorf("timeout notification failed: %w", notifyErr)
			}
			return OrganizeDatabaseResponse{Result: "timeout_warning", Message: err.Error()}, nil
		}
		app.MessageToOwnerWithError(gracefulCtx, "failed to OrganizeDB (general room)", err)
		return OrganizeDatabaseResponse{}, nil
	}

	return OrganizeDatabaseResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(OrganizeDatabase)
}
