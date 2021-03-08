package mylinebot

import (
	"fmt"
	"github.com/line/line-bot-sdk-go/linebot"
)


type LineBot struct {
	DestinationLineId	string
	Bot 	*linebot.Client
}


func NewLineBot(channelSecret string, channelToken string, destinationLineId string) (*LineBot, error) {
	bot, err := linebot.New(channelSecret, channelToken)
	if err != nil {
		return nil, err
	}
	return &LineBot{
		DestinationLineId: destinationLineId,
		Bot: bot,
	}, nil
}


func (bot *LineBot) SendMessage(message string) error {
	fmt.Println("sending a message to LINE \"", message + "\"")
	if _, err := bot.Bot.PushMessage(bot.DestinationLineId, linebot.NewTextMessage(message)).Do(); err != nil {
		fmt.Println("failed to send message to the LINE.")
		return err
	}
	return nil
}

func (bot *LineBot) SendMessageWithError(message string, err error) error {
	fmt.Println("sending an error to LINE: \n" + err.Error())
	message += ":\n" + err.Error()
	err = bot.SendMessage(message)
	if err != nil {
		return err
	}
	return nil
}
