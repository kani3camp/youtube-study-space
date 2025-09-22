package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"app.modules/aws-lambda/lambdautils"
	"app.modules/core/utils"
	"app.modules/core/workspaceapp"
	"github.com/aws/aws-lambda-go/lambda"
)

func UpdateWorkNameTrend(ctx context.Context) error {
	slog.Info(utils.NameOf(UpdateWorkNameTrend))

	secretName := os.Getenv("SECRET_NAME")
	if secretName == "" {
		log.Fatal("環境変数 SECRET_NAME を設定してください")
	}
	apiKey, err := lambdautils.SecretFieldFromSecretsManager(ctx, secretName, "OPENAI_API_KEY")
	if err != nil {
		return fmt.Errorf("failed to get OPENAI API key from Secrets Manager: %w", err)
	}

	clientOption, err := lambdautils.FirestoreClientOption()
	if err != nil {
		return fmt.Errorf("failed to get Firestore client option: %w", err)
	}
	app, err := workspaceapp.NewWorkspaceApp(ctx, false, clientOption)
	if err != nil {
		return fmt.Errorf("failed to get WorkspaceApp: %w", err)
	}

	defer app.CloseFirestoreClient()

	if err := app.UpdateWorkNameTrend(ctx, apiKey); err != nil {
		return fmt.Errorf("failed to update work name trends: %w", err)
	}

	slog.Info(utils.NameOf(UpdateWorkNameTrend) + " finished")

	return nil
}

func main() {
	lambda.Start(UpdateWorkNameTrend)
}
