package apigatewayhttp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

// Serve adapts an API Gateway HTTP API v2 request to a standard net/http handler.
//
// Keep application code behind http.Handler so the MyPage API does not depend on
// API Gateway or Lambda event shapes.
func Serve(ctx context.Context, event events.APIGatewayV2HTTPRequest, handler http.Handler) events.APIGatewayV2HTTPResponse {
	req, err := toHTTPRequest(ctx, event)
	if err != nil {
		return JSONError(http.StatusBadRequest, "invalid_request", "invalid request")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	result := recorder.Result()
	defer result.Body.Close()

	headers, multiValueHeaders, cookies := responseHeaders(result.Header)

	return events.APIGatewayV2HTTPResponse{
		StatusCode:        result.StatusCode,
		Headers:           headers,
		MultiValueHeaders: multiValueHeaders,
		Body:              recorder.Body.String(),
		IsBase64Encoded:   false,
		Cookies:           cookies,
	}
}

// JSONError returns a small JSON error response suitable for Lambda handlers.
// Lambda handlers can return this with nil error so API Gateway receives the
// intended HTTP status code.
func JSONError(statusCode int, code string, message string) events.APIGatewayV2HTTPResponse {
	body, err := json.Marshal(struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		Error: struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		body = []byte(`{"error":{"code":"internal_error","message":"internal server error"}}`)
		statusCode = http.StatusInternalServerError
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":  "application/json; charset=utf-8",
			"Cache-Control": "no-store",
		},
		Body:            string(body),
		IsBase64Encoded: false,
	}
}

func toHTTPRequest(ctx context.Context, event events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	bodyBytes := []byte(event.Body)
	if event.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(event.Body)
		if err != nil {
			return nil, err
		}
		bodyBytes = decoded
	}

	method := event.RequestContext.HTTP.Method
	if method == "" {
		method = http.MethodGet
	}

	path := event.RawPath
	if path == "" {
		path = event.RequestContext.HTTP.Path
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	target := path
	if event.RawQueryString != "" {
		target += "?" + event.RawQueryString
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	for name, value := range event.Headers {
		if strings.EqualFold(name, "host") {
			req.Host = value
			continue
		}
		req.Header.Set(name, value)
	}

	if len(event.Cookies) > 0 && req.Header.Get("Cookie") == "" {
		req.Header.Set("Cookie", strings.Join(event.Cookies, "; "))
	}

	if event.RequestContext.HTTP.SourceIP != "" {
		req.RemoteAddr = event.RequestContext.HTTP.SourceIP
	}
	if event.RequestContext.HTTP.UserAgent != "" && req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", event.RequestContext.HTTP.UserAgent)
	}

	return req, nil
}

func responseHeaders(header http.Header) (map[string]string, map[string][]string, []string) {
	headers := make(map[string]string, len(header))
	multiValueHeaders := make(map[string][]string)
	var cookies []string

	for name, values := range header {
		if strings.EqualFold(name, "Set-Cookie") {
			cookies = append(cookies, values...)
			continue
		}

		if len(values) == 0 {
			continue
		}

		headers[name] = strings.Join(values, ", ")
		if len(values) > 1 {
			copiedValues := make([]string, len(values))
			copy(copiedValues, values)
			multiValueHeaders[name] = copiedValues
		}
	}

	if len(multiValueHeaders) == 0 {
		multiValueHeaders = nil
	}
	if len(cookies) == 0 {
		cookies = nil
	}

	return headers, multiValueHeaders, cookies
}
