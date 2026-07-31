package youtubeauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"app.modules/core/mypage"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) FetchMyChannel(ctx context.Context, youtubeAccessToken string) (mypage.Viewer, error) {
	youtubeAccessToken = strings.TrimSpace(youtubeAccessToken)
	if youtubeAccessToken == "" {
		return mypage.Viewer{}, mypage.ErrInvalidYouTubeAccessToken
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: youtubeAccessToken,
	})
	httpClient := oauth2.NewClient(ctx, tokenSource)

	service, err := youtube.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return mypage.Viewer{}, fmt.Errorf("initialize youtube service: %w", err)
	}

	response, err := service.Channels.
		List([]string{"snippet"}).
		Mine(true).
		MaxResults(1).
		Context(ctx).
		Do()
	if err != nil {
		return mypage.Viewer{}, classifyChannelsListError(err)
	}

	if len(response.Items) == 0 {
		return mypage.Viewer{}, fmt.Errorf("%w: no youtube channel found", mypage.ErrYouTubeChannelNotFound)
	}

	channel := response.Items[0]
	if channel == nil || channel.Id == "" {
		return mypage.Viewer{}, fmt.Errorf("%w: channel id is empty", mypage.ErrInvalidYouTubeAccessToken)
	}

	return mypage.Viewer{
		YouTubeChannelID: channel.Id,
		DisplayName:      channelTitle(channel),
		ProfileImageURL:  thumbnailURL(channel),
	}, nil
}

func classifyChannelsListError(err error) error {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("%w: channels.list mine=true: %w", mypage.ErrYouTubeUpstream, err)
	}

	if apiErr.Code == 429 || hasReasonOrText(apiErr, rateLimitReasons...) {
		return fmt.Errorf("%w: channels.list mine=true: %w", mypage.ErrYouTubeRateLimited, err)
	}

	if apiErr.Code == 401 || hasReasonOrText(apiErr, authenticationReasons...) {
		return fmt.Errorf("%w: channels.list mine=true: %w", mypage.ErrInvalidYouTubeAccessToken, err)
	}

	if apiErr.Code == 403 {
		return fmt.Errorf("%w: channels.list mine=true: %w", mypage.ErrYouTubeForbidden, err)
	}

	return fmt.Errorf("%w: channels.list mine=true: %w", mypage.ErrYouTubeUpstream, err)
}

var rateLimitReasons = []string{
	"dailyLimitExceeded",
	"dailyLimitExceededUnreg",
	"quotaExceeded",
	"rateLimitExceeded",
	"userRateLimitExceeded",
}

var authenticationReasons = []string{
	"access_token_scope_insufficient",
	"authError",
	"insufficient authentication scope",
	"insufficientAuthenticationScopes",
	"insufficientPermissions",
}

func hasReasonOrText(apiErr *googleapi.Error, reasons ...string) bool {
	for _, item := range apiErr.Errors {
		for _, reason := range reasons {
			if strings.EqualFold(item.Reason, reason) {
				return true
			}
		}
	}

	responseText := strings.ToLower(apiErr.Message + "\n" + apiErr.Body)
	for _, reason := range reasons {
		if strings.Contains(responseText, strings.ToLower(reason)) {
			return true
		}
	}
	return false
}

func channelTitle(channel *youtube.Channel) string {
	if channel == nil || channel.Snippet == nil {
		return ""
	}
	return channel.Snippet.Title
}

func thumbnailURL(channel *youtube.Channel) string {
	if channel == nil || channel.Snippet == nil || channel.Snippet.Thumbnails == nil {
		return ""
	}

	thumbnails := channel.Snippet.Thumbnails

	if thumbnails.High != nil && thumbnails.High.Url != "" {
		return thumbnails.High.Url
	}
	if thumbnails.Medium != nil && thumbnails.Medium.Url != "" {
		return thumbnails.Medium.Url
	}
	if thumbnails.Default != nil && thumbnails.Default.Url != "" {
		return thumbnails.Default.Url
	}

	return ""
}
