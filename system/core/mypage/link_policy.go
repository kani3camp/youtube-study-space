package mypage

import (
	"fmt"
	"strings"
)

// validateChannelLinkOwnership reports whether youtubeChannelID may be linked to firebaseUID
// when existingOwnerUID is the Firebase UID that already owns the channel (empty if none).
func validateChannelLinkOwnership(youtubeChannelID, firebaseUID, existingOwnerUID string) error {
	youtubeChannelID = strings.TrimSpace(youtubeChannelID)
	if youtubeChannelID == "" {
		return fmt.Errorf("%w: youtube channel id is empty", ErrInvalidRequest)
	}

	firebaseUID = strings.TrimSpace(firebaseUID)
	if firebaseUID == "" {
		return ErrUnauthorized
	}

	existingOwnerUID = strings.TrimSpace(existingOwnerUID)
	if existingOwnerUID == "" || existingOwnerUID == firebaseUID {
		return nil
	}

	return ErrYouTubeChannelAlreadyLinked
}
