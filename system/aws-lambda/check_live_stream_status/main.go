package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp"
	"app.modules/internal/logging"
	"github.com/aws/aws-lambda-go/lambda"
	"google.golang.org/api/option"
)

func init() {
	logging.InitLogger()
}

type CheckLiveStreamResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

type checkLiveStreamApp interface {
	CheckLiveStreamStatus(ctx context.Context) error
	NotifyTimeoutToOwner(ctx context.Context, err error) error
	MessageToOwnerWithError(ctx context.Context, message string, err error)
	CloseFirestoreClient()
}

var (
	// Unit test で初期化失敗や timeout 分岐を差し替え検証できるようにしている。
	firestoreClientOptionCheck = lambdautils.FirestoreClientOption
	newCheckWorkspaceApp       = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (checkLiveStreamApp, error) {
		return workspaceapp.NewWorkspaceApp(ctx, isTest, clientOption)
	}
)

func okResponse() CheckLiveStreamResponse {
	return CheckLiveStreamResponse{Result: lambdautils.OK, Message: ""}
}

// CheckLiveStream checks the live stream status and, in case of an error, sends a message to the owner.
func CheckLiveStream(ctx context.Context) (CheckLiveStreamResponse, error) {
	slog.Info(utils.NameOf(CheckLiveStream))

	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	clientOption, err := firestoreClientOptionCheck()
	if err != nil {
		slog.ErrorContext(ctx, "in FirestoreClientOption",
			"err", err,
		)
		return okResponse(), nil
	}

	app, err := newCheckWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "in NewWorkspaceApp",
			"err", err,
		)
		return okResponse(), nil
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
		return okResponse(), nil
	}

	return okResponse(), nil
}

func main() {
	lambda.Start(CheckLiveStream)
}
