package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/repository"
	"app.modules/core/utils"
)

const durablePrimaryReplyIntentSlot = "primary"

var ErrDurableCommandNotSupported = errors.New("command is not supported by durable live chat processing yet")

type liveChatMessageTransactionRepository interface {
	BeginLiveChatMessageTransaction(
		tx *firestore.Transaction,
		liveChatID string,
		messageID string,
		workerID string,
		now time.Time,
	) (repository.LiveChatMessageTransactionGuard, error)
	FinalizeLiveChatMessageTransaction(
		ctx context.Context,
		tx *firestore.Transaction,
		guard repository.LiveChatMessageTransactionGuard,
		replyIntents []repository.LiveChatReplyOutboxDoc,
		now time.Time,
	) error
}

// ProcessClaimedDurableInboxMessage is the first executable slice of the P1
// durable worker. It intentionally supports only behavior whose domain and
// reply effects can already be committed atomically:
//   - bot's own source message: mark Processed, no user/reply effect
//   - ordinary/invalid command text: first-use User creation + Processed
//   - parse/validation failures: first-use User creation + reply intent + Processed
//   - !info: first-use User creation + reply intent + Processed
//
// Unsupported executable commands return ErrDurableCommandNotSupported before
// opening a transaction, leaving the caller-owned Inbox lease unchanged.
// Runtime cutover must not route all commands here until later command slices
// are supported.
func (app *WorkspaceApp) ProcessClaimedDurableInboxMessage(
	ctx context.Context,
	inbox repository.LiveChatInboxDoc,
	workerID string,
) error {
	durableRepo, ok := app.Repository.(liveChatMessageTransactionRepository)
	if !ok {
		return errors.New("workspace repository does not support durable live chat message transactions")
	}
	if app.Configs == nil {
		return errors.New("workspace configs are nil")
	}

	if inbox.AuthorChannelID == app.Configs.LiveChatBotChannelID {
		return app.finalizeDurableNoopMessage(ctx, durableRepo, inbox, workerID)
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
	if prepared.ImmediateReply == "" {
		if prepared.SkipExecution || prepared.CommandDetails == nil {
			return ErrDurableCommandNotSupported
		}
		switch prepared.CommandDetails.CommandType {
		case utils.NotCommand, utils.InvalidCommand, utils.Info:
			// Supported by this migration slice.
		default:
			return ErrDurableCommandNotSupported
		}
	}

	now := app.currentTime()
	return app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		guard, err := durableRepo.BeginLiveChatMessageTransaction(
			tx,
			inbox.LiveChatID,
			inbox.MessageID,
			workerID,
			now,
		)
		if err != nil {
			return fmt.Errorf("begin durable live chat message transaction: %w", err)
		}

		userDoc, userExists, err := app.readDurableMessageUser(ctx, tx, now)
		if err != nil {
			return err
		}

		reply := prepared.ImmediateReply
		if reply == "" {
			switch prepared.CommandDetails.CommandType {
			case utils.NotCommand, utils.InvalidCommand:
				// Legacy ProcessMessage registers first-use users for ordinary and
				// invalid-command text, then performs no command side effect/reply.
			case utils.Info:
				var totalStudyDuration time.Duration
				var dailyTotalStudyDuration time.Duration
				if userExists {
					totalStudyDuration, dailyTotalStudyDuration, err = app.GetUserRealtimeTotalStudyDurations(
						ctx,
						tx,
						app.ProcessedUserID,
					)
					if err != nil {
						return fmt.Errorf("read realtime study durations for durable !info: %w", err)
					}
				} else {
					inMemberRoom, inGeneralRoom, roomErr := app.IsUserInRoom(ctx, app.ProcessedUserID)
					if roomErr != nil {
						return fmt.Errorf("check room state for unregistered durable user: %w", roomErr)
					}
					if inMemberRoom || inGeneralRoom {
						return errors.New("unregistered user unexpectedly has an active seat")
					}
				}
				reply = app.buildUserInfoReply(
					userDoc,
					totalStudyDuration,
					dailyTotalStudyDuration,
					&prepared.CommandDetails.InfoOption,
				)
			default:
				return ErrDurableCommandNotSupported
			}
		}

		// All reads are complete before this point. Firestore transaction writes
		// start here and Finalize performs no additional reads.
		if !userExists {
			if err := app.Repository.CreateUser(ctx, tx, app.ProcessedUserID, userDoc); err != nil {
				return fmt.Errorf("create first-use durable user: %w", err)
			}
		}

		intents := durableReplyIntents(inbox, reply, now)
		if err := durableRepo.FinalizeLiveChatMessageTransaction(ctx, tx, guard, intents, now); err != nil {
			return fmt.Errorf("finalize durable live chat message: %w", err)
		}
		return nil
	})
}

func (app *WorkspaceApp) readDurableMessageUser(
	ctx context.Context,
	tx *firestore.Transaction,
	now time.Time,
) (repository.UserDoc, bool, error) {
	userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserID)
	if status.Code(err) == codes.NotFound {
		return repository.UserDoc{
			RegistrationDate:   now,
			DailyTotalStudySec: 0,
			TotalStudySec:      0,
		}, false, nil
	}
	if err != nil {
		return repository.UserDoc{}, false, fmt.Errorf("read durable message user: %w", err)
	}
	return userDoc, true, nil
}

func (app *WorkspaceApp) finalizeDurableNoopMessage(
	ctx context.Context,
	durableRepo liveChatMessageTransactionRepository,
	inbox repository.LiveChatInboxDoc,
	workerID string,
) error {
	now := app.currentTime()
	return app.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		guard, err := durableRepo.BeginLiveChatMessageTransaction(
			tx,
			inbox.LiveChatID,
			inbox.MessageID,
			workerID,
			now,
		)
		if err != nil {
			return fmt.Errorf("begin durable noop live chat message transaction: %w", err)
		}
		return durableRepo.FinalizeLiveChatMessageTransaction(ctx, tx, guard, nil, now)
	})
}

func durableReplyIntents(
	inbox repository.LiveChatInboxDoc,
	reply string,
	now time.Time,
) []repository.LiveChatReplyOutboxDoc {
	if reply == "" {
		return nil
	}
	return []repository.LiveChatReplyOutboxDoc{{
		LiveChatID:            inbox.LiveChatID,
		SourceMessageID:       inbox.MessageID,
		SourceAuthorChannelID: inbox.AuthorChannelID,
		IntentSlot:            durablePrimaryReplyIntentSlot,
		SourceSequence:        inbox.Sequence,
		Message:               reply,
		Status:                repository.LiveChatReplyOutboxPending,
		CreatedAt:             now,
		AvailableAt:           now,
	}}
}
