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
	textChannelID string
}

func NewDiscordBot(token string, textChannelID string) (*DiscordBot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create Discord session: %w", err)
	}

	// HTTPクライアントにタイムアウトを設定
	// NOTE: discordgoはcontextを受け取らないため、HTTPクライアントレベルでタイムアウトを設定する
	session.Client = &http.Client{
		Timeout: discordHTTPTimeout,
	}

	return &DiscordBot{
		session:       session,
		textChannelID: textChannelID,
	}, nil
}

func (bot *DiscordBot) SendMessage(ctx context.Context, message string) error {
	// Message bodies may contain raw YouTube chat/user data. Log only the
	// delivery event so process logs do not become a second long-lived copy.
	slog.InfoContext(ctx, "sending a message to Discord")
	_, err := bot.session.ChannelMessageSend(bot.textChannelID, message)
	if err != nil {
		return fmt.Errorf("in bot.session.ChannelMessageSend: %w", err)
	}
	return nil
}

func (bot *DiscordBot) SendMessageWithError(ctx context.Context, message string, err error) error {
	message += ":\n" + fmt.Sprintf("%+v", err)
	return bot.SendMessage(ctx, message)
}
