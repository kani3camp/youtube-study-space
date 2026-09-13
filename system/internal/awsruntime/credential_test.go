package awsruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/auth/credentials/externalaccount"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	htransport "google.golang.org/api/transport/http"
)

type fakeAWSCredentialsProvider struct {
	credentials   aws.Credentials
	err           error
	retrieveCalls *int
}

func (f fakeAWSCredentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	if f.retrieveCalls != nil {
		*f.retrieveCalls = *f.retrieveCalls + 1
	}
	return f.credentials, f.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(t *testing.T, request *http.Request, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal mock JSON response: %v", err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(bytes.NewReader(payload)),
		Request: request,
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
	t.Setenv(googleCloudProjectEnv, "")
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

func TestGoogleClientOptionWIFExchangesAndImpersonatesToken(t *testing.T) {
	t.Setenv(googleCloudProjectEnv, "test-youtube-study-space")
	t.Setenv(gcpWIFAudienceEnv, "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/aws/providers/dev")
	t.Setenv(gcpWIFServiceAccountEmailEnv, "runtime@test-youtube-study-space.iam.gserviceaccount.com")
	t.Setenv("AWS_REGION", "ap-northeast-1")

	retrieveCalls := 0
	originalLoadDefaultAWSConfig := loadDefaultAWSConfig
	loadDefaultAWSConfig = func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
		return aws.Config{
			Region: "ap-northeast-1",
			Credentials: fakeAWSCredentialsProvider{
				credentials: aws.Credentials{
					AccessKeyID:     "AKIAEXAMPLE",
					SecretAccessKey: "secret",
					SessionToken:    "session",
				},
				retrieveCalls: &retrieveCalls,
			},
		}, nil
	}
	t.Cleanup(func() {
		loadDefaultAWSConfig = originalLoadDefaultAWSConfig
	})

	stsCalls := 0
	impersonationCalls := 0
	originalDefaultTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "sts.googleapis.com":
			stsCalls++
			if err := request.ParseForm(); err != nil {
				t.Fatalf("parse STS form: %v", err)
			}
			subjectToken := request.Form.Get("subject_token")
			if subjectToken == "" {
				t.Error("STS request did not contain subject_token")
			}
			if !strings.Contains(subjectToken, "GetCallerIdentity") {
				t.Errorf("STS subject_token did not contain signed AWS GetCallerIdentity request: %q", subjectToken)
			}
			return jsonResponse(t, request, map[string]any{
				"access_token":      "federated-token",
				"expires_in":        3600,
				"issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
				"token_type":        "Bearer",
			}), nil
		case "iamcredentials.googleapis.com":
			impersonationCalls++
			if got := request.Header.Get("Authorization"); got != "Bearer federated-token" {
				t.Errorf("unexpected impersonation authorization header: %q", got)
			}
			return jsonResponse(t, request, map[string]any{
				"accessToken": "impersonated-token",
				"expireTime":  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			}), nil
		default:
			t.Errorf("unexpected WIF HTTP request: %s", request.URL.String())
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("unexpected request")),
				Request:    request,
			}, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = originalDefaultTransport
	})

	ctx := context.Background()
	clientOption, err := GoogleClientOption(ctx)
	if err != nil {
		t.Fatalf("GoogleClientOption returned error: %v", err)
	}

	finalCalls := 0
	transport, err := htransport.NewTransport(
		ctx,
		roundTripFunc(func(request *http.Request) (*http.Response, error) {
			finalCalls++
			if got := request.Header.Get("Authorization"); got != "Bearer impersonated-token" {
				t.Errorf("unexpected final authorization header: %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       http.NoBody,
				Request:    request,
			}, nil
		}),
		clientOption,
	)
	if err != nil {
		t.Fatalf("create authenticated transport: %v", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test/resource", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}

	if retrieveCalls == 0 {
		t.Fatal("AWS credentials provider was not used")
	}
	if stsCalls != 1 {
		t.Fatalf("unexpected STS call count: %d", stsCalls)
	}
	if impersonationCalls != 1 {
		t.Fatalf("unexpected impersonation call count: %d", impersonationCalls)
	}
	if finalCalls != 1 {
		t.Fatalf("unexpected final request count: %d", finalCalls)
	}
}
