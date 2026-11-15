package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/openai/openai-go/v2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const sentinelSecret = "SENTINEL-secret-token-user@example.com"

type fakeAWSError struct{ code string }

func (e fakeAWSError) Error() string     { return sentinelSecret }
func (e fakeAWSError) ErrorCode() string { return e.code }

type fakeAWSResponseError struct {
	fakeAWSError
	status int
}

func (e fakeAWSResponseError) HTTPStatusCode() int { return e.status }

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return sentinelSecret }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return false }

var _ net.Error = timeoutNetError{}

func flatten(attrs []any) (kind, code, status, typ string) {
	for _, a := range attrs {
		attr := a.(slog.Attr)
		switch attr.Key {
		case "error_kind":
			kind = attr.Value.String()
		case "error_code":
			code = attr.Value.String()
		case "error_status":
			status = attr.Value.String()
		case "error_type":
			typ = attr.Value.String()
		}
	}
	return
}

func TestErrorAttrs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string // "kind|code|status|type"
	}{
		{"deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), "timeout|||context.deadlineExceededError"},
		{"canceled", context.Canceled, "canceled|||*errors.errorString"},
		{"grpc permission", fmt.Errorf("w: %w", status.Error(codes.PermissionDenied, sentinelSecret)), "grpc|PermissionDenied||*status.Error"},
		{"grpc unavailable", status.Error(codes.Unavailable, "x"), "grpc|Unavailable||*status.Error"},
		{"googleapi", &googleapi.Error{Code: 403, Message: sentinelSecret, Errors: []googleapi.ErrorItem{{Reason: "quotaExceeded", Message: sentinelSecret}}}, "http|quotaExceeded|403|*googleapi.Error"},
		{"googleapi free text reason dropped", &googleapi.Error{Code: 500, Errors: []googleapi.ErrorItem{{Reason: "has spaces and " + sentinelSecret}}}, "http||500|*googleapi.Error"},
		{"openai", &openai.Error{StatusCode: 401, Code: "invalid_api_key", Message: sentinelSecret}, "openai|invalid_api_key|401|*apierror.Error"},
		{"aws", fakeAWSError{code: "ResourceNotFoundException"}, "aws_api|ResourceNotFoundException||logging.fakeAWSError"},
		{"aws with http status", fakeAWSResponseError{fakeAWSError{code: "AccessDeniedException"}, 403}, "aws_api|AccessDeniedException|403|logging.fakeAWSResponseError"},
		{"net timeout", timeoutNetError{}, "net_timeout|||logging.timeoutNetError"},
		{"unknown innermost type", fmt.Errorf("a: %w", fmt.Errorf("b: %w", errors.New(sentinelSecret))), "unknown|||*errors.errorString"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, code, status, typ := flatten(ErrorAttrs(tt.err))
			assert.Equal(t, tt.want, strings.Join([]string{kind, code, status, typ}, "|"))
		})
	}
}

func TestErrorAttrsNeverContainErrorMessage(t *testing.T) {
	errs := []error{
		errors.New(sentinelSecret),
		fmt.Errorf("wrap: %w", errors.New(sentinelSecret)),
		status.Error(codes.Internal, sentinelSecret),
		&googleapi.Error{Code: 400, Message: sentinelSecret, Body: sentinelSecret},
		fakeAWSError{code: "Code"},
		timeoutNetError{},
		fmt.Errorf("%s: %w", sentinelSecret, context.DeadlineExceeded),
	}
	for _, err := range errs {
		for _, a := range ErrorAttrs(err) {
			assert.NotContains(t, fmt.Sprint(a), "SENTINEL")
			assert.NotContains(t, fmt.Sprint(a), "example.com")
		}
	}
}

func TestErrorAttrsNil(t *testing.T) {
	assert.Empty(t, ErrorAttrs(nil))
}

func TestClassifiedErrorKeepsCauseWithoutLeakingIt(t *testing.T) {
	cause := fmt.Errorf("%s: %w", sentinelSecret, context.DeadlineExceeded)
	err := &ClassifiedError{Message: "provider request failed", Cause: cause}
	assert.Equal(t, "provider request failed", err.Error())
	assert.NotContains(t, err.Error(), "SENTINEL")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestErrorClassAttrsKeepsClass(t *testing.T) {
	attrs := ErrorClassAttrs("secret_fetch_failed", status.Error(codes.PermissionDenied, sentinelSecret))
	first := attrs[0].(slog.Attr)
	assert.Equal(t, "error_class", first.Key)
	assert.Equal(t, "secret_fetch_failed", first.Value.String())
	kind, code, _, _ := flatten(attrs[1:])
	assert.Equal(t, "grpc", kind)
	assert.Equal(t, "PermissionDenied", code)
}
