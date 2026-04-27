package main

import (
	"context"
	"errors"
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
		slog.ErrorContext(ctx, "failed to get Firestore client option for check_live_stream_status",
			"err", err,
		)
		return okResponse(), nil
	}

	app, err := newCheckWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "failed to init WorkspaceApp for check_live_stream_status",
			"err", err,
		)
		return okResponse(), nil
	}
	defer app.CloseFirestoreClient()

	if err := app.CheckLiveStreamStatus(gracefulCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			slog.ErrorContext(ctx, "timeout warning in check_live_stream_status during CheckLiveStreamStatus", "err", err)
			return okResponse(), nil
		}
		slog.ErrorContext(ctx, "failed to check live stream status", "err", err)
		app.MessageToOwnerWithError(ctx, "failed to check live stream status", err)
		return okResponse(), nil
	}

	return okResponse(), nil
}

func main() {
	lambda.Start(CheckLiveStream)
}
