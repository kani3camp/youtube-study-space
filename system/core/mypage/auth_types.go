package mypage

import "time"

type AuthenticatedFirebaseUser struct {
	FirebaseUID string
}

type LinkYouTubeRequest struct {
	YouTubeAccessToken string `json:"youtubeAccessToken"`
}

type LinkYouTubeResponse struct {
	Status string `json:"status"`
	Viewer Viewer `json:"viewer"`
}

type LinkedYouTubeAccount struct {
	YouTubeChannelID string
	DisplayName      string
	ProfileImageURL  string
	LinkedAt         time.Time
	UpdatedAt        time.Time
}
