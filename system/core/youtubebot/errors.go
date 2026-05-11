package youtubebot

import (
	"errors"
	"strings"

	"google.golang.org/api/googleapi"
)

func isLiveChatEndedError(err error) bool {
	if err == nil {
		return false
	}

	var googleErr *googleapi.Error
	if !errors.As(err, &googleErr) {
		return false
	}

	if googleErr.Code != 403 {
		return false
	}

	for _, item := range googleErr.Errors {
		if item.Reason == "liveChatEnded" {
			return true
		}
	}

	if len(googleErr.Errors) > 0 {
		return false
	}

	return strings.Contains(googleErr.Message, "liveChatEnded") ||
		strings.Contains(googleErr.Body, "liveChatEnded")
}
