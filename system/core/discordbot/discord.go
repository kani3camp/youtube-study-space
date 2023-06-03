package discordbot

import (
	"github.com/bwmarrin/discordgo"
	"log"
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
	log.Println("sending a message to Discord \"", message+"\"")
	_, err := bot.session.ChannelMessageSend(bot.textChannelId, message)
	if err != nil {
		return err
	}
	return nil
}

func (bot *DiscordBot) SendMessageWithError(message string, err error) error {
	message += ":\n" + err.Error()
	return bot.SendMessage(message)
}
