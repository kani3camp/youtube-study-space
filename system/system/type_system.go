package system

import (
	"app.modules/system/myfirestore"
	"app.modules/system/mylinebot"
	"app.modules/system/youtubebot"
)

type System struct {
	FirestoreController *myfirestore.FirestoreController
	LiveChatBot *youtubebot.YoutubeLiveChatBot
	LineBot *mylinebot.LineBot
	MinWorkTimeMin int
	MaxWorkTimeMin int
	ProcessedUserId string
	ProcessedUserDisplayName string
	DefaultSleepIntervalMilli int
}

type CommandDetails struct {
	commandType CommandType
	options CommandOptions
	commanderChannelId string
	commanderDisplayName string
}

type CommandType uint
const (
	NotCommand CommandType = iota
	In
	Out
	Info
)

type CommandOptions struct {
	roomType RoomType
	seatId int
	workName string
	workMin int
}

type RoomType uint
const (
	DefaultRoom RoomType = iota
	NoSeatRoom
)