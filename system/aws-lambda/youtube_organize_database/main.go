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

type organizeDatabaseApp interface {
	OrganizeDB(ctx context.Context, isMemberRoom bool) error
	MessageToOwnerWithError(ctx context.Context, message string, err error)
	CloseFirestoreClient()
}

type OrganizeDatabaseResponse struct {
	Result  string `json:"result"`
	Message string `json:"message"`
}

var (
	// Unit test で初期化失敗や timeout 分岐を差し替え検証できるようにしている。
	firestoreClientOption = lambdautils.FirestoreClientOption
	newWorkspaceApp       = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (organizeDatabaseApp, error) {
		return workspaceapp.NewWorkspaceApp(ctx, isTest, clientOption)
	}
)

func okResponse() OrganizeDatabaseResponse {
	return OrganizeDatabaseResponse{Result: lambdautils.OK, Message: ""}
}

func OrganizeDatabase(ctx context.Context) (OrganizeDatabaseResponse, error) {
	slog.Info(utils.NameOf(OrganizeDatabase))

	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	clientOption, err := firestoreClientOption()
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Firestore client option",
			"err", err,
		)
		return okResponse(), nil
	}

	app, err := newWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get WorkspaceApp",
			"err", err,
		)
		return okResponse(), nil
	}
	defer app.CloseFirestoreClient()

	if timedOut := runOrganizeDBRoom(ctx, gracefulCtx, app, true, "member room"); timedOut {
		return okResponse(), nil
	}
	if timedOut := runOrganizeDBRoom(ctx, gracefulCtx, app, false, "general room"); timedOut {
		return okResponse(), nil
	}
	return okResponse(), nil
}

func runOrganizeDBRoom(ctx context.Context, gracefulCtx context.Context, app organizeDatabaseApp, isMemberRoom bool, roomLabel string) bool {
	err := app.OrganizeDB(gracefulCtx, isMemberRoom)
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		slog.ErrorContext(ctx, "timeout warning in youtube_organize_database during OrganizeDB", "room", roomLabel, "err", err)
		return true
	}

	slog.ErrorContext(ctx, "failed to OrganizeDB", "room", roomLabel, "err", err)
	app.MessageToOwnerWithError(ctx, fmt.Sprintf("failed to OrganizeDB (%s)", roomLabel), err)
	return false
}

func main() {
	lambda.Start(OrganizeDatabase)
}
