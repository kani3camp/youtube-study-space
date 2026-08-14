package workspaceapp

import (
	"context"
	"fmt"
)

type preparedMessage struct {
	CommandDetails anyCommandDetails
	ImmediateReply string
	SkipExecution  bool
}

// anyCommandDetails keeps the preparation result typed without coupling this
// file to the command parser import; the concrete alias lives in parsing.go.
type anyCommandDetails = commandDetailsPtr

// prepareMessage performs the behavior that must happen before command
// execution, but deliberately does not post the normal YouTube reply itself.
// Legacy ProcessMessage sends ImmediateReply immediately; the durable Inbox
// worker can later persist the same reply as an outbox intent instead.
//
// Moderation side effects intentionally remain unchanged in this extraction.
// First-use user initialization is reusable inside a caller-owned tx, and
// parse/validation is now a separate side-effect-free phase.
func (app *WorkspaceApp) prepareMessage(
	ctx context.Context,
	ngWordConfig NGWordConfig,
	commandString string,
	userID string,
	userDisplayName string,
	userProfileImageURL string,
	isChatModerator bool,
	isChatOwner bool,
	isChatMember bool,
) (preparedMessage, error) {
	if userID == app.Configs.LiveChatBotChannelID {
		return preparedMessage{SkipExecution: true}, nil
	}
	if !app.Configs.Constants.YoutubeMembershipEnabled {
		isChatMember = false
	}
	app.SetProcessedUser(userID, userDisplayName, userProfileImageURL, isChatModerator, isChatOwner, isChatMember)

	// Preserve existing moderation behavior while moving normal chat replies out
	// of the preparation phase.
	if !isChatModerator && !isChatOwner {
		blocked, err := app.CheckIfUnwantedWordIncluded(ctx, ngWordConfig, userID, commandString, userDisplayName)
		if err != nil {
			app.MessageToOwnerWithError(ctx, "in CheckIfUnwantedWordIncluded", err)
			// Existing behavior continues processing when the moderation check itself
			// fails, because owner notification is the fallback.
		}
		if blocked {
			return preparedMessage{SkipExecution: true}, nil
		}
	}

	txErr := app.RunTransaction(ctx, app.ensureProcessedUserRegisteredTx)
	if txErr != nil {
		return preparedMessage{}, fmt.Errorf("in RunTransaction(): %w", txErr)
	}

	return app.parseAndValidateMessage(commandString, isChatMember), nil
}
