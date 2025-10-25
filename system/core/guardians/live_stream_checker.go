package guardians

import (
	"context"
	"fmt"
	"log/slog"

	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/youtubebot"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

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

	broadcastsService := youtube.NewLiveBroadcastsService(service)
	broadcastsListResponse, err := broadcastsService.List([]string{"snippet", "contentDetails"}).BroadcastStatus("active").Do()
	if err != nil {
		return fmt.Errorf("broadcasts.List: %w", err)
	}
	usingStreamIds := make(map[string]bool)
	for _, broadcast := range broadcastsListResponse.Items {
		usingStreamIds[broadcast.ContentDetails.BoundStreamId] = true
		slog.Info("active broadcast info.",
			"id", broadcast.Id,
			"BoundStreamId", broadcast.ContentDetails.BoundStreamId,
			"title", broadcast.Snippet.Title,
		)
	}

	streamsService := youtube.NewLiveStreamsService(service)
	liveStreamListResponse, err := streamsService.List([]string{"status"}).Mine(true).Do()
	if err != nil {
		return fmt.Errorf("in streamsService.List: %w", err)
	}

	for usingStreamId := range usingStreamIds {
		for _, stream := range liveStreamListResponse.Items {
			if stream.Id == usingStreamId {
				streamStatus := stream.Status.StreamStatus
				healthStatus := stream.Status.HealthStatus.Status

				slog.Info("live stream status.",
					"liveStreamId", stream.Id,
					"streamStatus", streamStatus,
					"healthStatus", healthStatus,
				)

				if streamStatus != "active" && streamStatus != "ready" {
					_ = checker.alertOwnerBot.SendMessage(ctx, "stream status is now : "+streamStatus)
				}
				if healthStatus != "good" && healthStatus != "ok" {
					_ = checker.alertOwnerBot.SendMessage(ctx, "stream HEALTH status is now : "+healthStatus)
				}

				break
			}
		}
	}

	return nil
}
