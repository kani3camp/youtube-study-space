package mydynamodb

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type SecretData struct {
	SecretData string `dynamodbav:"secret_data"`
}

func FetchFirebaseCredentialsAsBytes() ([]byte, error) {
	region := os.Getenv("AWS_REGION") // Use the same region as the Lambda function for the DynamoDB table
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		return nil, fmt.Errorf("AWS_REGION/AWS_DEFAULT_REGION not set")
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("in config.LoadDefaultConfig: %w", err)
	}
	db := dynamodb.NewFromConfig(cfg)

	params := &dynamodb.GetItemInput{
		TableName: aws.String("secrets"),
		Key: map[string]types.AttributeValue{
			"secret_name": &types.AttributeValueMemberS{Value: SecretNameFirestore},
		},
	}

	result, err := db.GetItem(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("in db.GetItem: %w", err)
	}
	secretData := SecretData{}
	if err := attributevalue.UnmarshalMap(result.Item, &secretData); err != nil {
		return nil, fmt.Errorf("in attributevalue.UnmarshalMap: %w", err)
	}
	return []byte(secretData.SecretData), nil
}
