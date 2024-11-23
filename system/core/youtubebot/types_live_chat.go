package youtubebot

import (
	"app.modules/core/myfirestore"
	"google.golang.org/api/youtube/v3"
)

type YoutubeLiveChatBot struct {
	LiveChatId            string
	ChannelYoutubeService *youtube.Service
	BotYoutubeService     *youtube.Service
	FirestoreController   myfirestore.FirestoreController
}
