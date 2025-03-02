package youtubebot

import (
	"app.modules/core/repository"
	"context"
	"google.golang.org/api/youtube/v3"
)

type LiveChatBot interface {
	ListMessages(ctx context.Context, nextPageToken string) ([]*youtube.LiveChatMessage, string, int, error)
	PostMessage(ctx context.Context, message string) error
	BanUser(ctx context.Context, userId string) error
}

type YoutubeLiveChatBot struct {
	LiveChatId            string
	ChannelYoutubeService *youtube.Service
	BotYoutubeService     *youtube.Service
	FirestoreController   repository.Repository
}
