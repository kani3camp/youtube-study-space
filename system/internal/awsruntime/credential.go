package awsruntime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.google.com/go/auth/credentials/externalaccount"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"google.golang.org/api/option"
)

const (
	googleCloudProjectEnv            = "GOOGLE_CLOUD_PROJECT"
	gcpWIFAudienceEnv                = "GCP_WIF_AUDIENCE"
	gcpWIFServiceAccountEmailEnv     = "GCP_WIF_SERVICE_ACCOUNT_EMAIL"
	awsSubjectTokenType              = "urn:ietf:params:aws:token-type:aws4_request"
	googleCloudPlatformScope         = "https://www.googleapis.com/auth/cloud-platform"
	iamCredentialsServiceAccountPath = "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken"
)

var loadDefaultAWSConfig = config.LoadDefaultConfig

// GoogleWIFConfig is the non-secret configuration required to exchange AWS
// credentials for short-lived Google Cloud credentials through WIF.
type GoogleWIFConfig struct {
	ProjectID           string
	Audience            string
	ServiceAccountEmail string
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
// workloads using Workload Identity Federation and service account impersonation.
func GoogleClientOption(ctx context.Context) (option.ClientOption, error) {
	wif, err := GoogleWIFConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return GoogleClientOptionWithConfig(ctx, wif, "")
}

// GoogleClientOptionWithConfig creates a Google Cloud client credential option
// from explicit WIF settings. When awsProfile is non-empty, AWS credentials are
// loaded from that shared-config profile; otherwise the normal AWS SDK default
// credential chain is used.
func GoogleClientOptionWithConfig(ctx context.Context, wif GoogleWIFConfig, awsProfile string) (option.ClientOption, error) {
	if err := validateGoogleWIFConfig(wif); err != nil {
		return nil, err
	}

	loadOptions := make([]func(*config.LoadOptions) error, 0, 2)
	if profile := strings.TrimSpace(awsProfile); profile != "" {
		loadOptions = append(loadOptions, config.WithSharedConfigProfile(profile))
	}
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
	}
	if region != "" {
		loadOptions = append(loadOptions, config.WithRegion(region))
	}

	awsConfig, err := loadDefaultAWSConfig(ctx, loadOptions...)
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
		Audience:                       wif.Audience,
		SubjectTokenType:               awsSubjectTokenType,
		ServiceAccountImpersonationURL: fmt.Sprintf(iamCredentialsServiceAccountPath, wif.ServiceAccountEmail),
		Scopes:                         []string{googleCloudPlatformScope},
		AwsSecurityCredentialsProvider: awsProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("create Google WIF credentials: %w", err)
	}

	return option.WithAuthCredentials(credentials), nil
}

// GoogleWIFConfigFromEnv loads the non-secret Google WIF settings shared by
// AWS runtimes and local operator commands.
func GoogleWIFConfigFromEnv() (GoogleWIFConfig, error) {
	wif := GoogleWIFConfig{
		ProjectID:           strings.TrimSpace(os.Getenv(googleCloudProjectEnv)),
		Audience:            strings.TrimSpace(os.Getenv(gcpWIFAudienceEnv)),
		ServiceAccountEmail: strings.TrimSpace(os.Getenv(gcpWIFServiceAccountEmailEnv)),
	}
	if err := validateGoogleWIFConfig(wif); err != nil {
		return GoogleWIFConfig{}, err
	}
	return wif, nil
}

func validateGoogleWIFConfig(wif GoogleWIFConfig) error {
	if strings.TrimSpace(wif.ProjectID) == "" {
		return fmt.Errorf("%s is required", googleCloudProjectEnv)
	}
	if strings.TrimSpace(wif.Audience) == "" {
		return fmt.Errorf("%s is required", gcpWIFAudienceEnv)
	}
	if strings.TrimSpace(wif.ServiceAccountEmail) == "" {
		return fmt.Errorf("%s is required", gcpWIFServiceAccountEmailEnv)
	}
	return nil
}

// FirestoreClientOption is kept as a compatibility wrapper for existing call sites.
// New code should prefer GoogleClientOption with the caller's context.
func FirestoreClientOption() (option.ClientOption, error) {
	return GoogleClientOption(context.Background())
}
