package workspaceapp

import (
	"errors"

	"app.modules/core/repository"
	"app.modules/core/utils"
)

// CanProcessDurableInboxMessage performs a persistence- and actor-state-free
// capability preflight for the current migration slice. It must run before
// acquiring a processing lease so unsupported commands do not consume
// AttemptCount while waiting for a later deployment to add coverage.
func (app *WorkspaceApp) CanProcessDurableInboxMessage(inbox repository.LiveChatInboxDoc) (bool, error) {
	if app.Configs == nil {
		return false, errors.New("workspace configs are nil")
	}
	if inbox.AuthorChannelID == app.Configs.LiveChatBotChannelID {
		return true, nil
	}

	isChatMember := inbox.AuthorIsChatMember
	if !app.Configs.Constants.YoutubeMembershipEnabled {
		isChatMember = false
	}

	commandDetails, parseReply := utils.ParseCommand(inbox.MessageText, isChatMember)
	if parseReply != "" {
		return true, nil
	}
	if commandDetails == nil {
		return false, nil
	}
	if validationReply := app.ValidateCommand(*commandDetails); validationReply != "" {
		return true, nil
	}

	switch commandDetails.CommandType {
	case utils.NotCommand, utils.InvalidCommand, utils.Info, utils.Seat, utils.More, utils.Rank, utils.Clear, utils.Break:
		return true, nil
	default:
		return false, nil
	}
}
