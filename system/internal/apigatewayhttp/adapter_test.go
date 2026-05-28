package apigatewayhttp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServe_AdaptsAPIGatewayV2RequestToHTTPHandler(t *testing.T) {
	t.Parallel()

	body := `{"ok":true}`
	event := events.APIGatewayV2HTTPRequest{
		RawPath:         "mypage/me",
		RawQueryString:  "a=1&b=two",
		Body:            base64.StdEncoding.EncodeToString([]byte(body)),
		IsBase64Encoded: true,
		Headers:         map[string]string{"Host": "api.example.com", "X-Test": "yes"},
		Cookies:         []string{"session=abc", "theme=dark"},
		RequestContext:  events.APIGatewayV2HTTPRequestContext{},
		RouteKey:        "POST /mypage/me",
		Version:         "2.0",
	}
	event.RequestContext.HTTP.Method = http.MethodPost
	event.RequestContext.HTTP.Path = "/fallback"
	event.RequestContext.HTTP.SourceIP = "203.0.113.10"
	event.RequestContext.HTTP.UserAgent = "test-agent"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/mypage/me", r.URL.Path)
		assert.Equal(t, "a=1&b=two", r.URL.RawQuery)
		assert.Equal(t, "api.example.com", r.Host)
		assert.Equal(t, "yes", r.Header.Get("X-Test"))
		assert.Equal(t, "session=abc; theme=dark", r.Header.Get("Cookie"))
		assert.Equal(t, "203.0.113.10", r.RemoteAddr)
		assert.Equal(t, "test-agent", r.UserAgent())

		gotBody, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, body, string(gotBody))

		w.Header().Set("X-Single", "value")
		w.Header().Add("X-Multi", "first")
		w.Header().Add("X-Multi", "second")
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "updated"})
		http.SetCookie(w, &http.Cookie{Name: "mode", Value: "test"})
		w.WriteHeader(http.StatusAccepted)
		_, err = w.Write([]byte(`{"accepted":true}`))
		require.NoError(t, err)
	})

	resp := Serve(context.Background(), event, handler)

	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, `{"accepted":true}`, resp.Body)
	assert.False(t, resp.IsBase64Encoded)
	assert.Equal(t, "value", resp.Headers["X-Single"])
	assert.Equal(t, "first, second", resp.Headers["X-Multi"])
	assert.Equal(t, []string{"first", "second"}, resp.MultiValueHeaders["X-Multi"])
	assert.Len(t, resp.Cookies, 2)
	assert.Contains(t, resp.Cookies[0], "session=updated")
	assert.Contains(t, resp.Cookies[1], "mode=test")
}

func TestServe_ReturnsBadRequestForInvalidBase64Body(t *testing.T) {
	t.Parallel()

	resp := Serve(
		context.Background(),
		events.APIGatewayV2HTTPRequest{
			Body:            "invalid-base64",
			IsBase64Encoded: true,
		},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called")
		}),
	)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", resp.Headers["Content-Type"])

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &body))
	assert.Equal(t, "invalid_request", body.Error.Code)
}
