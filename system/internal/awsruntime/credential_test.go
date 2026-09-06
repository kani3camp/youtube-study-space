package awsruntime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cloud.google.com/go/auth/credentials/externalaccount"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type fakeAWSCredentialsProvider struct {
	credentials aws.Credentials
	err         error
}

func (f fakeAWSCredentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return f.credentials, f.err
}

func TestGoogleClientOptionRejectsUnknownAuthMode(t *testing.T) {
	t.Setenv(gcpAuthModeEnv, "wat")

	_, err := GoogleClientOption(context.Background())
	if err == nil || !strings.Contains(err.Error(), gcpAuthModeEnv) {
		t.Fatalf("expected invalid auth mode error, got %v", err)
	}
}

func TestWIFConfigFromEnv(t *testing.T) {
	t.Setenv(googleCloudProjectEnv, "test-youtube-study-space")
	t.Setenv(gcpWIFAudienceEnv, "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/aws/providers/dev")
	t.Setenv(gcpWIFServiceAccountEmailEnv, "runtime@test-youtube-study-space.iam.gserviceaccount.com")

	got, err := wifConfigFromEnv()
	if err != nil {
		t.Fatalf("wifConfigFromEnv returned error: %v", err)
	}
	if got.audience != "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/aws/providers/dev" {
		t.Fatalf("unexpected audience: %q", got.audience)
	}
	if got.serviceAccountEmail != "runtime@test-youtube-study-space.iam.gserviceaccount.com" {
		t.Fatalf("unexpected service account email: %q", got.serviceAccountEmail)
	}
}

func TestWIFConfigFromEnvRequiresProjectID(t *testing.T) {
	t.Setenv(gcpWIFAudienceEnv, "audience")
	t.Setenv(gcpWIFServiceAccountEmailEnv, "runtime@example.iam.gserviceaccount.com")

	_, err := wifConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), googleCloudProjectEnv) {
		t.Fatalf("expected missing project ID error, got %v", err)
	}
}

func TestAWSSDKSecurityCredentialsProvider(t *testing.T) {
	want := aws.Credentials{
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "secret",
		SessionToken:    "session",
	}
	provider := &awsSDKSecurityCredentialsProvider{
		region:      "ap-northeast-1",
		credentials: fakeAWSCredentialsProvider{credentials: want},
	}

	region, err := provider.AwsRegion(context.Background(), &externalaccount.RequestOptions{})
	if err != nil {
		t.Fatalf("AwsRegion returned error: %v", err)
	}
	if region != "ap-northeast-1" {
		t.Fatalf("unexpected region: %q", region)
	}

	got, err := provider.AwsSecurityCredentials(context.Background(), &externalaccount.RequestOptions{})
	if err != nil {
		t.Fatalf("AwsSecurityCredentials returned error: %v", err)
	}
	if got.AccessKeyID != want.AccessKeyID || got.SecretAccessKey != want.SecretAccessKey || got.SessionToken != want.SessionToken {
		t.Fatalf("unexpected credentials: %#v", got)
	}
}

func TestAWSSDKSecurityCredentialsProviderPropagatesRetrieveError(t *testing.T) {
	provider := &awsSDKSecurityCredentialsProvider{
		region:      "ap-northeast-1",
		credentials: fakeAWSCredentialsProvider{err: errors.New("boom")},
	}

	_, err := provider.AwsSecurityCredentials(context.Background(), &externalaccount.RequestOptions{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected retrieve error, got %v", err)
	}
}

func TestGoogleClientOptionWIFUsesAWSDefaultCredentialChain(t *testing.T) {
	t.Setenv(gcpAuthModeEnv, gcpAuthModeWIF)
	t.Setenv(googleCloudProjectEnv, "test-youtube-study-space")
	t.Setenv(gcpWIFAudienceEnv, "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/aws/providers/dev")
	t.Setenv(gcpWIFServiceAccountEmailEnv, "runtime@test-youtube-study-space.iam.gserviceaccount.com")
	t.Setenv("AWS_REGION", "ap-northeast-1")

	originalLoadDefaultAWSConfig := loadDefaultAWSConfig
	loadDefaultAWSConfig = func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{
			Region: "ap-northeast-1",
			Credentials: fakeAWSCredentialsProvider{credentials: aws.Credentials{
				AccessKeyID:     "AKIAEXAMPLE",
				SecretAccessKey: "secret",
				SessionToken:    "session",
			}},
		}, nil
	}
	t.Cleanup(func() {
		loadDefaultAWSConfig = originalLoadDefaultAWSConfig
	})

	if _, err := GoogleClientOption(context.Background()); err != nil {
		t.Fatalf("GoogleClientOption returned error: %v", err)
	}
}
