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

// ProcessClaimedDurableInboxMessage is the executable core of the P1 durable
// worker. It supports only behavior whose domain/reply effects can already be
// committed atomically with Inbox Processed:
//   - bot self message: Processed only
//   - ordinary/invalid text: first-use User + Processed
//   - parse/validation failures: first-use User + reply intent + Processed
//   - !info / !seat: first-use User + reply intent + Processed
//   - !more: Seat update + reply intent + Processed
//   - !rank: User setting + optional Seat appearance + reply intent + Processed
//   - !clear: WorkSegment + Seat update + reply intent + Processed
//   - !break: WorkSegment + Seat transition + activity + reply intent + Processed
//
// Unsupported executable commands return ErrDurableCommandNotSupported before
// opening a transaction. Runtime cutover must not route all commands here until
// later command slices are supported.
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
		case utils.NotCommand, utils.InvalidCommand, utils.Info, utils.Seat, utils.More, utils.Rank, utils.Clear, utils.Break:
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
			case utils.Seat:
				reply, err = app.buildDurableSeatInfoReply(ctx, &prepared.CommandDetails.SeatOption)
				if err != nil {
					return err
				}
			case utils.More:
				reply, err = app.buildDurableMoreReplyTx(ctx, tx, &prepared.CommandDetails.MoreOption)
				if err != nil {
					return err
				}
			case utils.Rank:
				reply, err = app.buildDurableRankReplyTx(ctx, tx, &userDoc, userExists)
				if err != nil {
					return err
				}
			case utils.Clear:
				reply, err = app.buildDurableClearReplyTx(ctx, tx, userExists)
				if err != nil {
					return err
				}
			case utils.Break:
				reply, err = app.buildDurableBreakReplyTx(ctx, tx, &prepared.CommandDetails.BreakOption, userExists)
				if err != nil {
					return err
				}
			default:
				return ErrDurableCommandNotSupported
			}
		}

		// All transaction reads are complete before this point. Supported write
		// commands may already have staged their domain writes in tx, but no later
		// transaction read occurs here.
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
