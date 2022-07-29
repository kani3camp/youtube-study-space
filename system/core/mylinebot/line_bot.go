package mylinebot

import (
	"github.com/line/line-bot-sdk-go/linebot"
	"log"
)

type LineBot struct {
	DestinationLineId string
	Bot               *linebot.Client
}

func NewLineBot(channelSecret string, channelToken string, destinationLineId string) (*LineBot, error) {
	bot, err := linebot.New(channelSecret, channelToken)
	if err != nil {
		return nil, err
	}
	return &LineBot{
		DestinationLineId: destinationLineId,
		Bot:               bot,
	}, nil
}

// SendMessage LINEにメッセージを送信する。ログも残す。
func (bot *LineBot) SendMessage(message string) error {
	log.Println("sending a message to LINE \"", message+"\"")
	if _, err := bot.Bot.PushMessage(bot.DestinationLineId, linebot.NewTextMessage(message)).Do(); err != nil {
		log.Println("failed to send message to the LINE.")
		return err
	}
	return nil
}

// SendMessageWithError LINEにエラーを送信する。ログも残す。
func (bot *LineBot) SendMessageWithError(message string, err error) error {
	message += ":\n" + err.Error()
	return bot.SendMessage(message)
}
