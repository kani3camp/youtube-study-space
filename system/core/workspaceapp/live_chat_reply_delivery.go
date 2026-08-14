package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app.modules/core/repository"
)

// LiveChatReplyDeliveryRepository is the narrow persistence port required by
// the external YouTube reply worker. The Firestore implementation already
// provides lease/retry/dead-letter semantics for these operations.
type LiveChatReplyDeliveryRepository interface {
	ListClaimableLiveChatReplies(
		ctx context.Context,
		liveChatID string,
		now time.Time,
		limit int,
	) ([]repository.LiveChatReplyOutboxDoc, error)
	ClaimLiveChatReply(
		ctx context.Context,
		liveChatID, sourceMessageID, intentSlot, workerID string,
		now time.Time,
		leaseDuration time.Duration,
		maxAttempts int,
	) (repository.LiveChatReplyOutboxDoc, error)
	CompleteLiveChatReply(
		ctx context.Context,
		liveChatID, sourceMessageID, intentSlot, workerID string,
		now time.Time,
	) error
	FailLiveChatReply(
		ctx context.Context,
		liveChatID, sourceMessageID, intentSlot, workerID string,
		now time.Time,
		maxAttempts int,
		retryDelay time.Duration,
		deliveryErr error,
	) (repository.LiveChatReplyOutboxStatus, error)
}

type LiveChatReplySender interface {
	PostMessage(ctx context.Context, message string) error
}

type LiveChatReplyDeliveryOptions struct {
	LeaseDuration time.Duration
	RetryDelay    time.Duration
	MaxAttempts   int
}

func (o LiveChatReplyDeliveryOptions) validate() error {
	if o.LeaseDuration <= 0 {
		return fmt.Errorf("reply lease duration must be positive: %s", o.LeaseDuration)
	}
	if o.RetryDelay < 0 {
		return fmt.Errorf("reply retry delay must not be negative: %s", o.RetryDelay)
	}
	if o.MaxAttempts <= 0 {
		return fmt.Errorf("reply max attempts must be positive: %d", o.MaxAttempts)
	}
	return nil
}

type LiveChatReplyDeliveryResult struct {
	DidWork         bool
	SourceMessageID string
	IntentSlot      string
	Status          repository.LiveChatReplyOutboxStatus
}

type LiveChatReplyDeliveryWorker struct {
	repository LiveChatReplyDeliveryRepository
	sender     LiveChatReplySender
	now        func() time.Time
}

func NewLiveChatReplyDeliveryWorker(
	repo LiveChatReplyDeliveryRepository,
	sender LiveChatReplySender,
) *LiveChatReplyDeliveryWorker {
	return &LiveChatReplyDeliveryWorker{
		repository: repo,
		sender:     sender,
		now:        time.Now,
	}
}

// DeliverNext delivers at most one reply intent. Limiting each pass to the
// first claimable item preserves source ordering within one live chat and makes
// retry behavior explicit. Concurrent workers are still safe because Claim is
// the transactional authority.
//
// YouTube's live-chat insert API has no idempotency key. Therefore outbound
// delivery is at-least-once: if PostMessage succeeds but the process crashes or
// CompleteLiveChatReply fails before the durable Delivered marker commits, the
// item may be sent again after its lease expires.
func (w *LiveChatReplyDeliveryWorker) DeliverNext(
	ctx context.Context,
	liveChatID string,
	workerID string,
	options LiveChatReplyDeliveryOptions,
) (LiveChatReplyDeliveryResult, error) {
	if w == nil || w.repository == nil {
		return LiveChatReplyDeliveryResult{}, errors.New("live chat reply delivery repository is nil")
	}
	if w.sender == nil {
		return LiveChatReplyDeliveryResult{}, errors.New("live chat reply sender is nil")
	}
	if w.now == nil {
		return LiveChatReplyDeliveryResult{}, errors.New("live chat reply delivery clock is nil")
	}
	if err := options.validate(); err != nil {
		return LiveChatReplyDeliveryResult{}, err
	}

	now := w.now()
	items, err := w.repository.ListClaimableLiveChatReplies(ctx, liveChatID, now, 1)
	if err != nil {
		return LiveChatReplyDeliveryResult{}, fmt.Errorf("list claimable live chat replies: %w", err)
	}
	if len(items) == 0 {
		return LiveChatReplyDeliveryResult{}, nil
	}
	candidate := items[0]

	claimed, err := w.repository.ClaimLiveChatReply(
		ctx,
		candidate.LiveChatID,
		candidate.SourceMessageID,
		candidate.IntentSlot,
		workerID,
		now,
		options.LeaseDuration,
		options.MaxAttempts,
	)
	if err != nil {
		if errors.Is(err, repository.ErrLiveChatReplyNotClaimable) {
			// Another worker won the race after the advisory list query.
			return LiveChatReplyDeliveryResult{}, nil
		}
		result := LiveChatReplyDeliveryResult{
			DidWork:         errors.Is(err, repository.ErrLiveChatReplyRetryExhausted),
			SourceMessageID: candidate.SourceMessageID,
			IntentSlot:      candidate.IntentSlot,
		}
		if errors.Is(err, repository.ErrLiveChatReplyRetryExhausted) {
			result.Status = repository.LiveChatReplyOutboxDeadLettered
		}
		return result, fmt.Errorf("claim live chat reply: %w", err)
	}

	result := LiveChatReplyDeliveryResult{
		DidWork:         true,
		SourceMessageID: claimed.SourceMessageID,
		IntentSlot:      claimed.IntentSlot,
		Status:          claimed.Status,
	}

	if err := w.sender.PostMessage(ctx, claimed.Message); err != nil {
		failedAt := w.now()
		status, recordErr := w.repository.FailLiveChatReply(
			ctx,
			claimed.LiveChatID,
			claimed.SourceMessageID,
			claimed.IntentSlot,
			workerID,
			failedAt,
			options.MaxAttempts,
			options.RetryDelay,
			err,
		)
		result.Status = status
		if recordErr != nil {
			return result, errors.Join(
				fmt.Errorf("post live chat reply: %w", err),
				fmt.Errorf("record live chat reply failure: %w", recordErr),
			)
		}
		return result, fmt.Errorf("post live chat reply: %w", err)
	}

	completedAt := w.now()
	if err := w.repository.CompleteLiveChatReply(
		ctx,
		claimed.LiveChatID,
		claimed.SourceMessageID,
		claimed.IntentSlot,
		workerID,
		completedAt,
	); err != nil {
		return result, fmt.Errorf(
			"complete live chat reply after external post; delivery may repeat after lease expiry: %w",
			err,
		)
	}
	result.Status = repository.LiveChatReplyOutboxDelivered
	return result, nil
}
