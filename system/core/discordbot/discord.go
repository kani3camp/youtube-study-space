package discordbot

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log/slog"
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

	return &DiscordBot{
		session:       session,
		textChannelId: textChannelId,
	}, nil
}

func (bot *DiscordBot) SendMessage(message string) error {
	slog.Info("sending a message to Discord.", "message", message)
	_, err := bot.session.ChannelMessageSend(bot.textChannelId, message)
	if err != nil {
		return fmt.Errorf("in bot.session.ChannelMessageSend: %w", err)
	}
	return nil
}

func (bot *DiscordBot) SendMessageWithError(message string, err error) error {
	message += ":\n" + fmt.Sprintf("%+v", err)
	return bot.SendMessage(message)
}
