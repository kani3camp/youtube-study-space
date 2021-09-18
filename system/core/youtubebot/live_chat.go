package youtubebot

import (
	"app.modules/core/myfirestore"
	"app.modules/core/utils"
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
)


type AccessTokenResponseStruct struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}


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
	log.Println("ListMessages()")
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
		log.Println("first call failed in ListMessages().")
		// bot credentialのaccess tokenが期限切れの可能性
		botCredentialConfig, err := bot.FirestoreController.RetrieveYoutubeBotCredentialConfig(ctx)
		if err != nil {
			return nil, "", 0, err
		}
		if botCredentialConfig.ExpirationDate.Before(utils.JstNow()) {
			// access tokenが期限切れのため、更新する
			err := bot.RefreshBotAccessToken(ctx)
			if err != nil {
				return nil, "", 0, err
			}
		} else {
			// live chat idが変わっている可能性があるため、更新して再試行
			err := bot.RefreshLiveChatId(ctx)
			if err != nil {
				return nil, "", 0, err
			}
		}
		// second call
		log.Println("trying second call in ListMessages()...")
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
	log.Println("sending a message to Youtube Live \"" + message + "\"")
	// todo 送れなかった場合はlineで通知
	// first call
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
		log.Println("first post was failed")
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
			log.Println("second post was failed")
			return err
		}
	}
	//if config.ExpireDate.Before(core.JstNow()) {
	//	log.Println("access token is expired. refreshing...")
	//	_ = RefreshChannelAccessToken(&config, client, ctx)
	//}
	
	return nil
}

// RefreshLiveChatId live chat idを取得するとともに、firestoreに保存（更新）する
func (bot *YoutubeLiveChatBot) RefreshLiveChatId(ctx context.Context) error {
	log.Println("RefreshLiveChatId()")
	broadCastsService := youtube.NewLiveBroadcastsService(bot.ChannelYoutubeService)
	part := []string{"snippet"}
	listCall := broadCastsService.List(part).BroadcastStatus("active")
	response, err := listCall.Do()
	if err != nil {
		// channel credentialのaccess tokenを更新する必要がある可能性
		log.Println("first call failed in RefreshLiveChatId().")
		err := bot.RefreshChannelAccessToken(ctx)
		if err != nil {
			return err
		}
		log.Println("trying second call in RefreshLiveChatId()...")
		broadCastsService = youtube.NewLiveBroadcastsService(bot.ChannelYoutubeService)
		listCall = broadCastsService.List(part).BroadcastStatus("active")
		response, err = listCall.Do()
		if err != nil {
			return err
		}
	}
	if len(response.Items) == 1 {
		newLiveChatId := response.Items[0].Snippet.LiveChatId
		log.Println("live chat id :", newLiveChatId)
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

func (bot *YoutubeLiveChatBot) RefreshChannelAccessToken(ctx context.Context) error {
	log.Println("RefreshChannelAccessToken()")
	channelCredentialConfig, err := bot.FirestoreController.RetrieveYoutubeChannelCredentialConfig(ctx)
	if err != nil {
		return err
	}
	
	newAccessToken, newExpirationDate, err := bot._RefreshAccessToken(
		channelCredentialConfig.ClientId,
		channelCredentialConfig.ClientSecret,
		channelCredentialConfig.RefreshToken,
		ctx)
	if err != nil {
		return err
	}
	// 更新
	config := &oauth2.Config{
		ClientID:     channelCredentialConfig.ClientId,
		ClientSecret: channelCredentialConfig.ClientSecret,
		Endpoint:     oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/auth",
			TokenURL:  "https://accounts.google.com/o/oauth2/token",
			AuthStyle: 0,
		},
		RedirectURL:  "https://youtube.com/",
		Scopes:       nil,
	}
	channelOauthToken := &oauth2.Token{
		AccessToken:  newAccessToken,
		TokenType:    "Bearer",
		RefreshToken: channelCredentialConfig.RefreshToken,
		Expiry:       newExpirationDate,
	}
	channelClientOption := option.WithTokenSource(config.TokenSource(ctx, channelOauthToken))
	newService, err := youtube.NewService(ctx, channelClientOption)
	if err != nil {
		return err
	}
	bot.ChannelYoutubeService = newService
	
	// Firestoreに保存
	err = bot.FirestoreController.SetAccessTokenOfChannelCredential(newAccessToken, newExpirationDate, ctx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *YoutubeLiveChatBot) RefreshBotAccessToken(ctx context.Context) error {
	log.Println("RefreshBotAccessToken()")
	botCredentialConfig, err := bot.FirestoreController.RetrieveYoutubeBotCredentialConfig(ctx)
	if err != nil {
		return err
	}
	
	newAccessToken, newExpirationDate, err := bot._RefreshAccessToken(
		botCredentialConfig.ClientId,
		botCredentialConfig.ClientSecret,
		botCredentialConfig.RefreshToken,
		ctx)
	if err != nil {
		return err
	}
	// 更新
	config := &oauth2.Config{
		ClientID:     botCredentialConfig.ClientId,
		ClientSecret: botCredentialConfig.ClientSecret,
		Endpoint:     oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/auth",
			TokenURL:  "https://accounts.google.com/o/oauth2/token",
			AuthStyle: 0,
		},
		RedirectURL:  "https://youtube.com/",
		Scopes:       nil,
	}
	botOauthToken := &oauth2.Token{
		AccessToken:  newAccessToken,
		TokenType:    "Bearer",
		RefreshToken: botCredentialConfig.RefreshToken,
		Expiry:       newExpirationDate,
	}
	botClientOption := option.WithTokenSource(config.TokenSource(ctx, botOauthToken))
	newService, err := youtube.NewService(ctx, botClientOption)
	if err != nil {
		return err
	}
	bot.ChannelYoutubeService = newService
	
	// Firestoreに保存
	err = bot.FirestoreController.SetAccessTokenOfBotCredential(newAccessToken, newExpirationDate, ctx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *YoutubeLiveChatBot) _RefreshAccessToken(clientId string, clientSecret string, refreshToken string, ctx context.Context) (string, time.Time, error) {
	youtubeLiveConfig, err := bot.FirestoreController.RetrieveYoutubeLiveConfig(ctx)
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
		youtubeLiveConfig.OAuthRefreshTokenUrl,
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





