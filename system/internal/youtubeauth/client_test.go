package youtubeauth

import (
	"context"
	"errors"
	"testing"

	"app.modules/core/mypage"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/googleapi"
)

func TestClassifyChannelsListError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "invalid access token",
			err:  &googleapi.Error{Code: 401},
			want: mypage.ErrInvalidYouTubeAccessToken,
		},
		{
			name: "insufficient scope",
			err: &googleapi.Error{
				Code:   403,
				Errors: []googleapi.ErrorItem{{Reason: "insufficientPermissions"}},
			},
			want: mypage.ErrInvalidYouTubeAccessToken,
		},
		{
			name: "insufficient scope in structured error body",
			err: &googleapi.Error{
				Code: 403,
				Body: `{"error":{"details":[{"reason":"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}]}}`,
			},
			want: mypage.ErrInvalidYouTubeAccessToken,
		},
		{
			name: "quota exceeded",
			err: &googleapi.Error{
				Code:   403,
				Errors: []googleapi.ErrorItem{{Reason: "quotaExceeded"}},
			},
			want: mypage.ErrYouTubeRateLimited,
		},
		{
			name: "http rate limit",
			err:  &googleapi.Error{Code: 429},
			want: mypage.ErrYouTubeRateLimited,
		},
		{
			name: "other forbidden response",
			err:  &googleapi.Error{Code: 403},
			want: mypage.ErrYouTubeForbidden,
		},
		{
			name: "youtube server error",
			err:  &googleapi.Error{Code: 503},
			want: mypage.ErrYouTubeUpstream,
		},
		{
			name: "unexpected google api client error",
			err:  &googleapi.Error{Code: 400},
			want: mypage.ErrYouTubeUpstream,
		},
		{
			name: "network error",
			err:  errors.New("connection reset"),
			want: mypage.ErrYouTubeUpstream,
		},
		{
			name: "context timeout",
			err:  context.DeadlineExceeded,
			want: mypage.ErrYouTubeUpstream,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyChannelsListError(tt.err)

			assert.ErrorIs(t, got, tt.want)
			assert.ErrorIs(t, got, tt.err)
		})
	}
}
