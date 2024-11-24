package core

import (
	"app.modules/core/discordbot"
	"app.modules/core/myfirestore"
	"app.modules/core/youtubebot"
)

type System struct {
	Configs             *SystemConfigs
	FirestoreController myfirestore.FirestoreController
	LiveChatBot         youtubebot.YoutubeLiveChatBotInterface
	discordOwnerBot     *discordbot.DiscordBot
	discordSharedBot    *discordbot.DiscordBot
	discordSharedLogBot *discordbot.DiscordBot

	ProcessedUserId                 string
	ProcessedUserDisplayName        string
	ProcessedUserProfileImageUrl    string
	ProcessedUserIsModeratorOrOwner bool
	ProcessedUserIsMember           bool

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
