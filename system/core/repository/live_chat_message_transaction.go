package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

var ErrLiveChatInboxAlreadyProcessed = errors.New("live chat inbox message is already processed")

// LiveChatMessageTransactionGuard proves that the source Inbox document was
// read and its active processing lease was validated in the current Firestore
// transaction before domain writes begin.
type LiveChatMessageTransactionGuard struct {
	Inbox    LiveChatInboxDoc
	WorkerID string
}

// BeginLiveChatMessageTransaction must be called near the beginning of the
// caller's Firestore transaction, before domain writes. Firestore tracks this
// Inbox read as part of the transaction read set, so a concurrent lease reclaim
// or status change forces the whole domain transaction to retry/abort.
func (c *FirestoreControllerImplements) BeginLiveChatMessageTransaction(
	tx *firestore.Transaction,
	liveChatID string,
	messageID string,
	workerID string,
	now time.Time,
) (LiveChatMessageTransactionGuard, error) {
	if tx == nil {
		return LiveChatMessageTransactionGuard{}, errors.New("live chat message transaction requires a Firestore transaction")
	}
	if now.IsZero() {
		return LiveChatMessageTransactionGuard{}, errors.New("now is zero")
	}
	ref, err := c.liveChatInboxMessageRef(liveChatID, messageID)
	if err != nil {
		return LiveChatMessageTransactionGuard{}, err
	}
	inbox, err := readLiveChatInboxMessageTx(tx, ref)
	if err != nil {
		return LiveChatMessageTransactionGuard{}, err
	}
	if inbox.LiveChatID != liveChatID || inbox.MessageID != messageID {
		return LiveChatMessageTransactionGuard{}, fmt.Errorf(
			"%w: deterministic key points to different source message",
			ErrLiveChatInboxCorruptState,
		)
	}
	if inbox.Status == LiveChatInboxProcessed {
		return LiveChatMessageTransactionGuard{}, ErrLiveChatInboxAlreadyProcessed
	}
	if err := validateLiveChatInboxLease(inbox, workerID, now); err != nil {
		return LiveChatMessageTransactionGuard{}, err
	}
	return LiveChatMessageTransactionGuard{Inbox: inbox, WorkerID: workerID}, nil
}

// FinalizeLiveChatMessageTransaction stages reply intents and marks the source
// Inbox Processed without performing another Firestore read. The caller may
// stage domain writes before this function; all domain writes, reply intents,
// and Inbox completion then commit or roll back together.
func (c *FirestoreControllerImplements) FinalizeLiveChatMessageTransaction(
	ctx context.Context,
	tx *firestore.Transaction,
	guard LiveChatMessageTransactionGuard,
	replyIntents []LiveChatReplyOutboxDoc,
	now time.Time,
) error {
	if tx == nil {
		return errors.New("live chat message transaction requires a Firestore transaction")
	}
	if now.IsZero() {
		return errors.New("now is zero")
	}
	if err := validateLiveChatInboxLease(guard.Inbox, guard.WorkerID, now); err != nil {
		return err
	}

	seenSlots := make(map[string]struct{}, len(replyIntents))
	for i, intent := range replyIntents {
		if intent.LiveChatID != guard.Inbox.LiveChatID ||
			intent.SourceMessageID != guard.Inbox.MessageID ||
			intent.SourceAuthorChannelID != guard.Inbox.AuthorChannelID ||
			intent.SourceSequence != guard.Inbox.Sequence {
			return fmt.Errorf("reply intent at index %d does not match source Inbox message", i)
		}
		if _, exists := seenSlots[intent.IntentSlot]; exists {
			return fmt.Errorf("duplicate reply intent slot %q", intent.IntentSlot)
		}
		seenSlots[intent.IntentSlot] = struct{}{}
		if err := validateNewLiveChatReplyIntent(intent); err != nil {
			return fmt.Errorf("validate reply intent at index %d: %w", i, err)
		}
	}

	for _, intent := range replyIntents {
		if err := c.CreateLiveChatReplyIntent(ctx, tx, intent); err != nil {
			return err
		}
	}

	processed := guard.Inbox
	processedAt := now
	processed.Status = LiveChatInboxProcessed
	processed.ProcessedAt = &processedAt
	processed.LeaseOwner = ""
	processed.LeaseUntil = nil
	processed.LastError = ""
	ref, err := c.liveChatInboxMessageRef(processed.LiveChatID, processed.MessageID)
	if err != nil {
		return err
	}
	if err := c.set(ctx, tx, ref, processed); err != nil {
		return fmt.Errorf("finalize live chat Inbox message: %w", err)
	}
	return nil
}
