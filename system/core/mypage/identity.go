package mypage

import (
	"context"
	"net/http"
)

type Identity struct {
	FirebaseUID      string
	YouTubeChannelID string
	DisplayName      string
	ProfileImageURL  string
}

type IdentityResolver interface {
	Resolve(ctx context.Context, r *http.Request) (Identity, error)
}
