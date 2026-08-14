package workspaceapp

import (
	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/utils"
)

// parseAndValidateMessage converts chat text into executable command details or
// an immediate user-facing validation reply. It performs no repository access,
// Firestore transaction, YouTube post, or moderation side effect, so durable
// Inbox processing can safely call it inside its own transaction boundary.
func (app *WorkspaceApp) parseAndValidateMessage(
	commandString string,
	isChatMember bool,
) preparedMessage {
	commandDetails, message := utils.ParseCommand(commandString, isChatMember)
	if message != "" {
		return preparedMessage{
			ImmediateReply: i18nmsg.CommonSir(app.ProcessedUserDisplayName) + message,
			SkipExecution:  true,
		}
	}

	if message = app.ValidateCommand(*commandDetails); message != "" {
		return preparedMessage{
			ImmediateReply: i18nmsg.CommonSir(app.ProcessedUserDisplayName) + message,
			SkipExecution:  true,
		}
	}

	return preparedMessage{CommandDetails: commandDetails}
}
