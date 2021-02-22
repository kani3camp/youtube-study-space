package youtubebot

import (
	"google.golang.org/api/youtube/v3"
)

type YoutubeLiveChatBot struct {
	LiveChatId string
	YoutubeService *youtube.Service
	LiveChatMessagesService *youtube.LiveChatMessagesService
	SleepIntervalMilli int
	NextPageToken string
}

//type LiveChatMessage struct {
//	AuthorChannelId string
//	Message string
//	PublishedAt time.Time
//}

