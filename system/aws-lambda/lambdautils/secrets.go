package lambdautils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// SecretStringFromSecretsManager is a common function to get secret value (string) from Secrets Manager.
// If SecretString is present, it is returned; otherwise, SecretBinary is converted to a string and returned.
func SecretStringFromSecretsManager(ctx context.Context, secretName string) (string, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		return "", fmt.Errorf("AWS_REGION/AWS_DEFAULT_REGION not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("in config.LoadDefaultConfig: %w", err)
	}
	sm := secretsmanager.NewFromConfig(cfg)

	out, err := sm.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return "", fmt.Errorf("in GetSecretValue: %w", err)
	}

	if out.SecretString != nil {
		return *out.SecretString, nil
	}
	if out.SecretBinary != nil {
		return string(out.SecretBinary), nil
	}
	return "", fmt.Errorf("secret value is empty: %s", secretName)
}

// SecretJSONFromSecretsManager reads secret value from Secrets Manager as JSON and converts it to a string map.
// If the value is not a string, it is converted to a string using fmt.Sprint.
func SecretJSONFromSecretsManager(ctx context.Context, secretName string) (map[string]string, error) {
	raw, err := SecretStringFromSecretsManager(ctx, secretName)
	if err != nil {
		return nil, err
	}
	var anyMap map[string]any
	if err := json.Unmarshal([]byte(raw), &anyMap); err != nil {
		return nil, fmt.Errorf("in json.Unmarshal: %w", err)
	}
	result := make(map[string]string, len(anyMap))
	for k, v := range anyMap {
		switch vv := v.(type) {
		case string:
			result[k] = vv
		default:
			result[k] = fmt.Sprint(v)
		}
	}
	return result, nil
}

// SecretFieldFromSecretsManager はJSONシークレットから指定キーの値を取得します。
func SecretFieldFromSecretsManager(ctx context.Context, secretName string, field string) (string, error) {
	m, err := SecretJSONFromSecretsManager(ctx, secretName)
	if err != nil {
		return "", err
	}
	val, ok := m[field]
	if !ok || val == "" {
		return "", fmt.Errorf("field not found or empty: %s", field)
	}
	return val, nil
}
