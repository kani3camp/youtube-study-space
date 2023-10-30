package youtubebot

import (
	"app.modules/core/myfirestore"
	"cloud.google.com/go/firestore"
	"context"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log"
	"strconv"
	"unicode/utf8"
)

const MaxLiveChatMessageLength = 200

type AccessTokenResponseStruct struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

func NewYoutubeLiveChatBot(liveChatId string, controller *myfirestore.FirestoreController, ctx context.Context) (*YoutubeLiveChatBot, error) {
	var channelYoutubeService *youtube.Service
	var botYoutubeService *youtube.Service
	
	txErr := controller.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		credentials, err := controller.ReadCredentialsConfig(ctx, tx)
		if err != nil {
			return err
		}
		
		// channel
		channelConfig := &oauth2.Config{
			ClientID:     credentials.YoutubeChannelClientId,
			ClientSecret: credentials.YoutubeChannelClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL: "https://accounts.google.com/o/oauth2/token",
			},
		}
		channelToken := &oauth2.Token{
			TokenType:    "Bearer",
			RefreshToken: credentials.YoutubeChannelRefreshToken,
		}
		channelTokenSource := channelConfig.TokenSource(ctx, channelToken)
		channelClientOption := option.WithTokenSource(channelTokenSource)
		channelYoutubeService, err = youtube.NewService(ctx, channelClientOption)
		if err != nil {
			return err
		}
		
		// bot
		botConfig := &oauth2.Config{
			ClientID:     credentials.YoutubeBotClientId,
			ClientSecret: credentials.YoutubeBotClientSecret,
			Endpoint: oauth2.Endpoint{
				TokenURL: "https://accounts.google.com/o/oauth2/token",
			},
		}
		botToken := &oauth2.Token{
			TokenType:    "Bearer",
			RefreshToken: credentials.YoutubeBotRefreshToken,
		}
		botTokenSource := botConfig.TokenSource(ctx, botToken)
		botClientOption := option.WithTokenSource(botTokenSource)
		botYoutubeService, err = youtube.NewService(ctx, botClientOption)
		if err != nil {
			return err
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	
	return &YoutubeLiveChatBot{
		LiveChatId:            liveChatId,
		ChannelYoutubeService: channelYoutubeService,
		BotYoutubeService:     botYoutubeService,
		FirestoreController:   controller,
	}, nil
}

func (b *YoutubeLiveChatBot) ListMessages(ctx context.Context, nextPageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	liveChatMessageService := youtube.NewLiveChatMessagesService(b.BotYoutubeService)
	part := []string{
		"snippet",
		"authorDetails",
	}
	
	// first call
	listCall := liveChatMessageService.List(b.LiveChatId, part)
	if nextPageToken != "" {
		listCall = listCall.PageToken(nextPageToken)
	}
	response, err := listCall.Do()
	if err != nil {
		log.Println("first call failed in ListMessages().")
		log.Println(err)
		
		// errのステータスコードを確認
		gerr, ok := err.(*googleapi.Error)
		if !ok {
			return nil, "", 0, errors.New("failed to cast error to googleapi.Error")
		}
		switch gerr.Code {
		case 403:
			fallthrough
		case 404:
			// live chat idが変わっている可能性があるため、更新して再試行
			err := b.refreshLiveChatId(ctx)
			if err != nil {
				return nil, "", 0, err
			}
		case 500:
			return nil, "", 0, nil
		default:
			log.Println("unknown status code: " + strconv.Itoa(gerr.Code))
			return nil, "", 0, err
		}
		
		// second call
		log.Println("trying second call in ListMessages()...")
		listCall := liveChatMessageService.List(b.LiveChatId, part)
		if nextPageToken != "" {
			listCall = listCall.PageToken(nextPageToken)
		}
		response, err = listCall.Do()
		if err != nil {
			log.Println("second call failed in ListMessages().")
			return nil, "", 0, err
		}
	}
	return response.Items, response.NextPageToken, int(response.PollingIntervalMillis), nil
}

func (b *YoutubeLiveChatBot) PostMessage(ctx context.Context, message string) error {
	log.Println("sending a message to Youtube Live \"" + message + "\"")
	
	if utf8.RuneCountInString(message) <= MaxLiveChatMessageLength {
		return b.postMessage(ctx, message)
	}
	var messages []string
	for {
		if utf8.RuneCountInString(message) <= MaxLiveChatMessageLength {
			messages = append(messages, message)
			break
		}
		var p int // 文字列中のインデックス
		for i := range message {
			if utf8.RuneCountInString(message[:i]) > MaxLiveChatMessageLength {
				break
			}
			p = i
		}
		
		// リストに追加
		messages = append(messages, message[:p])
		message = message[p:]
	}
	for _, m := range messages {
		err := b.postMessage(ctx, m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *YoutubeLiveChatBot) postMessage(ctx context.Context, message string) error {
	if len(message) == 0 {
		return errors.New("message length is 0.")
	}
	
	part := []string{"snippet"}
	liveChatMessage := youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			DisplayMessage: message,
			LiveChatId:     b.LiveChatId,
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: message,
			},
			Type: "textMessageEvent",
		},
	}
	liveChatMessageService := youtube.NewLiveChatMessagesService(b.BotYoutubeService)
	insertCall := liveChatMessageService.Insert(part, &liveChatMessage)
	
	// first call
	_, err := insertCall.Do()
	if err != nil {
		log.Println("first post was failed", err)
		// second call
		_, err := insertCall.Do()
		if err == nil {
			log.Println("second post succeeded!")
			return nil
		}
		log.Println("second post was failed", err)
		
		// live chat idが変わっている可能性があるため、更新して再試行
		err = b.refreshLiveChatId(ctx)
		if err != nil {
			return err
		}
		
		// third call
		liveChatMessage.Snippet.LiveChatId = b.LiveChatId
		liveChatMessageService = youtube.NewLiveChatMessagesService(b.BotYoutubeService)
		insertCall = liveChatMessageService.Insert(part, &liveChatMessage)
		_, err = insertCall.Do()
		if err != nil {
			log.Println("third post was failed")
			return err
		}
		log.Println("third post succeeded!")
	}
	
	return nil
}

// refreshLiveChatId live chat idを取得するとともに、firestoreに保存（更新）する
func (b *YoutubeLiveChatBot) refreshLiveChatId(ctx context.Context) error {
	log.Println("refreshLiveChatId()")
	broadCastsService := youtube.NewLiveBroadcastsService(b.ChannelYoutubeService)
	part := []string{"snippet"}
	listCall := broadCastsService.List(part).BroadcastStatus("active")
	response, err := listCall.Do()
	if err != nil {
		log.Println("first call failed in refreshLiveChatId().")
		log.Println("trying second call in refreshLiveChatId()...")
		broadCastsService = youtube.NewLiveBroadcastsService(b.ChannelYoutubeService)
		listCall = broadCastsService.List(part).BroadcastStatus("active")
		response, err = listCall.Do()
		if err != nil {
			return err
		}
	}
	if len(response.Items) == 1 {
		newLiveChatId := response.Items[0].Snippet.LiveChatId
		log.Println("live chat id :", newLiveChatId)
		err := b.FirestoreController.UpdateLiveChatId(ctx, nil, newLiveChatId)
		if err != nil {
			return err
		}
		b.LiveChatId = newLiveChatId
		return nil
	} else if len(response.Items) == 0 {
		log.Println("ライブ1個もやってない（1回目）")
		
		// たまに、配信してるのにこの結果になることがあるかも（未確認）しれないので、もう一度。
		broadCastsService := youtube.NewLiveBroadcastsService(b.ChannelYoutubeService)
		part := []string{"snippet"}
		listCall := broadCastsService.List(part).BroadcastStatus("active")
		response, err := listCall.Do()
		if err != nil {
			log.Println("first call failed in refreshLiveChatId().")
			log.Println("trying second call in refreshLiveChatId()...")
			broadCastsService = youtube.NewLiveBroadcastsService(b.ChannelYoutubeService)
			listCall = broadCastsService.List(part).BroadcastStatus("active")
			response, err = listCall.Do()
			if err != nil {
				return err
			}
		}
		if len(response.Items) == 1 {
			newLiveChatId := response.Items[0].Snippet.LiveChatId
			log.Println("live chat id :", newLiveChatId)
			err := b.FirestoreController.UpdateLiveChatId(ctx, nil, newLiveChatId)
			if err != nil {
				return err
			}
			b.LiveChatId = newLiveChatId
			return nil
		} else if len(response.Items) == 0 {
			return errors.New("2回試したけどライブ1個もやってない")
		} else {
			return errors.New("more than 2 live broadcasts!: " + strconv.Itoa(len(response.Items)))
		}
	} else {
		return errors.New("more than 2 live broadcasts!: " + strconv.Itoa(len(response.Items)))
	}
}

// BanUser 指定したユーザー（Youtubeチャンネル）をブロックする。
func (b *YoutubeLiveChatBot) BanUser(ctx context.Context, userId string) error {
	err := b.banRequest(ctx, b.LiveChatId, userId)
	// first call
	if err != nil {
		log.Println("first banRequest was failed", err)
		
		// live chat idが変わっている可能性があるため、更新して再試行
		err := b.refreshLiveChatId(ctx)
		if err != nil {
			return err
		}
		
		// second call
		err = b.banRequest(ctx, b.LiveChatId, userId)
		if err != nil {
			log.Println("second banRequest was failed")
			return err
		}
	}
	return nil
}

func (b *YoutubeLiveChatBot) banRequest(ctx context.Context, liveChatId string, userId string) error {
	part := []string{"snippet"}
	liveChatBan := youtube.LiveChatBan{
		Snippet: &youtube.LiveChatBanSnippet{
			LiveChatId: liveChatId,
			Type:       "permanent",
			BannedUserDetails: &youtube.ChannelProfileDetails{
				ChannelId: userId,
			},
		},
	}
	liveChatBanService := youtube.NewLiveChatBansService(b.BotYoutubeService)
	insertCall := liveChatBanService.Insert(part, &liveChatBan)
	
	_, err := insertCall.Do()
	return err
}
