package youtubebot

import (
	"google.golang.org/api/youtube/v3"
)

type YoutubeLiveChatBot struct {
	LiveChatId                string
	YoutubeService            *youtube.Service
	LiveChatMessagesService   *youtube.LiveChatMessagesService
	DefaultSleepIntervalMilli int
}

//type LiveChatMessage struct {
//	AuthorChannelId string
//	Message string
//	PublishedAt time.Time
//}

