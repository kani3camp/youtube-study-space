package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"app.modules/aws-lambda/lambdautils"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

type stepFunctionsClient interface {
	StartExecution(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
}

func init() {
	lambdautils.InitLogger()
}

const (
	jstOffsetSeconds = 9 * 60 * 60
	emptyJSONInput   = "{}"
)

var (
	loadDefaultConfig = config.LoadDefaultConfig
	newSFNClient      = func(cfg aws.Config) stepFunctionsClient {
		return sfn.NewFromConfig(cfg)
	}
)

func handler(ctx context.Context) error {
	// Lambdaタイムアウトの5秒前にキャンセルされる派生コンテキストを作成
	gracefulCtx, cancel := lambdautils.CreateGracefulContext(ctx, lambdautils.DefaultGraceSeconds)
	defer cancel()

	stateMachineArn := os.Getenv("STATE_MACHINE_ARN")
	if stateMachineArn == "" {
		return fmt.Errorf("STATE_MACHINE_ARN is not set")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		return fmt.Errorf("AWS region is not set")
	}

	// Build idempotent execution name based on JST date
	jst := time.FixedZone("JST", jstOffsetSeconds)
	today := time.Now().In(jst).Format("20060102")
	execName := fmt.Sprintf("daily-batch-%s", today)

	cfg, err := loadDefaultConfig(gracefulCtx, config.WithRegion(region))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// NOTE: このLambdaはWorkspaceAppを使用していないため、Discord通知はできない（ログのみ）
			slog.ErrorContext(gracefulCtx, "timeout warning in start_daily_batch during LoadDefaultConfig", "err", err)
			return fmt.Errorf("timeout in config.LoadDefaultConfig: %w", err)
		}
		slog.ErrorContext(gracefulCtx, "failed to load AWS config", "err", err)
		return fmt.Errorf("in config.LoadDefaultConfig: %w", err)
	}
	client := newSFNClient(cfg)

	_, err = client.StartExecution(gracefulCtx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(stateMachineArn),
		Name:            aws.String(execName),
		Input:           aws.String(emptyJSONInput),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			slog.ErrorContext(gracefulCtx, "timeout warning in start_daily_batch during StartExecution", "err", err)
			return fmt.Errorf("timeout in StartExecution: %w", err)
		}
		slog.ErrorContext(gracefulCtx, "failed to start state machine execution", "name", execName, "err", err)
		return fmt.Errorf("in StartExecution: %w", err)
	}
	slog.InfoContext(gracefulCtx, "started state machine execution", "name", execName)
	return nil
}

func main() {
	lambda.Start(handler)
}
