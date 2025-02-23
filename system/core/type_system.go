package core

import (
	"app.modules/core/discordbot"
	"app.modules/core/repository"
	"app.modules/core/wordsreader"
	"app.modules/core/youtubebot"
)

type System struct {
	Configs             *SystemConfigs
	Repository          repository.Repository
	WordsReader         wordsreader.WordsReader
	LiveChatBot         youtubebot.LiveChatBot
	discordOwnerBot     *discordbot.DiscordBot
	discordSharedBot    *discordbot.DiscordBot
	discordSharedLogBot *discordbot.DiscordBot

	ProcessedUserId                 string
	ProcessedUserDisplayName        string
	ProcessedUserProfileImageUrl    string
	ProcessedUserIsModeratorOrOwner bool
	ProcessedUserIsMember           bool

	blockRegexesForChatMessage        []string
	blockRegexesForChannelName        []string
	notificationRegexesForChatMessage []string
	notificationRegexesForChannelName []string

	SortedMenuItems []repository.MenuDoc // メニューコードで昇順ソートして格納
}

// SystemConfigs System生成時に初期化すべきフィールド値
type SystemConfigs struct {
	Constants repository.ConstantsConfigDoc

	LiveChatBotChannelId string
}
