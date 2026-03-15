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

type CheckLiveStreamResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

// CheckLiveStream checks the live stream status and, in case of an error, sends a message to the owner.
func CheckLiveStream(ctx context.Context) (CheckLiveStreamResponse, error) {
	slog.Info(utils.NameOf(CheckLiveStream))

	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return CheckLiveStreamResponse{}, fmt.Errorf("in FirestoreClientOption: %w", err)
	}

	app, err := workspaceapp.NewWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// 初期化中のタイムアウト（appがないのでDiscord通知不可）
			return CheckLiveStreamResponse{}, fmt.Errorf("timeout during NewWorkspaceApp: %w", err)
		}
		return CheckLiveStreamResponse{}, fmt.Errorf("in NewWorkspaceApp: %w", err)
	}
	defer app.CloseFirestoreClient()

	if err := app.CheckLiveStreamStatus(gracefulCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// NOTE: gracefulCtxは既にキャンセル済みのため、まだ残り時間のある元のctxを使用
			if notifyErr := app.NotifyTimeoutToOwner(ctx, fmt.Errorf("CheckLiveStreamStatusでタイムアウト: %w", err)); notifyErr != nil {
				return CheckLiveStreamResponse{}, fmt.Errorf("timeout notification failed: %w", notifyErr)
			}
			return CheckLiveStreamResponse{Result: "timeout_warning", Message: err.Error()}, nil
		}
		app.MessageToOwnerWithError(ctx, "failed to check live stream status", err)
		return CheckLiveStreamResponse{}, err
	}

	return CheckLiveStreamResponse{
		Result:  lambdautils.OK,
		Message: "",
	}, nil
}

func main() {
	lambda.Start(CheckLiveStream)
}
