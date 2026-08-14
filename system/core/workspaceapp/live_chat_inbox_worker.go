package workspaceapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"app.modules/core/repository"
)

type LiveChatInboxWorkerRepository interface {
	ListClaimableLiveChatInboxMessagesFromSequence(
		ctx context.Context,
		liveChatID string,
		processFromSequence int64,
		now time.Time,
		limit int,
	) ([]repository.LiveChatInboxDoc, error)
	ClaimLiveChatInboxMessage(
		ctx context.Context,
		liveChatID string,
		messageID string,
		workerID string,
		now time.Time,
		leaseDuration time.Duration,
		maxAttempts int,
	) (repository.LiveChatInboxDoc, error)
	FailLiveChatInboxMessage(
		ctx context.Context,
		liveChatID string,
		messageID string,
		workerID string,
		now time.Time,
		maxAttempts int,
		processingErr error,
	) (repository.LiveChatInboxStatus, error)
}

type LiveChatInboxMessageProcessor interface {
	ProcessClaimedDurableInboxMessage(
		ctx context.Context,
		inbox repository.LiveChatInboxDoc,
		workerID string,
	) error
}

type LiveChatInboxWorkerOptions struct {
	ProcessFromSequence int64
	LeaseDuration       time.Duration
	MaxAttempts         int
}

func (o LiveChatInboxWorkerOptions) validate() error {
	if o.ProcessFromSequence < 0 {
		return fmt.Errorf("process-from sequence must not be negative: %d", o.ProcessFromSequence)
	}
	if o.LeaseDuration <= 0 {
		return fmt.Errorf("inbox lease duration must be positive: %s", o.LeaseDuration)
	}
	if o.MaxAttempts <= 0 {
		return fmt.Errorf("inbox max attempts must be positive: %d", o.MaxAttempts)
	}
	return nil
}

type LiveChatInboxWorkerResult struct {
	DidWork   bool
	MessageID string
	Sequence  int64
	Status    repository.LiveChatInboxStatus
}

type LiveChatInboxWorker struct {
	repository LiveChatInboxWorkerRepository
	processor  LiveChatInboxMessageProcessor
	now        func() time.Time
}

func NewLiveChatInboxWorker(
	repo LiveChatInboxWorkerRepository,
	processor LiveChatInboxMessageProcessor,
) *LiveChatInboxWorker {
	return &LiveChatInboxWorker{
		repository: repo,
		processor:  processor,
		now:        time.Now,
	}
}

// ProcessNext handles at most one post-cutover message. Keeping a single head
// item in flight preserves source ordering within one worker. The advisory list
// query never grants ownership; ClaimLiveChatInboxMessage is the transactional
// authority when multiple workers race.
//
// ProcessClaimedDurableInboxMessage owns successful completion because it must
// atomically commit domain effects, reply intents, and Inbox Processed. This
// worker records only failures.
func (w *LiveChatInboxWorker) ProcessNext(
	ctx context.Context,
	liveChatID string,
	workerID string,
	options LiveChatInboxWorkerOptions,
) (LiveChatInboxWorkerResult, error) {
	if w == nil || w.repository == nil {
		return LiveChatInboxWorkerResult{}, errors.New("live chat inbox worker repository is nil")
	}
	if w.processor == nil {
		return LiveChatInboxWorkerResult{}, errors.New("live chat inbox message processor is nil")
	}
	if w.now == nil {
		return LiveChatInboxWorkerResult{}, errors.New("live chat inbox worker clock is nil")
	}
	if err := options.validate(); err != nil {
		return LiveChatInboxWorkerResult{}, err
	}

	now := w.now()
	items, err := w.repository.ListClaimableLiveChatInboxMessagesFromSequence(
		ctx,
		liveChatID,
		options.ProcessFromSequence,
		now,
		1,
	)
	if err != nil {
		return LiveChatInboxWorkerResult{}, fmt.Errorf("list claimable durable live chat messages: %w", err)
	}
	if len(items) == 0 {
		return LiveChatInboxWorkerResult{}, nil
	}
	candidate := items[0]

	claimed, err := w.repository.ClaimLiveChatInboxMessage(
		ctx,
		candidate.LiveChatID,
		candidate.MessageID,
		workerID,
		now,
		options.LeaseDuration,
		options.MaxAttempts,
	)
	if err != nil {
		if errors.Is(err, repository.ErrLiveChatInboxNotClaimable) {
			return LiveChatInboxWorkerResult{}, nil
		}
		result := LiveChatInboxWorkerResult{
			DidWork:   errors.Is(err, repository.ErrLiveChatInboxRetryExhausted),
			MessageID: candidate.MessageID,
			Sequence:  candidate.Sequence,
		}
		if errors.Is(err, repository.ErrLiveChatInboxRetryExhausted) {
			result.Status = repository.LiveChatInboxDeadLettered
		}
		return result, fmt.Errorf("claim durable live chat message: %w", err)
	}

	result := LiveChatInboxWorkerResult{
		DidWork:   true,
		MessageID: claimed.MessageID,
		Sequence:  claimed.Sequence,
		Status:    claimed.Status,
	}

	processingErr := w.processor.ProcessClaimedDurableInboxMessage(ctx, claimed, workerID)
	if processingErr == nil {
		result.Status = repository.LiveChatInboxProcessed
		return result, nil
	}

	if errors.Is(processingErr, ErrDurableCommandNotSupported) {
		// A migration capability gap is not a poison message. Do not consume the
		// retry budget or skip the message. The uncompleted lease deliberately
		// stalls ordered processing until the command is migrated or the runtime
		// is disabled.
		return result, fmt.Errorf("durable command coverage is incomplete: %w", processingErr)
	}

	failedAt := w.now()
	failureStatus, recordErr := w.repository.FailLiveChatInboxMessage(
		ctx,
		claimed.LiveChatID,
		claimed.MessageID,
		workerID,
		failedAt,
		options.MaxAttempts,
		processingErr,
	)
	result.Status = failureStatus
	if recordErr != nil {
		return result, errors.Join(
			fmt.Errorf("process durable live chat message: %w", processingErr),
			fmt.Errorf("record durable live chat processing failure: %w", recordErr),
		)
	}
	return result, fmt.Errorf("process durable live chat message: %w", processingErr)
}
