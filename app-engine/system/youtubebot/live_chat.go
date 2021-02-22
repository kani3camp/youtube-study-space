package youtubebot

import (
	"context"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

func NewYoutubeLiveChatBot(liveChatId string, sleepIntervalMilli int, ctx context.Context) (*YoutubeLiveChatBot, error) {
	// todo get credential properly
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/music-quiz-287112-83a452727d6d.json")
	youtubeService, err := youtube.NewService(ctx, clientOption)
	if err != nil {
		return nil, err
	}
	liveChatMessagesService := youtube.NewLiveChatMessagesService(youtubeService)

	return &YoutubeLiveChatBot{
		LiveChatId:                liveChatId,
		YoutubeService:            youtubeService,
		LiveChatMessagesService:   liveChatMessagesService,
		DefaultSleepIntervalMilli: sleepIntervalMilli,
	}, nil
}

func (bot *YoutubeLiveChatBot) ListMessages(nextPageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	part := []string{
		"snippet",
	}
	listCall := bot.LiveChatMessagesService.List(bot.LiveChatId, part)
	if nextPageToken != "" {
		listCall = listCall.PageToken(nextPageToken)
	}
	response, err := listCall.Do()
	if err != nil {
		return nil, "", 0, err
	}
	return response.Items, response.NextPageToken, int(response.PollingIntervalMillis), nil
}

func (bot *YoutubeLiveChatBot) PostMessage(message string)  {
	// todo 送れなかった場合はlineで通知
}
