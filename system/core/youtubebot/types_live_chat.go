package youtubebot

import (
	"context"

	"app.modules/core/repository"
	"google.golang.org/api/youtube/v3"
)

type LiveChatBot interface {
	ListMessages(ctx context.Context, nextPageToken string) ([]*youtube.LiveChatMessage, string, int, error)
	PostMessage(ctx context.Context, message string) error
	BanUser(ctx context.Context, userID string) error
}

type YoutubeLiveChatBot struct {
	LiveChatID            string
	ChannelYoutubeService *youtube.Service
	BotYoutubeService     *youtube.Service
	FirestoreController   repository.Repository
}
