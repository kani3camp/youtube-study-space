package guardians

import (
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/youtubebot"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log/slog"
)

type LiveStreamsListResponse struct {
	Kind      string `json:"access_token"`
	ExpiresIn int    `json:"expires_in"`
	Scope     string `json:"scope"`
	TokenType string `json:"token_type"`
}

type LiveStreamChecker struct {
	YoutubeLiveChatBot  youtubebot.LiveChatBot
	alertOwnerBot       moderatorbot.MessageBot
	FirestoreController repository.Repository
}

func NewLiveStreamChecker(
	controller repository.Repository,
	youtubeLiveChatBot youtubebot.LiveChatBot,
	messageBot moderatorbot.MessageBot,
) *LiveStreamChecker {

	return &LiveStreamChecker{
		YoutubeLiveChatBot:  youtubeLiveChatBot,
		alertOwnerBot:       messageBot,
		FirestoreController: controller,
	}
}

func (checker *LiveStreamChecker) Check(ctx context.Context) error {
	credentials, err := checker.FirestoreController.ReadCredentialsConfig(ctx, nil)
	if err != nil {
		return err
	}
	config := &oauth2.Config{
		ClientID:     credentials.YoutubeChannelClientId,
		ClientSecret: credentials.YoutubeChannelClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
	}
	channelOauthToken := &oauth2.Token{
		TokenType:    "Bearer",
		RefreshToken: credentials.YoutubeChannelRefreshToken,
	}
	channelClientOption := option.WithTokenSource(config.TokenSource(ctx, channelOauthToken))
	service, err := youtube.NewService(ctx, channelClientOption)
	if err != nil {
		return fmt.Errorf("in youtube.NewService: %w", err)
	}
	streamsService := youtube.NewLiveStreamsService(service)
	liveStreamListResponse, err := streamsService.List([]string{"status"}).Mine(true).Do()
	if err != nil {
		return fmt.Errorf("in streamsService.List: %w", err)
	}

	streamStatus := liveStreamListResponse.Items[0].Status.StreamStatus
	healthStatus := liveStreamListResponse.Items[0].Status.HealthStatus.Status

	slog.Info("live stream status.", "streamStatus", streamStatus, "healthStatus", healthStatus)

	if streamStatus != "active" && streamStatus != "ready" {
		_ = checker.alertOwnerBot.SendMessage("stream status is now : " + streamStatus)
	}
	if healthStatus != "good" && healthStatus != "ok" && healthStatus != "noData" {
		_ = checker.alertOwnerBot.SendMessage("stream HEALTH status is now : " + healthStatus)
	}

	return nil
}
