package core

import (
	"app.modules/core/discordbot"
	"app.modules/core/myfirestore"
	"app.modules/core/mylinebot"
	"app.modules/core/youtubebot"
)

type System struct {
	FirestoreController             *myfirestore.FirestoreController
	LiveChatBot                     *youtubebot.YoutubeLiveChatBot
	LineBot                         *mylinebot.LineBot
	DiscordBot                      *discordbot.DiscordBot
	MinWorkTimeMin                  int
	MaxWorkTimeMin                  int
	DefaultWorkTimeMin              int
	ProcessedUserId                 string
	ProcessedUserDisplayName        string
	ProcessedUserIsModeratorOrOwner bool
	DefaultSleepIntervalMilli       int
	CheckDesiredMaxSeatsIntervalSec int
}

type CommandDetails struct {
	CommandType   CommandType
	InOptions     InOptions
	InfoOption    InfoOption
	MyOptions     []MyOption
	ChangeOptions []ChangeOption
	ReportMessage string
	KickSeatId    int
	MoreMinutes   int
}

type CommandType uint

const (
	NotCommand CommandType = iota
	InvalidCommand
	In     // !in
	SeatIn // !席番号
	Out    // !out
	Info   // !info
	My     // !my
	Change // !change
	Seat   // !seat
	Report // !report
	Kick   // !kick
	More   // !more
	Rank   // !rank
)

type InfoOption struct {
	ShowDetails bool
}

type MyOptionType uint

const (
	RankVisible MyOptionType = iota
	DefaultStudyMin
)

type ChangeOptionType uint

const (
	WorkName ChangeOptionType = iota
	WorkTime
)

type InOptions struct {
	SeatId   int
	WorkName string
	WorkMin  int
}

type MyOption struct {
	Type        MyOptionType
	IntValue    int
	BoolValue   bool
	StringValue string
}

type ChangeOption struct {
	Type        ChangeOptionType
	StringValue string
	IntValue    int
}

type UserIdTotalStudySecSet struct {
	UserId        string `json:"user_id"`
	TotalStudySec int    `json:"total_study_sec"`
}
