package awsruntime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"app.modules/internal/awsruntime/mydynamodb"

	"cloud.google.com/go/auth/credentials/externalaccount"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"google.golang.org/api/option"
)

const (
	gcpAuthModeEnv                   = "GCP_AUTH_MODE"
	gcpAuthModeLegacy                = "legacy"
	gcpAuthModeWIF                   = "wif"
	googleCloudProjectEnv            = "GOOGLE_CLOUD_PROJECT"
	gcpWIFAudienceEnv                = "GCP_WIF_AUDIENCE"
	gcpWIFServiceAccountEmailEnv     = "GCP_WIF_SERVICE_ACCOUNT_EMAIL"
	awsSubjectTokenType              = "urn:ietf:params:aws:token-type:aws4_request"
	googleCloudPlatformScope         = "https://www.googleapis.com/auth/cloud-platform"
	iamCredentialsServiceAccountPath = "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken"
)

var loadDefaultAWSConfig = config.LoadDefaultConfig

type wifConfig struct {
	audience            string
	serviceAccountEmail string
}

type awsSDKSecurityCredentialsProvider struct {
	region      string
	credentials aws.CredentialsProvider
}

func (p *awsSDKSecurityCredentialsProvider) AwsRegion(context.Context, *externalaccount.RequestOptions) (string, error) {
	if p.region == "" {
		return "", fmt.Errorf("AWS region is empty")
	}
	return p.region, nil
}

func (p *awsSDKSecurityCredentialsProvider) AwsSecurityCredentials(ctx context.Context, _ *externalaccount.RequestOptions) (*externalaccount.AwsSecurityCredentials, error) {
	if p.credentials == nil {
		return nil, fmt.Errorf("AWS credentials provider is nil")
	}

	credentials, err := p.credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieve AWS credentials: %w", err)
	}

	return &externalaccount.AwsSecurityCredentials{
		AccessKeyID:     credentials.AccessKeyID,
		SecretAccessKey: credentials.SecretAccessKey,
		SessionToken:    credentials.SessionToken,
	}, nil
}

// GoogleClientOption returns the Google Cloud client credential option for AWS
// workloads. During the migration, legacy DynamoDB-backed service account
// credentials remain the default so deployments are rollback-safe.
func GoogleClientOption(ctx context.Context) (option.ClientOption, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(gcpAuthModeEnv)))
	if mode == "" {
		mode = gcpAuthModeLegacy
	}

	switch mode {
	case gcpAuthModeLegacy:
		return legacyGoogleClientOption()
	case gcpAuthModeWIF:
		return wifGoogleClientOption(ctx)
	default:
		return nil, fmt.Errorf("%s must be %q or %q, got %q", gcpAuthModeEnv, gcpAuthModeLegacy, gcpAuthModeWIF, mode)
	}
}

func legacyGoogleClientOption() (option.ClientOption, error) {
	credentialBytes, err := mydynamodb.FetchFirebaseCredentialsAsBytes()
	if err != nil {
		return nil, fmt.Errorf("in FetchFirebaseCredentialsAsBytes: %w", err)
	}
	//nolint:staticcheck // Temporary legacy fallback. Removed after production WIF validation and credential-key retirement.
	return option.WithCredentialsJSON(credentialBytes), nil
}

func wifGoogleClientOption(ctx context.Context) (option.ClientOption, error) {
	wif, err := wifConfigFromEnv()
	if err != nil {
		return nil, err
	}

	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
	}

	var awsConfig aws.Config
	if region == "" {
		awsConfig, err = loadDefaultAWSConfig(ctx)
	} else {
		awsConfig, err = loadDefaultAWSConfig(ctx, config.WithRegion(region))
	}
	if err != nil {
		return nil, fmt.Errorf("load AWS default config: %w", err)
	}
	if awsConfig.Region == "" {
		return nil, fmt.Errorf("AWS region could not be resolved")
	}

	awsProvider := &awsSDKSecurityCredentialsProvider{
		region:      awsConfig.Region,
		credentials: awsConfig.Credentials,
	}
	credentials, err := externalaccount.NewCredentials(&externalaccount.Options{
		Audience:                       wif.audience,
		SubjectTokenType:               awsSubjectTokenType,
		ServiceAccountImpersonationURL: fmt.Sprintf(iamCredentialsServiceAccountPath, wif.serviceAccountEmail),
		Scopes:                         []string{googleCloudPlatformScope},
		AwsSecurityCredentialsProvider: awsProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("create Google WIF credentials: %w", err)
	}

	return option.WithAuthCredentials(credentials), nil
}

func wifConfigFromEnv() (wifConfig, error) {
	if strings.TrimSpace(os.Getenv(googleCloudProjectEnv)) == "" {
		return wifConfig{}, fmt.Errorf("%s is required when %s=%s", googleCloudProjectEnv, gcpAuthModeEnv, gcpAuthModeWIF)
	}

	audience := strings.TrimSpace(os.Getenv(gcpWIFAudienceEnv))
	if audience == "" {
		return wifConfig{}, fmt.Errorf("%s is required when %s=%s", gcpWIFAudienceEnv, gcpAuthModeEnv, gcpAuthModeWIF)
	}

	serviceAccountEmail := strings.TrimSpace(os.Getenv(gcpWIFServiceAccountEmailEnv))
	if serviceAccountEmail == "" {
		return wifConfig{}, fmt.Errorf("%s is required when %s=%s", gcpWIFServiceAccountEmailEnv, gcpAuthModeEnv, gcpAuthModeWIF)
	}

	return wifConfig{
		audience:            audience,
		serviceAccountEmail: serviceAccountEmail,
	}, nil
}

// FirestoreClientOption is kept as a migration compatibility wrapper for
// existing call sites. New code should use GoogleClientOption with the caller's
// context. This wrapper is removed together with the legacy auth path.
func FirestoreClientOption() (option.ClientOption, error) {
	return GoogleClientOption(context.Background())
}
