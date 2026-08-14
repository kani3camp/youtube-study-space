package workspaceapp

import (
	"errors"

	"app.modules/core/repository"
	"app.modules/core/utils"
)

// CanProcessDurableInboxMessage performs a side-effect-free capability preflight
// for the current migration slice. It must run before acquiring a processing
// lease so unsupported commands do not consume AttemptCount while waiting for a
// later deployment to add coverage.
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
	app.SetProcessedUser(
		inbox.AuthorChannelID,
		inbox.AuthorDisplayName,
		inbox.AuthorProfileImageURL,
		inbox.AuthorIsChatModerator,
		inbox.AuthorIsChatOwner,
		isChatMember,
	)
	prepared := app.parseAndValidateMessage(inbox.MessageText, isChatMember)
	if prepared.ImmediateReply != "" {
		return true, nil
	}
	if prepared.SkipExecution || prepared.CommandDetails == nil {
		return false, nil
	}
	switch prepared.CommandDetails.CommandType {
	case utils.NotCommand, utils.Info:
		return true, nil
	default:
		return false, nil
	}
}
