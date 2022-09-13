package guardians

import (
	"app.modules/core/myfirestore"
	"app.modules/core/mylinebot"
	"app.modules/core/youtubebot"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type LiveStreamsListResponse struct {
	Kind      string `json:"access_token"`
	ExpiresIn int    `json:"expires_in"`
	Scope     string `json:"scope"`
	TokenType string `json:"token_type"`
}

type LiveStreamChecker struct {
	YoutubeLiveChatBot  *youtubebot.YoutubeLiveChatBot
	LineBot             *mylinebot.LineBot
	FirestoreController *myfirestore.FirestoreController
}

func NewLiveStreamChecker(
	controller *myfirestore.FirestoreController,
	youtubeLiveChatBot *youtubebot.YoutubeLiveChatBot,
	lineBot *mylinebot.LineBot,
) *LiveStreamChecker {
	
	return &LiveStreamChecker{
		YoutubeLiveChatBot:  youtubeLiveChatBot,
		LineBot:             lineBot,
		FirestoreController: controller,
	}
}

func (checker *LiveStreamChecker) Check(ctx context.Context) error {
	credentials, err := checker.FirestoreController.RetrieveCredentialsConfig(ctx)
	if err != nil {
		return err
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
	service, err := youtube.NewService(ctx, channelClientOption)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	streamsService := youtube.NewLiveStreamsService(service)
	liveStreamListResponse, err := streamsService.List([]string{"status"}).Mine(true).Do()
	//fmt.Printf("%# v\n", pretty.Formatter(liveStreamListResponse))
	
	streamStatus := liveStreamListResponse.Items[0].Status.StreamStatus
	healthStatus := liveStreamListResponse.Items[0].Status.HealthStatus.Status
	
	fmt.Println(streamStatus)
	fmt.Println(healthStatus)
	
	if streamStatus != "active" && streamStatus != "ready" {
		_ = checker.LineBot.SendMessage("stream status is now : " + streamStatus)
	}
	if healthStatus != "good" && healthStatus != "ok" && healthStatus != "noData" {
		_ = checker.LineBot.SendMessage("stream HEALTH status is now : " + healthStatus)
	}
	
	return nil
}
