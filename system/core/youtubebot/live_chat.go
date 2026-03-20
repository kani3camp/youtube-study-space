package youtubebot

import (
	"context"
	"log/slog"
	"strconv"
	"unicode/utf8"

	"app.modules/core/repository"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const MaxLiveChatMessageLength = 200

type AccessTokenResponseStruct struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

func NewYoutubeLiveChatBot(liveChatID string, controller repository.Repository, ctx context.Context) (LiveChatBot, error) {
	var channelYoutubeService *youtube.Service
	var botYoutubeService *youtube.Service

	txErr := controller.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		credentials, err := controller.ReadCredentialsConfig(ctx, tx)
		if err != nil {
			return err
		}

		// channel
		channelConfig := &oauth2.Config{
			ClientID:     credentials.YoutubeChannelClientID,
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
			ClientID:     credentials.YoutubeBotClientID,
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
		LiveChatID:            liveChatID,
		ChannelYoutubeService: channelYoutubeService,
		BotYoutubeService:     botYoutubeService,
		FirestoreController:   controller,
	}, nil
}

func (b *YoutubeLiveChatBot) ListMessages(ctx context.Context, nextPageToken string) ([]*youtube.LiveChatMessage, string, int, error) {
	// 1回目の試行
	response, err := b.tryListMessages(nextPageToken, b.LiveChatID)
	if err == nil {
		return response.Items, response.NextPageToken, int(response.PollingIntervalMillis), nil
	}

	slog.Error("first call failed in tryListMessages()", "err", err)

	// エラーコードを確認
	var errGoogle *googleapi.Error
	ok := errors.As(err, &errGoogle)
	if !ok {
		return nil, "", 0, errors.New("failed to cast error to googleapi.Error")
	}

	switch errGoogle.Code {
	case 400, 403, 404:
		// live chat idが変わっている可能性があるため、更新して再試行
		if err := b.refreshLiveChatID(ctx); err != nil {
			return nil, "", 0, err
		}
	case 500:
		return nil, "", 0, nil
	default:
		slog.Warn("Unknown status code.", "code", errGoogle.Code)
		return nil, "", 0, err
	}

	// 2回目の試行（更新されたLiveChatIDで）
	slog.Info("trying second call in ListMessages()...")
	response, err = b.tryListMessages(nextPageToken, b.LiveChatID)
	if err != nil {
		slog.Error("second call failed in tryListMessages()")
		return nil, "", 0, err
	}

	return response.Items, response.NextPageToken, int(response.PollingIntervalMillis), nil
}

// tryListMessages 指定されたLiveChatIDでメッセージリストを取得する
func (b *YoutubeLiveChatBot) tryListMessages(nextPageToken string, liveChatID string) (*youtube.LiveChatMessageListResponse, error) {
	liveChatMessageService := youtube.NewLiveChatMessagesService(b.BotYoutubeService)
	part := []string{
		"snippet",
		"authorDetails",
	}

	listCall := liveChatMessageService.List(liveChatID, part)
	if nextPageToken != "" {
		listCall = listCall.PageToken(nextPageToken)
	}

	return listCall.Do()
}

func (b *YoutubeLiveChatBot) PostMessage(ctx context.Context, message string) error {
	slog.Info("sending a message to Youtube Live.", "message", message)

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
		if err := b.postMessage(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (b *YoutubeLiveChatBot) postMessage(ctx context.Context, message string) error {
	if len(message) == 0 {
		return errors.New("message length is 0.")
	}

	// メッセージ送信を試行
	err := b.tryPostMessage(message, b.LiveChatID)
	if err == nil {
		return nil
	}

	// 2回目の試行
	slog.Error("first post failed", "err", err)
	err = b.tryPostMessage(message, b.LiveChatID)
	if err == nil {
		slog.Info("second post succeeded!")
		return nil
	}

	slog.Error("second post failed", "err", err)

	// live chat idが変わっている可能性があるため、更新して再試行
	if err := b.refreshLiveChatID(ctx); err != nil {
		return err
	}

	// 3回目の試行（更新されたLiveChatIDで）
	err = b.tryPostMessage(message, b.LiveChatID)
	if err != nil {
		slog.Error("third post failed", "err", err)
		return err
	}

	slog.Info("third post succeeded!")
	return nil
}

// tryPostMessage 指定されたLiveChatIDでメッセージを送信する
func (b *YoutubeLiveChatBot) tryPostMessage(message string, liveChatID string) error {
	part := []string{"snippet"}
	liveChatMessage := youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			DisplayMessage: message,
			LiveChatId:     liveChatID,
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: message,
			},
			Type: "textMessageEvent",
		},
	}
	liveChatMessageService := youtube.NewLiveChatMessagesService(b.BotYoutubeService)
	insertCall := liveChatMessageService.Insert(part, &liveChatMessage)
	_, err := insertCall.Do()
	return err
}

// refreshLiveChatID live chat idを取得するとともに、firestoreに保存（更新）する
func (b *YoutubeLiveChatBot) refreshLiveChatID(ctx context.Context) error {
	slog.Info(utils.NameOf(b.refreshLiveChatID))

	// 1回目の試行
	response, err := b.fetchActiveBroadcasts()
	if err != nil {
		slog.Error("first attempt to fetch broadcasts failed", "err", err)
		return err
	}

	if len(response.Items) == 1 {
		return b.updateLiveChatID(ctx, response.Items[0].Snippet.LiveChatId)
	} else if len(response.Items) == 0 {
		slog.Warn("ライブ1個もやってない（1回目）")

		// たまに、配信してるのにこの結果になることがあるかも（未確認）しれないので、もう一度。
		response, err := b.fetchActiveBroadcasts()
		if err != nil {
			slog.Error("second attempt to fetch broadcasts failed", "err", err)
			return err
		}

		if len(response.Items) == 1 {
			return b.updateLiveChatID(ctx, response.Items[0].Snippet.LiveChatId)
		} else if len(response.Items) == 0 {
			return errors.New("2回試したけどライブ1個もやってない")
		} else {
			return errors.New("more than 2 live broadcasts!: " + strconv.Itoa(len(response.Items)))
		}
	} else {
		return errors.New("more than 2 live broadcasts!: " + strconv.Itoa(len(response.Items)))
	}
}

// fetchActiveBroadcasts アクティブな配信を取得する
func (b *YoutubeLiveChatBot) fetchActiveBroadcasts() (*youtube.LiveBroadcastListResponse, error) {
	broadCastsService := youtube.NewLiveBroadcastsService(b.ChannelYoutubeService)
	part := []string{"snippet"}
	listCall := broadCastsService.List(part).BroadcastStatus("active")
	response, err := listCall.Do()
	if err != nil {
		slog.Info("trying second call...")
		// 失敗した場合は再試行
		broadCastsService = youtube.NewLiveBroadcastsService(b.ChannelYoutubeService)
		listCall = broadCastsService.List(part).BroadcastStatus("active")
		response, err = listCall.Do()
	}
	return response, err
}

// updateLiveChatID LiveChatIdを更新する
func (b *YoutubeLiveChatBot) updateLiveChatID(ctx context.Context, newLiveChatID string) error {
	slog.Info("new live chat id: " + newLiveChatID)
	if err := b.FirestoreController.UpdateLiveChatID(ctx, nil, newLiveChatID); err != nil {
		return err
	}
	b.LiveChatID = newLiveChatID
	return nil
}

// BanUser 指定したユーザー（Youtubeチャンネル）をブロックする。
func (b *YoutubeLiveChatBot) BanUser(ctx context.Context, userID string) error {
	// 1回目の試行
	err := b.tryBanUser(userID, b.LiveChatID)
	if err == nil {
		return nil
	}

	slog.Error("first ban request failed", "err", err)

	// live chat idが変わっている可能性があるため、更新して再試行
	if err := b.refreshLiveChatID(ctx); err != nil {
		return err
	}

	// 2回目の試行（更新されたLiveChatIDで）
	if err := b.tryBanUser(userID, b.LiveChatID); err != nil {
		slog.Error("second ban request failed", "err", err)
		return err
	}

	return nil
}

// tryBanUser 指定されたLiveChatIDでユーザーをブロックする
func (b *YoutubeLiveChatBot) tryBanUser(userID string, liveChatID string) error {
	part := []string{"snippet"}
	liveChatBan := youtube.LiveChatBan{
		Snippet: &youtube.LiveChatBanSnippet{
			LiveChatId: liveChatID,
			Type:       "permanent",
			BannedUserDetails: &youtube.ChannelProfileDetails{
				ChannelId: userID,
			},
		},
	}
	liveChatBanService := youtube.NewLiveChatBansService(b.BotYoutubeService)
	insertCall := liveChatBanService.Insert(part, &liveChatBan)

	_, err := insertCall.Do()
	return err
}
