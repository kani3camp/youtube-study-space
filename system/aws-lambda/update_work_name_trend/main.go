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
)

func init() {
	logging.InitLogger()
}

func UpdateWorkNameTrend(ctx context.Context) error {
	slog.Info(utils.NameOf(UpdateWorkNameTrend))

	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	secretName := os.Getenv("SECRET_NAME")
	if secretName == "" {
		return fmt.Errorf("環境変数 SECRET_NAME を設定してください")
	}

	apiKey, err := lambdautils.SecretFieldFromSecretsManager(gracefulCtx, secretName, "OPENAI_API_KEY")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// 初期化中のタイムアウト（appがないのでDiscord通知不可）
			return fmt.Errorf("timeout during SecretFieldFromSecretsManager: %w", err)
		}
		return fmt.Errorf("failed to get OPENAI API key from Secrets Manager: %w", err)
	}

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return fmt.Errorf("failed to get Firestore client option: %w", err)
	}

	app, err := workspaceapp.NewWorkspaceApp(gracefulCtx, false, clientOption)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("timeout during NewWorkspaceApp: %w", err)
		}
		return fmt.Errorf("failed to get WorkspaceApp: %w", err)
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
		return fmt.Errorf("failed to update work name trends: %w", err)
	}

	slog.Info(utils.NameOf(UpdateWorkNameTrend) + " finished")

	return nil
}

func main() {
	lambda.Start(UpdateWorkNameTrend)
}
