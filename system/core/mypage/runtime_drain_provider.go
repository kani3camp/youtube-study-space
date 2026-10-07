package mypage

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

// Preserve a stable public string while conveying uncertainty without keeping
// credential-bearing URL/provider errors anywhere in the returned error.
type runtimeProviderError struct{ *APIError }

func (*runtimeProviderError) Unwrap() error { return ErrRuntimeOutcomeUnknown }

func unknownProviderError(code string) error {
	return &runtimeProviderError{APIError: &APIError{Code: code}}
}

func exchangeProviderError(err error) error {
	var response *oauth2.RetrieveError
	if errors.As(err, &response) && response.Response != nil && response.Response.StatusCode >= http.StatusBadRequest && response.Response.StatusCode < http.StatusInternalServerError {
		switch response.ErrorCode {
		case "invalid_request", "invalid_client", "invalid_grant", "unauthorized_client", "unsupported_grant_type", "invalid_scope", "access_denied":
			return apiError("OAUTH_FAILED")
		}
	}
	// Transport failures, child deadlines, 5xx and malformed grant responses
	// cannot prove whether the remote code exchange produced credentials.
	return unknownProviderError("OAUTH_FAILED")
}

type runtimeProviderBody struct {
	reader io.Reader
	failed bool
}

func (body *runtimeProviderBody) Read(p []byte) (int, error) {
	n, err := body.reader.Read(p)
	if err == nil {
		return n, nil
	}
	if errors.Is(err, io.EOF) {
		return n, io.EOF
	}
	body.failed = true
	return n, fmt.Errorf("read provider body: %w", err)
}
