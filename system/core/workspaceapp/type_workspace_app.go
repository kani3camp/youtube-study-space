package workspaceapp

import (
	"app.modules/core/moderatorbot"
	"app.modules/core/repository"
	"app.modules/core/wordsreader"
	"app.modules/core/youtubebot"
)

type WorkspaceApp struct {
	Configs            *SystemConfigs
	Repository         repository.Repository
	WordsReader        wordsreader.WordsReader
	LiveChatBot        youtubebot.LiveChatBot
	alertOwnerBot      moderatorbot.MessageBot
	alertModeratorsBot moderatorbot.MessageBot
	logModeratorsBot   moderatorbot.MessageBot

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
