package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

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

type updateWorkNameTrendApp interface {
	UpdateWorkNameTrend(ctx context.Context, apiKey string) error
	NotifyTimeoutToOwner(ctx context.Context, err error) error
	CloseFirestoreClient()
}

var (
	// Unit test で初期化失敗や timeout 分岐を差し替え検証できるようにしている。
	secretFieldFromSecretsManager = lambdautils.SecretFieldFromSecretsManager
	firestoreClientOptionTrend    = lambdautils.FirestoreClientOption
	newTrendWorkspaceApp          = func(ctx context.Context, isTest bool, clientOption option.ClientOption) (updateWorkNameTrendApp, error) {
		return workspaceapp.NewWorkspaceApp(ctx, isTest, clientOption)
	}
)

func UpdateWorkNameTrend(ctx context.Context) error {
	slog.Info(utils.NameOf(UpdateWorkNameTrend))

	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	secretName := os.Getenv("SECRET_NAME")
	if secretName == "" {
		slog.ErrorContext(ctx, "環境変数 SECRET_NAME を設定してください",
		)
		return nil
	}

	apiKey, err := secretFieldFromSecretsManager(gracefulCtx, secretName, "OPENAI_API_KEY")
	if err != nil {
		slog.ErrorContext(ctx, "failed to get OPENAI API key from Secrets Manager",
			"err", err,
		)
		return nil
	}

	clientOption, err := firestoreClientOptionTrend()
	if err != nil {
		slog.ErrorContext(ctx, "failed to get Firestore client option",
			"err", err,
		)
		return nil
	}

	app, err := newTrendWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get WorkspaceApp",
			"err", err,
		)
		return nil
	}
	defer app.CloseFirestoreClient()

	if err := app.UpdateWorkNameTrend(gracefulCtx, apiKey); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// NOTE: gracefulCtxは既にキャンセル済みのため、まだ残り時間のある元のctxを使用
			if notifyErr := app.NotifyTimeoutToOwner(ctx, fmt.Errorf("UpdateWorkNameTrendでタイムアウト: %w", err)); notifyErr != nil {
				return fmt.Errorf("timeout notification failed: %w", notifyErr)
			}
			return nil // タイムアウト警告はDiscord通知成功で、成功として返す
		}
		slog.ErrorContext(ctx, "failed to update work name trends",
			"err", err,
		)
		return nil
	}

	slog.Info(utils.NameOf(UpdateWorkNameTrend) + " finished")

	return nil
}

func main() {
	lambda.Start(UpdateWorkNameTrend)
}
