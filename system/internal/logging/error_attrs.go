package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/openai/openai-go/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/status"
)

// ErrorAttrs はエラーから、ログに出しても安全な診断属性を作る。
//
// 型の検査だけで値を決め、err.Error() や各SDKのメッセージ・本文・URLは使わない。
// 認証情報、メールアドレス、チャンネルID、チャット本文、作業名などが混ざり得るためである。
// 返す属性は次のとおり（該当しないものは含めない）。
//   - error_kind: timeout / canceled / grpc / http / openai / aws_api / net_timeout / unknown
//   - error_code: gRPCコード名、googleapiの先頭reason、OpenAI/AWSのエラーコード
//   - error_status: HTTPステータスコード
//   - error_type: 最も内側のエラーのGo型名
//
// 既存のアラート・フィルタ互換のため、error_class は呼び出し側で別途残すこと。
func ErrorAttrs(err error) []any {
	if err == nil {
		return nil
	}
	kind, code, httpStatus := classifyError(err)
	attrs := []any{slog.String("error_kind", kind)}
	if code != "" {
		attrs = append(attrs, slog.String("error_code", code))
	}
	if httpStatus != 0 {
		attrs = append(attrs, slog.Int("error_status", httpStatus))
	}
	return append(attrs, slog.String("error_type", innermostTypeName(err)))
}

// awsAPIError は smithy.APIError のうち、コードの取得に必要な部分だけを表す。
type awsAPIError interface {
	error
	ErrorCode() string
}

// httpStatusError は AWS SDK の ResponseError などが持つHTTPステータス取得口。
type httpStatusError interface {
	error
	HTTPStatusCode() int
}

func classifyError(err error) (kind, code string, httpStatus int) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout", "", 0
	case errors.Is(err, context.Canceled):
		return "canceled", "", 0
	}

	var openaiErr *openai.Error
	if errors.As(err, &openaiErr) {
		return "openai", sanitizeCode(openaiErr.Code), openaiErr.StatusCode
	}

	var googleErr *googleapi.Error
	if errors.As(err, &googleErr) {
		if len(googleErr.Errors) > 0 {
			code = sanitizeCode(googleErr.Errors[0].Reason)
		}
		return "http", code, googleErr.Code
	}

	if st, ok := status.FromError(err); ok {
		return "grpc", st.Code().String(), 0
	}

	var apiErr awsAPIError
	if errors.As(err, &apiErr) {
		var statusErr httpStatusError
		if errors.As(err, &statusErr) {
			httpStatus = statusErr.HTTPStatusCode()
		}
		return "aws_api", sanitizeCode(apiErr.ErrorCode()), httpStatus
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "net_timeout", "", 0
	}

	return "unknown", "", 0
}

// sanitizeCode は列挙値らしい短い文字列だけを通す。
// 自由文が紛れ込んでもログへ出さないための防御である。
func sanitizeCode(s string) string {
	const maxLen = 64
	if s == "" || len(s) > maxLen {
		return ""
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
		default:
			return ""
		}
	}
	return s
}

// innermostTypeName はUnwrapを辿った先のエラーの型名を返す。型名は安全だがメッセージは安全でない。
func innermostTypeName(err error) string {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return fmt.Sprintf("%T", err)
		}
		err = next
	}
}

// ClassifiedError は呼び出し元へ返す固定メッセージのエラーで、原因は Unwrap で保持する。
// メッセージに原因の本文を含めないため、Lambdaの実行結果やログへ秘匿情報が漏れない。
// errors.Is / errors.As は原因に対して引き続き使える。
type ClassifiedError struct {
	Message string
	Cause   error
}

func (e *ClassifiedError) Error() string { return e.Message }
func (e *ClassifiedError) Unwrap() error { return e.Cause }

// ErrorClassAttrs は固定の error_class に ErrorAttrs の診断属性を添える。
// slog の可変長引数へ展開して使う: slog.Error(msg, logging.ErrorClassAttrs("x", err)...)
func ErrorClassAttrs(class string, err error) []any {
	return append([]any{slog.String("error_class", class)}, ErrorAttrs(err)...)
}
