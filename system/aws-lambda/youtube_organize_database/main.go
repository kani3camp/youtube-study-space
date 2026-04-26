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
	NotifyTimeoutToOwner(ctx context.Context, err error) error
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

type organizeRoomResult struct {
	err                   error
	timeoutWarningMessage string
	abort                 bool
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

	memberResult := runOrganizeDBRoom(ctx, gracefulCtx, app, true, "member room")
	if memberResult.timeoutWarningMessage != "" {
		return OrganizeDatabaseResponse{Result: "timeout_warning", Message: memberResult.timeoutWarningMessage}, nil
	}
	if memberResult.abort {
		return OrganizeDatabaseResponse{}, memberResult.err
	}
	generalResult := runOrganizeDBRoom(ctx, gracefulCtx, app, false, "general room")
	if generalResult.timeoutWarningMessage != "" {
		return OrganizeDatabaseResponse{Result: "timeout_warning", Message: generalResult.timeoutWarningMessage}, nil
	}
	if generalResult.abort {
		return OrganizeDatabaseResponse{}, generalResult.err
	}
	return okResponse(), nil
}

func runOrganizeDBRoom(ctx context.Context, gracefulCtx context.Context, app organizeDatabaseApp, isMemberRoom bool, roomLabel string) organizeRoomResult {
	err := app.OrganizeDB(gracefulCtx, isMemberRoom)
	if err == nil {
		return organizeRoomResult{}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		timeoutErr := fmt.Errorf("OrganizeDB (%s)でタイムアウト: %w", roomLabel, err)
		// NOTE: gracefulCtxは既にキャンセル済みのため、まだ残り時間のある元のctxを使用
		if notifyErr := app.NotifyTimeoutToOwner(ctx, timeoutErr); notifyErr != nil {
			return organizeRoomResult{
				err:   fmt.Errorf("timeout notification failed: %w", notifyErr),
				abort: true,
			}
		}
		return organizeRoomResult{timeoutWarningMessage: timeoutErr.Error()}
	}

	app.MessageToOwnerWithError(ctx, fmt.Sprintf("failed to OrganizeDB (%s)", roomLabel), err)
	return organizeRoomResult{}
}

func main() {
	lambda.Start(OrganizeDatabase)
}
