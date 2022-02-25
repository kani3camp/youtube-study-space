package core

import (
	"app.modules/core/discordbot"
	"app.modules/core/myfirestore"
	"app.modules/core/mylinebot"
	"app.modules/core/youtubebot"
)

type System struct {
	Constants                       *SystemConstants
	ProcessedUserId                 string
	ProcessedUserDisplayName        string
	ProcessedUserIsModeratorOrOwner bool
}

// SystemConstants System生成時に初期化すべきフィールド値
type SystemConstants struct {
	FirestoreController             *myfirestore.FirestoreController
	liveChatBot                     *youtubebot.YoutubeLiveChatBot
	lineBot                         *mylinebot.LineBot
	discordBot                      *discordbot.DiscordBot
	LiveChatBotChannelId            string
	MinWorkTimeMin                  int
	MaxWorkTimeMin                  int
	DefaultWorkTimeMin              int
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
	CheckSeatId   int
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
	Check  // !check
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
