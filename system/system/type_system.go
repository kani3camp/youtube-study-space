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
	DefaultWorkTimeMin int	// TODO: firestoreに追加
	ProcessedUserId string
	ProcessedUserDisplayName string
	DefaultSleepIntervalMilli int
}

type CommandDetails struct {
	commandType CommandType
	options CommandOptions
	// 以下2つは不要ならばいつか消す
	//commanderChannelId string
	//commanderDisplayName string
}

type CommandType uint
const (
	NotCommand CommandType = iota
	In		// !in
	SeatIn	// !席番号
	Out		// !out
	Info	// !info
)

type CommandOptions struct {
	seatId int
	workName string
	workMin int
}

