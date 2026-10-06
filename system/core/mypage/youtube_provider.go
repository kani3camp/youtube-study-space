package mypage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	youtubeReadOnly    = "https://www.googleapis.com/auth/youtube.readonly"
	youtubeChannelsURL = "https://www.googleapis.com/youtube/v3/channels"
)

var youtubeChannelID = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)

// GoogleYouTubeOAuth uses a short-lived token only within Resolve. Its client
// has no token source, automatic refresh, retry, credential logging or storage.
// Supply server-only secrets after the infrastructure/credential Ready Gate.
type GoogleYouTubeOAuth struct {
	config oauth2.Config
	client *http.Client
}

func NewGoogleYouTubeOAuth(clientID, clientSecret, publicOrigin string, client *http.Client) (*GoogleYouTubeOAuth, error) {
	origin, err := url.Parse(publicOrigin)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.Opaque != "" || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" || clientID == "" || clientSecret == "" {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	bounded := http.Client{Timeout: 8 * time.Second}
	if client != nil {
		bounded = *client
	}
	if bounded.Timeout <= 0 || bounded.Timeout > 8*time.Second {
		bounded.Timeout = 8 * time.Second
	}
	// Never forward the code, client secret or bearer token to a redirect target.
	bounded.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &GoogleYouTubeOAuth{client: &bounded, config: oauth2.Config{
		ClientID: clientID, ClientSecret: clientSecret,
		RedirectURL: publicOrigin + "/api/auth/youtube/callback",
		Scopes:      []string{youtubeReadOnly},
		Endpoint:    oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", AuthStyle: oauth2.AuthStyleInParams},
	}}, nil
}

func (p *GoogleYouTubeOAuth) AuthorizationURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("prompt", "select_account"))
}

func (p *GoogleYouTubeOAuth) Resolve(ctx context.Context, code string) ([]Channel, error) {
	if code == "" {
		return nil, apiError("OAUTH_FAILED")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		// oauth2.RetrieveError and URL errors can contain credentials/provider text.
		return nil, apiError("OAUTH_FAILED")
	}
	scope, ok := token.Extra("scope").(string)
	if !ok || !slices.Contains(strings.Fields(scope), youtubeReadOnly) {
		return nil, apiError("OAUTH_SCOPE_INSUFFICIENT")
	}
	if token.AccessToken == "" || !strings.EqualFold(token.TokenType, "Bearer") || token.Expiry.IsZero() || !token.Valid() {
		return nil, apiError("OAUTH_FAILED")
	}
	// Discard even an unsolicited refresh token; never build an OAuth client or
	// retain the exchange result in the provider/store/session.
	token.RefreshToken = ""
	query := url.Values{"mine": {"true"}, "part": {"snippet"}, "maxResults": {"2"}, "fields": {"nextPageToken,items(id,snippet(title,customUrl,thumbnails(default(url))))"}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeChannelsURL+"?"+query, nil)
	if err != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	// Clear our local credential reference before processing the public metadata.
	token.AccessToken = ""
	if err != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	defer response.Body.Close() //nolint:errcheck // Closing a read-only response cannot change the proof; never log credential-bearing transport errors.
	if response.StatusCode != http.StatusOK {
		return nil, apiError("CHANNEL_UNAVAILABLE")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	var result struct {
		NextPageToken string `json:"nextPageToken"`
		Items         []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title      string `json:"title"`
				CustomURL  string `json:"customUrl"`
				Thumbnails struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if json.Unmarshal(data, &result) != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	if len(result.Items) > 1 || result.NextPageToken != "" {
		return nil, apiError("CHANNEL_AMBIGUOUS")
	}
	if len(result.Items) != 1 {
		return nil, apiError("CHANNEL_UNAVAILABLE")
	}
	item := result.Items[0]
	if !youtubeChannelID.MatchString(item.ID) || strings.TrimSpace(item.Snippet.Title) == "" {
		return nil, apiError("CHANNEL_UNAVAILABLE")
	}
	channel := Channel{ID: item.ID, DisplayName: item.Snippet.Title}
	// customUrl may be a legacy custom URL instead of a current @handle.
	if strings.HasPrefix(item.Snippet.CustomURL, "@") {
		channel.Handle = &item.Snippet.CustomURL
	}
	if avatar, err := url.Parse(item.Snippet.Thumbnails.Default.URL); err == nil && avatar.Scheme == "https" && avatar.Host != "" && avatar.User == nil && avatar.Opaque == "" {
		channel.AvatarURL = &item.Snippet.Thumbnails.Default.URL
	}
	return []Channel{channel}, nil
}
