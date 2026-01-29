package moderatorbot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	// discordHTTPTimeout はDiscord APIへのHTTPリクエストのタイムアウト時間
	discordHTTPTimeout = 10 * time.Second
)

type DiscordBot struct {
	session       *discordgo.Session
	textChannelId string
}

func NewDiscordBot(token string, textChannelId string) (*DiscordBot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	// HTTPクライアントにタイムアウトを設定
	// NOTE: discordgoはcontextを受け取らないため、HTTPクライアントレベルでタイムアウトを設定する
	session.Client = &http.Client{
		Timeout: discordHTTPTimeout,
	}

	return &DiscordBot{
		session:       session,
		textChannelId: textChannelId,
	}, nil
}

func (bot *DiscordBot) SendMessage(ctx context.Context, message string) error {
	slog.InfoContext(ctx, "sending a message to Discord.", "message", message)
	_, err := bot.session.ChannelMessageSend(bot.textChannelId, message)
	if err != nil {
		return fmt.Errorf("in bot.session.ChannelMessageSend: %w", err)
	}
	return nil
}

func (bot *DiscordBot) SendMessageWithError(ctx context.Context, message string, err error) error {
	message += ":\n" + fmt.Sprintf("%+v", err)
	return bot.SendMessage(ctx, message)
}
