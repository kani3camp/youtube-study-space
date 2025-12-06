package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

const (
	jstOffsetSeconds = 9 * 60 * 60
	emptyJSONInput   = "{}"
)

func handler(ctx context.Context) error {
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

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		slog.ErrorContext(ctx, "failed to load AWS config", "err", err)
		return fmt.Errorf("in config.LoadDefaultConfig: %w", err)
	}
	client := sfn.NewFromConfig(cfg)

	_, err = client.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(stateMachineArn),
		Name:            aws.String(execName),
		Input:           aws.String(emptyJSONInput),
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to start state machine execution", "name", execName, "err", err)
		return err
	}
	slog.InfoContext(ctx, "started state machine execution", "name", execName)
	return nil
}

func main() {
	lambda.Start(handler)
}
