package core

import (
	"app.modules/core/discordbot"
	"app.modules/core/myfirestore"
	"app.modules/core/mylinebot"
	"app.modules/core/utils"
	"app.modules/core/youtubebot"
)

type System struct {
	Configs             *SystemConfigs
	FirestoreController *myfirestore.FirestoreController
	liveChatBot         *youtubebot.YoutubeLiveChatBot
	lineBot             *mylinebot.LineBot
	discordBot          *discordbot.DiscordBot

	ProcessedUserId                 string
	ProcessedUserDisplayName        string
	ProcessedUserIsModeratorOrOwner bool

	blockRegexListForChatMessage        []string
	blockRegexListForChannelName        []string
	notificationRegexListForChatMessage []string
	notificationRegexListForChannelName []string
}

// SystemConfigs System生成時に初期化すべきフィールド値
type SystemConfigs struct {
	Constants myfirestore.ConstantsConfigDoc

	LiveChatBotChannelId string
}

type CommandDetails struct {
	CommandType  CommandType
	InOption     InOption
	InfoOption   InfoOption
	MyOptions    []MyOption
	SeatOption   SeatOption
	KickOption   KickOption
	CheckOption  CheckOption
	BlockOption  BlockOption
	ReportOption ReportOption
	ChangeOption MinutesAndWorkNameOption
	MoreOption   MoreOption
	BreakOption  MinutesAndWorkNameOption
	ResumeOption WorkNameOption
}

type CommandType uint

const (
	NotCommand CommandType = iota
	InvalidCommand
	In     // !in もしくは !席番号
	Out    // !out
	Info   // !info
	My     // !my
	Change // !change
	Seat   // !seat
	Report // !report
	Kick   // !kick
	Check  // !check
	Block  // !block
	More   // !more
	Rank   // !rank
	Break  // !break
	Resume // !resume
)

type InfoOption struct {
	ShowDetails bool
}

type MyOptionType uint

const (
	RankVisible MyOptionType = iota
	DefaultStudyMin
	FavoriteColor
)

type InOption struct {
	IsSeatIdSet        bool
	SeatId             int
	MinutesAndWorkName MinutesAndWorkNameOption
}

type MyOption struct {
	Type        MyOptionType
	IntValue    int
	BoolValue   bool
	StringValue string
}

type SeatOption struct {
	ShowDetails bool
}

type KickOption struct {
	SeatId int
}

type CheckOption struct {
	SeatId int
}

type BlockOption struct {
	SeatId int
}

type ReportOption struct {
	Message string
}

type MoreOption struct {
	DurationMin int
}

type WorkNameOption struct {
	IsWorkNameSet bool
	WorkName      string
}

type MinutesAndWorkNameOption struct {
	IsWorkNameSet    bool
	IsDurationMinSet bool
	WorkName         string
	DurationMin      int
}

func (o *MinutesAndWorkNameOption) NumOptionsSet() int {
	return utils.NumTrue(o.IsWorkNameSet, o.IsDurationMinSet)
}

type UserIdTotalStudySecSet struct {
	UserId        string `json:"user_id"`
	TotalStudySec int    `json:"total_study_sec"`
}
