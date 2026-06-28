package mypage

import (
	"context"
	"fmt"
	"strings"
)

type FirebaseAuthenticator interface {
	Authenticate(ctx context.Context, firebaseIDTokenRequest FirebaseIDTokenRequest) (AuthenticatedFirebaseUser, error)
}

type FirebaseIDTokenRequest interface {
	AuthorizationHeader() string
}

type YouTubeChannelFetcher interface {
	FetchMyChannel(ctx context.Context, youtubeAccessToken string) (Viewer, error)
}

type LinkedAccountStore interface {
	FindFirebaseUIDByYouTubeChannelID(ctx context.Context, youtubeChannelID string) (string, error)
	SaveLinkedYouTubeAccount(ctx context.Context, firebaseUID string, viewer Viewer) error
}

func (s *Service) LinkYouTube(
	ctx context.Context,
	authenticatedUser AuthenticatedFirebaseUser,
	youtubeAccessToken string,
	channelFetcher YouTubeChannelFetcher,
	linkedAccountStore LinkedAccountStore,
) (LinkYouTubeResponse, error) {
	if authenticatedUser.FirebaseUID == "" {
		return LinkYouTubeResponse{}, ErrUnauthorized
	}

	youtubeAccessToken = strings.TrimSpace(youtubeAccessToken)
	if youtubeAccessToken == "" {
		return LinkYouTubeResponse{}, fmt.Errorf("%w: youtubeAccessToken is empty", ErrInvalidRequest)
	}

	viewer, err := channelFetcher.FetchMyChannel(ctx, youtubeAccessToken)
	if err != nil {
		return LinkYouTubeResponse{}, fmt.Errorf("fetch my youtube channel: %w", err)
	}

	if viewer.YouTubeChannelID == "" {
		return LinkYouTubeResponse{}, fmt.Errorf("%w: youtube channel id is empty", ErrInvalidYouTubeAccessToken)
	}

	existingOwnerUID, err := linkedAccountStore.FindFirebaseUIDByYouTubeChannelID(ctx, viewer.YouTubeChannelID)
	if err != nil {
		return LinkYouTubeResponse{}, fmt.Errorf("find linked youtube account owner: %w", err)
	}

	if err := validateChannelLinkOwnership(
		viewer.YouTubeChannelID,
		authenticatedUser.FirebaseUID,
		existingOwnerUID,
	); err != nil {
		return LinkYouTubeResponse{}, err
	}

	if err := linkedAccountStore.SaveLinkedYouTubeAccount(ctx, authenticatedUser.FirebaseUID, viewer); err != nil {
		return LinkYouTubeResponse{}, fmt.Errorf("save linked youtube account: %w", err)
	}

	return LinkYouTubeResponse{
		Status: "ok",
		Viewer: viewer,
	}, nil
}
