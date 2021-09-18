package core

import (
	"app.modules/core/myfirestore"
	"app.modules/core/mylinebot"
	"app.modules/core/youtubebot"
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
	CommandType CommandType
	Options     CommandOptions
	// 以下2つは不要ならばいつか消す
	//commanderChannelId string
	//commanderDisplayName string
}

type CommandType uint
const (
	NotCommand CommandType = iota
	InvalidCommand
	In		// !in
	SeatIn	// !席番号
	Out		// !out
	Info	// !info
)

type CommandOptions struct {
	SeatId   int
	WorkName string
	WorkMin  int
}


type UserIdTotalStudySecSet struct {
	UserId string	`json:"user_id"`
	TotalStudySec int	`json:"total_study_sec"`
}