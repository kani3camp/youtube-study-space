package workspaceapp

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/utils"
)

type preparedMessage struct {
	CommandDetails *utils.CommandDetails
	ImmediateReply string
	SkipExecution  bool
}

// prepareMessage performs the behavior that must happen before command
// execution, but deliberately does not post the normal YouTube reply itself.
// Legacy ProcessMessage sends ImmediateReply immediately; the durable Inbox
// worker can later persist the same reply as an outbox intent instead.
//
// Moderation side effects and first-use user initialization intentionally remain
// unchanged in this first extraction. They will be separated in later phases.
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

	// Preserve the existing first-use initialization transaction. This will be
	// folded into the durable message transaction in a later migration step.
	txErr := app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		isRegistered, err := app.IfUserRegistered(ctx, tx)
		if err != nil {
			return fmt.Errorf("in IfUserRegistered(): %w", err)
		}
		if !isRegistered {
			if err := app.CreateUser(ctx, tx); err != nil {
				return fmt.Errorf("in CreateUser(): %w", err)
			}
		return nil
	})
	if txErr != nil {
		return preparedMessage{}, fmt.Errorf("in RunTransaction(): %w", txErr)
	}

	commandDetails, message := utils.ParseCommand(commandString, isChatMember)
	if message != "" {
		return preparedMessage{
			ImmediateReply: i18nmsg.CommonSir(app.ProcessedUserDisplayName) + message,
			SkipExecution:  true,
		}, nil
	}

	if message = app.ValidateCommand(*commandDetails); message != "" {
		return preparedMessage{
			ImmediateReply: i18nmsg.CommonSir(app.ProcessedUserDisplayName) + message,
			SkipExecution:  true,
		}, nil
	}

	return preparedMessage{CommandDetails: commandDetails}, nil
}
