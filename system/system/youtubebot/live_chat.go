package youtubebot

import (
	"app.modules/system/myfirestore"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"strconv"
)

func NewYoutubeLiveChatBot(liveChatId string, controller *myfirestore.FirestoreController, ctx context.Context) (*YoutubeLiveChatBot, error) {
	//clientOption := option.WithCredentialsFile("/Users/drew/Development/機密ファイル/GCP/youtube-study-space-c4bcd4edbd8a.json")
	//clientOption := option.WithCredentialsFile("C:/Development/GCP Credentials/music-quiz-287112-83a452727d6d.json")
	
	channelCredential, err := controller.RetrieveYoutubeChannelCredentialConfig(ctx)
	if err != nil {
		return nil, err
	}
	config := &oauth2.Config{
		ClientID:     channelCredential.ClientId,
		ClientSecret: channelCredential.ClientSecret,
		Endpoint:     oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/auth",
			TokenURL:  "https://accounts.google.com/o/oauth2/token",
			AuthStyle: 0,
		},
		RedirectURL:  "https://youtube.com/",
		Scopes:       nil,
	}
	channelOauthToken := &oauth2.Token{
		AccessToken:  channelCredential.AccessToken,
		TokenType:    "Bearer",
		RefreshToken: channelCredential.RefreshToken,
		Expiry:       channelCredential.ExpirationDate,
	}
	channelClientOption := option.WithTokenSource(config.TokenSource(ctx, channelOauthToken))
	channelYoutubeService, err := youtube.NewService(ctx, channelClientOption)
	if err != nil {
		return nil, err
	}
	
	botCredential, err := controller.RetrieveYoutubeBotCredentialConfig(ctx)
	if err != nil {
		return nil, err
	}
	botOauthToken := &oauth2.Token{
		AccessToken:  botCredential.AccessToken,
		TokenType:    "Bearer",
		RefreshToken: botCredential.RefreshToken,
		Expiry:       botCredential.ExpirationDate,
	}
	botClientOption := option.WithTokenSource(config.TokenSource(ctx, botOauthToken))
	botYoutubeService, err := youtube.NewService(ctx, botClientOption)
	if err != nil {
		return nil, err
	}
	
	return &YoutubeLiveChatBot{
		LiveChatId:                liveChatId,
		ChannelYoutubeService: channelYoutubeService,
		BotYoutubeService: botYoutubeService,
		FirestoreController: controller,
	}, nil
}

func (bot *YoutubeLiveChatBot) ListMessages(nextPageToken string, ctx context.Context) ([]*youtube.LiveChatMessage, string, int, error) {
	fmt.Println("ListMessages()")
	liveChatMessageService := youtube.NewLiveChatMessagesService(bot.BotYoutubeService)
	part := []string{
		"snippet",
		"authorDetails",
	}
	
	// first call
	listCall := liveChatMessageService.List(bot.LiveChatId, part)
	if nextPageToken != "" {
		listCall = listCall.PageToken(nextPageToken)
	}
	response, err := listCall.Do()
	if err != nil {
		// live chat idが変わっている可能性があるため、更新して再試行
		err := bot.RefreshLiveChatId(ctx)
		if err != nil {
			return nil, "", 0, err
		}
		// second call
		listCall := liveChatMessageService.List(bot.LiveChatId, part)
		if nextPageToken != "" {
			listCall = listCall.PageToken(nextPageToken)
		}
		response, err = listCall.Do()
		if err != nil {
			return nil, "", 0, err
		}
	}
	return response.Items, response.NextPageToken, int(response.PollingIntervalMillis), nil
}

func (bot *YoutubeLiveChatBot) PostMessage(message string, ctx context.Context) error {
	fmt.Println("sending a message to Youtube Live \"" + message + "\"")
	// todo 送れなかった場合はlineで通知
	// acces token 読み込み
	// expire確認
	// post1
	part := []string{"snippet"}
	liveChatMessage := youtube.LiveChatMessage{
		Snippet:         &youtube.LiveChatMessageSnippet{
			DisplayMessage:          message,
			LiveChatId:              bot.LiveChatId,
			TextMessageDetails:      &youtube.LiveChatTextMessageDetails{
				MessageText:     message,
			},
			Type:                    "textMessageEvent",
		},
	}
	liveChatMessageService := youtube.NewLiveChatMessagesService(bot.BotYoutubeService)
	insertCall := liveChatMessageService.Insert(part, &liveChatMessage)
	_, err := insertCall.Do()
	if err != nil {
		fmt.Println("first post was failed")
		// post2
		err := bot.RefreshLiveChatId(ctx)
		if err != nil {
			return err
		}
		liveChatMessage.Snippet.LiveChatId = bot.LiveChatId
		liveChatMessageService = youtube.NewLiveChatMessagesService(bot.BotYoutubeService)
		insertCall = liveChatMessageService.Insert(part, &liveChatMessage)
		_, err = insertCall.Do()
		if err != nil {
			fmt.Println("second post was failed")
			return err
		}
	}
	//if config.ExpireDate.Before(time.Now()) {
	//	log.Println("access token is expired. refreshing...")
	//	_ = RefreshAccessToken(&config, client, ctx)
	//}
	
	return nil
}

func (bot *YoutubeLiveChatBot) RefreshAccessToken() error {
	fmt.Println("RefreshAccessToken()")
	// todo
	return nil
}

// RefreshLiveChatId: live chat idを取得するとともに、firestoreに保存（更新）する
func (bot *YoutubeLiveChatBot) RefreshLiveChatId(ctx context.Context) error {
	fmt.Println("RefreshLiveChatId()")
	broadCastsService := youtube.NewLiveBroadcastsService(bot.ChannelYoutubeService)
	part := []string{"snippet"}
	listCall := broadCastsService.List(part).BroadcastStatus("active")
	response, err := listCall.Do()
	if err != nil {
		return err
	}
	if len(response.Items) == 1 {
		newLiveChatId := response.Items[0].Snippet.LiveChatId
		fmt.Println("live chat id :", newLiveChatId)
		err := bot.FirestoreController.SaveLiveChatId(newLiveChatId, ctx)
		if err != nil {
			return err
		}
		bot.LiveChatId = newLiveChatId
		return nil
	} else if len(response.Items) == 0 {
		return errors.New("there are no live broadcast!")
	} else {
		return errors.New("more than 2 live broadcasts!: " + strconv.Itoa(len(response.Items)))
	}
}


