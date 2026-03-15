package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

type mockStepFunctionsClient struct {
	startExecution func(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
}

func (m mockStepFunctionsClient) StartExecution(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	return m.startExecution(ctx, params, optFns...)
}

func TestHandlerReturnsErrorWhenLoadDefaultConfigTimesOut(t *testing.T) {
	t.Setenv("STATE_MACHINE_ARN", "arn:aws:states:ap-northeast-1:123456789012:stateMachine:test")
	t.Setenv("AWS_REGION", "ap-northeast-1")

	originalLoadDefaultConfig := loadDefaultConfig
	originalNewSFNClient := newSFNClient
	t.Cleanup(func() {
		loadDefaultConfig = originalLoadDefaultConfig
		newSFNClient = originalNewSFNClient
	})

	loadDefaultConfig = func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, context.DeadlineExceeded
	}
	newSFNClient = func(cfg aws.Config) stepFunctionsClient {
		t.Fatalf("newSFNClient should not be called when config loading fails")
		return nil
	}

	err := handler(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}
}

func TestHandlerReturnsErrorWhenStartExecutionTimesOut(t *testing.T) {
	t.Setenv("STATE_MACHINE_ARN", "arn:aws:states:ap-northeast-1:123456789012:stateMachine:test")
	t.Setenv("AWS_REGION", "ap-northeast-1")

	originalLoadDefaultConfig := loadDefaultConfig
	originalNewSFNClient := newSFNClient
	t.Cleanup(func() {
		loadDefaultConfig = originalLoadDefaultConfig
		newSFNClient = originalNewSFNClient
	})

	loadDefaultConfig = func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, nil
	}
	newSFNClient = func(cfg aws.Config) stepFunctionsClient {
		return mockStepFunctionsClient{
			startExecution: func(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
				return nil, context.DeadlineExceeded
			},
		}
	}

	err := handler(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}
}

func TestHandlerWrapsErrorWhenStartExecutionFails(t *testing.T) {
	t.Setenv("STATE_MACHINE_ARN", "arn:aws:states:ap-northeast-1:123456789012:stateMachine:test")
	t.Setenv("AWS_REGION", "ap-northeast-1")

	originalLoadDefaultConfig := loadDefaultConfig
	originalNewSFNClient := newSFNClient
	t.Cleanup(func() {
		loadDefaultConfig = originalLoadDefaultConfig
		newSFNClient = originalNewSFNClient
	})

	loadDefaultConfig = func(context.Context, ...func(*awsconfig.LoadOptions) error) (aws.Config, error) {
		return aws.Config{}, nil
	}

	startErr := errors.New("start failed")
	newSFNClient = func(cfg aws.Config) stepFunctionsClient {
		return mockStepFunctionsClient{
			startExecution: func(ctx context.Context, params *sfn.StartExecutionInput, optFns ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
				return nil, startErr
			},
		}
	}

	err := handler(context.Background())
	if !errors.Is(err, startErr) {
		t.Fatalf("expected wrapped start error, got %v", err)
	}
	if !strings.Contains(err.Error(), "in StartExecution") {
		t.Fatalf("expected context in error message, got %v", err)
	}
}
