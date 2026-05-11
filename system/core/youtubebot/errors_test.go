package youtubebot

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/googleapi"
)

func TestIsLiveChatEndedError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "non googleapi error",
			err:  errors.New("liveChatEnded"),
			want: false,
		},
		{
			name: "403 liveChatEnded reason",
			err: &googleapi.Error{
				Code: 403,
				Errors: []googleapi.ErrorItem{
					{Reason: "liveChatEnded"},
				},
			},
			want: true,
		},
		{
			name: "403 without reason",
			err: &googleapi.Error{
				Code: 403,
			},
			want: false,
		},
		{
			name: "403 different reason",
			err: &googleapi.Error{
				Code: 403,
				Errors: []googleapi.ErrorItem{
					{Reason: "forbidden"},
				},
			},
			want: false,
		},
		{
			name: "fallback message",
			err: &googleapi.Error{
				Code:    403,
				Message: "The specified live chat is no longer live., liveChatEnded",
			},
			want: true,
		},
		{
			name: "fallback body",
			err: &googleapi.Error{
				Code: 403,
				Body: `{"error":{"errors":[{"reason":"liveChatEnded"}]}}`,
			},
			want: true,
		},
		{
			name: "does not use fallback when errors are present",
			err: &googleapi.Error{
				Code:    403,
				Message: "liveChatEnded",
				Errors: []googleapi.ErrorItem{
					{Reason: "forbidden"},
				},
			},
			want: false,
		},
		{
			name: "non 403 with liveChatEnded text",
			err: &googleapi.Error{
				Code:    500,
				Message: "liveChatEnded",
			},
			want: false,
		},
		{
			name: "wrapped liveChatEnded error",
			err: fmt.Errorf("wrapped: %w", &googleapi.Error{
				Code: 403,
				Errors: []googleapi.ErrorItem{
					{Reason: "liveChatEnded"},
				},
			}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isLiveChatEndedError(tt.err))
		})
	}
}
