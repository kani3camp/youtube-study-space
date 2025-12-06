package youtubebot

import (
	"testing"

	"google.golang.org/api/youtube/v3"

	"github.com/stretchr/testify/assert"
)

func TestExtractAuthorDisplayName(t *testing.T) {
	tests := []struct {
		name           string
		displayName    string
		expectedResult string
	}{
		{
			name:           "Display name with @ prefix",
			displayName:    "@username",
			expectedResult: "username",
		},
		{
			name:           "Display name without @ prefix",
			displayName:    "username",
			expectedResult: "username",
		},
		{
			name:           "Empty string",
			displayName:    "",
			expectedResult: "",
		},
		{
			name:           "Only @ character",
			displayName:    "@",
			expectedResult: "",
		},
		{
			name:           "Multiple @ at start (only first one removed)",
			displayName:    "@@username",
			expectedResult: "@username",
		},
		{
			name:           "@ in middle of name (not at start)",
			displayName:    "user@name",
			expectedResult: "user@name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chat := &youtube.LiveChatMessage{
				AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
					DisplayName: tt.displayName,
				},
			}

			result := ExtractAuthorDisplayName(chat)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
