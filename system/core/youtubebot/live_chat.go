package youtubebot

import (
	"app.modules/core/myfirestore"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
	credentials, err := controller.RetrieveCredentialsConfig(ctx, nil)
	if err != nil {
		return nil, err
	}
	config := &oauth2.Config{
		ClientID:     credentials.YoutubeChannelClientId,
		ClientSecret: credentials.YoutubeChannelClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/auth",
			TokenURL:  "https://accounts.google.com/o/oauth2/token",
			AuthStyle: 0,
		},
		RedirectURL: "https://youtube.com/",
		Scopes:      nil,
	}
	channelOauthToken := &oauth2.Token{
		AccessToken:  credentials.YoutubeChannelAccessToken,
		TokenType:    "Bearer",
		RefreshToken: credentials.YoutubeChannelRefreshToken,
		Expiry:       credentials.YoutubeChannelExpirationDate,
	}
	channelClientOption := option.WithTokenSource(config.TokenSource(ctx, channelOauthToken))
	channelYoutubeService, err := youtube.NewService(ctx, channelClientOption)
	if err != nil {
		return nil, err
	}
	
	botOauthToken := &oauth2.Token{
		AccessToken:  credentials.YoutubeBotAccessToken,
		TokenType:    "Bearer",
		RefreshToken: credentials.YoutubeBotRefreshToken,
		Expiry:       credentials.YoutubeBotExpirationDate,
	}
	botClientOption := option.WithTokenSource(config.TokenSource(ctx, botOauthToken))
	botYoutubeService, err := youtube.NewService(ctx, botClientOption)
	if err != nil {
		return nil, err
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
		// b credentialのaccess tokenが期限切れの可能性
		credentialConfig, err := b.FirestoreController.RetrieveCredentialsConfig(ctx, nil)
		if err != nil {
			return nil, "", 0, err
		}
		if credentialConfig.YoutubeBotExpirationDate.Before(utils.JstNow()) {
			// access tokenが期限切れのため、更新する
			err := b.refreshBotAccessToken(ctx, nil)
			if err != nil {
				return nil, "", 0, err
			}
		} else {
			// live chat idが変わっている可能性があるため、更新して再試行
			err := b.refreshLiveChatId(ctx)
			if err != nil {
				return nil, "", 0, err
			}
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

func (b *YoutubeLiveChatBot) PostMessage(ctx context.Context, tx *firestore.Transaction, message string) error {
	log.Println("sending a message to Youtube Live \"" + message + "\"")
	
	if utf8.RuneCountInString(message) <= MaxLiveChatMessageLength {
		return b.postMessage(ctx, tx, message)
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
		err := b.postMessage(ctx, tx, m)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *YoutubeLiveChatBot) postMessage(ctx context.Context, tx *firestore.Transaction, message string) error {
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
		
		// b credentialのaccess tokenが期限切れの可能性
		credentialConfig, err := b.FirestoreController.RetrieveCredentialsConfig(ctx, nil)
		if err != nil {
			return err
		}
		if credentialConfig.YoutubeBotExpirationDate.Before(utils.JstNow()) {
			// access tokenが期限切れのため、更新する
			err := b.refreshBotAccessToken(ctx, tx)
			if err != nil {
				return err
			}
		} else {
			// live chat idが変わっている可能性があるため、更新して再試行
			err := b.refreshLiveChatId(ctx)
			if err != nil {
				return err
			}
		}
		
		// second call
		liveChatMessage.Snippet.LiveChatId = b.LiveChatId
		liveChatMessageService = youtube.NewLiveChatMessagesService(b.BotYoutubeService)
		insertCall = liveChatMessageService.Insert(part, &liveChatMessage)
		_, err = insertCall.Do()
		if err != nil {
			log.Println("second post was failed")
			return err
		}
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
		// channel credentialのaccess tokenを更新する必要がある可能性
		log.Println("first call failed in refreshLiveChatId().")
		err := b.refreshChannelAccessToken(ctx)
		if err != nil {
			return err
		}
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
		err := b.FirestoreController.SaveLiveChatId(ctx, nil, newLiveChatId)
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
			// channel credentialのaccess tokenを更新する必要がある可能性
			log.Println("first call failed in refreshLiveChatId().")
			err := b.refreshChannelAccessToken(ctx)
			if err != nil {
				return err
			}
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
			err := b.FirestoreController.SaveLiveChatId(ctx, nil, newLiveChatId)
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

func (b *YoutubeLiveChatBot) refreshChannelAccessToken(ctx context.Context) error {
	log.Println("refreshChannelAccessToken()")
	return b.FirestoreController.FirestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		credentialConfig, err := b.FirestoreController.RetrieveCredentialsConfig(ctx, nil)
		if err != nil {
			return err
		}
		
		newAccessToken, newExpirationDate, err := b.refreshAccessToken(
			ctx,
			credentialConfig.YoutubeChannelClientId,
			credentialConfig.YoutubeChannelClientSecret,
			credentialConfig.YoutubeChannelRefreshToken,
		)
		if err != nil {
			return err
		}
		// 更新
		config := &oauth2.Config{
			ClientID:     credentialConfig.YoutubeChannelClientId,
			ClientSecret: credentialConfig.YoutubeChannelClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://accounts.google.com/o/oauth2/auth",
				TokenURL:  "https://accounts.google.com/o/oauth2/token",
				AuthStyle: 0,
			},
			RedirectURL: "https://youtube.com/",
			Scopes:      nil,
		}
		channelOauthToken := &oauth2.Token{
			AccessToken:  newAccessToken,
			TokenType:    "Bearer",
			RefreshToken: credentialConfig.YoutubeChannelRefreshToken,
			Expiry:       newExpirationDate,
		}
		channelClientOption := option.WithTokenSource(config.TokenSource(ctx, channelOauthToken))
		newService, err := youtube.NewService(ctx, channelClientOption)
		if err != nil {
			return err
		}
		b.ChannelYoutubeService = newService
		
		// Firestoreに保存
		err = b.FirestoreController.SetAccessTokenOfChannelCredential(tx, newAccessToken, newExpirationDate)
		if err != nil {
			return err
		}
		return nil
	})
}

func (b *YoutubeLiveChatBot) refreshBotAccessToken(ctx context.Context, tx *firestore.Transaction) error {
	log.Println("refreshBotAccessToken()")
	credentialConfig, err := b.FirestoreController.RetrieveCredentialsConfig(ctx, nil)
	if err != nil {
		return err
	}
	
	newAccessToken, newExpirationDate, err := b.refreshAccessToken(
		ctx,
		credentialConfig.YoutubeBotClientId,
		credentialConfig.YoutubeBotClientSecret,
		credentialConfig.YoutubeBotRefreshToken)
	if err != nil {
		return err
	}
	// 更新
	config := &oauth2.Config{
		ClientID:     credentialConfig.YoutubeBotClientId,
		ClientSecret: credentialConfig.YoutubeBotClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/auth",
			TokenURL:  "https://accounts.google.com/o/oauth2/token",
			AuthStyle: 0,
		},
		RedirectURL: "https://youtube.com/",
		Scopes:      nil,
	}
	botOauthToken := &oauth2.Token{
		AccessToken:  newAccessToken,
		TokenType:    "Bearer",
		RefreshToken: credentialConfig.YoutubeBotRefreshToken,
		Expiry:       newExpirationDate,
	}
	botClientOption := option.WithTokenSource(config.TokenSource(ctx, botOauthToken))
	newService, err := youtube.NewService(ctx, botClientOption)
	if err != nil {
		return err
	}
	b.ChannelYoutubeService = newService
	
	// Firestoreに保存
	err = b.FirestoreController.SetAccessTokenOfBotCredential(ctx, tx, newAccessToken, newExpirationDate)
	if err != nil {
		return err
	}
	return nil
}

func (b *YoutubeLiveChatBot) refreshAccessToken(ctx context.Context, clientId string, clientSecret string, refreshToken string) (string, time.Time, error) {
	log.Println("refreshAccessToken()")
	credentialsConfig, err := b.FirestoreController.RetrieveCredentialsConfig(ctx, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	data := url.Values{}
	data.Set("client_id", clientId)
	data.Add("client_secret", clientSecret)
	data.Add("refresh_token", refreshToken)
	data.Add("grant_type", "refresh_token")
	
	req, err := http.NewRequest(
		http.MethodPost,
		credentialsConfig.OAuthRefreshTokenUrl,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", time.Time{}, err
	}
	if req != nil {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	if resp != nil {
		defer func() {
			_ = resp.Body.Close()
		}()
		
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", time.Time{}, err
		}
		
		var responseBody AccessTokenResponseStruct
		err = json.Unmarshal(body, &responseBody)
		if err != nil {
			return "", time.Time{}, err
		}
		log.Println(string(body))
		newAccessToken := responseBody.AccessToken
		log.Println("new access token: " + newAccessToken)
		
		newExpirationDate := utils.JstNow().Add(time.Duration(responseBody.ExpiresIn) * time.Second)
		return newAccessToken, newExpirationDate, nil
	} else {
		return "", time.Time{}, err
	}
}
