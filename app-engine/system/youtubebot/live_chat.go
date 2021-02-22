package youtubebot

import (
	"app.modules/system/customerror"
	"context"
	"fmt"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"time"
)

func NewYoutubeLiveChatBot(liveChatId string, sleepIntervalMilli int, ctx context.Context) (*YoutubeLiveChatBot, customerror.CustomError) {
	// todo get credential properly
	clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/music-quiz-287112-83a452727d6d.json")
	youtubeService, err := youtube.NewService(ctx, clientOption)
	if err != nil {
		return nil, customerror.YoutubeLiveChatBotFailed.Wrap(err, "failed youtube.NewService()")
	}
	liveChatMessagesService := youtube.NewLiveChatMessagesService(youtubeService)

	return &YoutubeLiveChatBot{
		LiveChatId:     liveChatId,
		YoutubeService: youtubeService,
		LiveChatMessagesService: liveChatMessagesService,
		SleepIntervalMilli: sleepIntervalMilli,
		NextPageToken: "",
	}, customerror.CustomError{Body: nil}
}

func (bot *YoutubeLiveChatBot) ListMessages() (*[]youtube.LiveChatMessage, error) {
	part := []string{
		"snippet",
	}
	listCall := bot.LiveChatMessagesService.List(bot.LiveChatId, part)
	if bot.NextPageToken != "" {
		listCall = listCall.PageToken(bot.NextPageToken)
	}
	response, err := listCall.Do()
	if err != nil {
		return nil, err
	}
	for _, item := range response.Items {
		fmt.Println(item.Snippet.DisplayMessage)
	}
	bot.NextPageToken = response.NextPageToken

	return , nil
}

func (bot *YoutubeLiveChatBot) PostMessage(message string)  {

}
