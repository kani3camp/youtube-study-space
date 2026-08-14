package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	LiveChatInboxProcessing   LiveChatInboxStatus = "processing"
	LiveChatInboxProcessed    LiveChatInboxStatus = "processed"
	LiveChatInboxDeadLettered LiveChatInboxStatus = "dead-lettered"
)

var (
	ErrLiveChatInboxNotClaimable   = errors.New("live chat inbox message is not claimable")
	ErrLiveChatInboxLeaseLost      = errors.New("live chat inbox processing lease is not owned or has expired")
	ErrLiveChatInboxRetryExhausted = errors.New("live chat inbox processing retry limit exhausted")
	ErrLiveChatInboxCorruptState   = errors.New("live chat inbox message has inconsistent processing state")
)

// ListClaimableLiveChatInboxMessages returns pending messages and processing
// messages whose leases have expired. Results are merged by durable sequence so
// workers preserve source ordering as far as the current bounded batch allows.
// ClaimLiveChatInboxMessage remains the authority: listing is only advisory and
// concurrent workers must still claim each message transactionally.
func (c *FirestoreControllerImplements) ListClaimableLiveChatInboxMessages(
	ctx context.Context,
	liveChatID string,
	now time.Time,
	limit int,
) ([]LiveChatInboxDoc, error) {
	if strings.TrimSpace(liveChatID) == "" {
		return nil, errors.New("live chat id is empty")
	}
	if now.IsZero() {
		return nil, errors.New("now is zero")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive: %d", limit)
	}

	pending, err := c.listPendingLiveChatInboxMessages(ctx, liveChatID, limit)
	if err != nil {
		return nil, err
	}
	expired, err := c.listExpiredLiveChatInboxMessages(ctx, liveChatID, now, limit)
	if err != nil {
		return nil, err
	}

	messages := append(pending, expired...)
	sort.SliceStable(messages, func(i, j int) bool {
		return messages[i].Sequence < messages[j].Sequence
	})
	if len(messages) > limit {
		messages = messages[:limit]
	}
	return messages, nil
}

func (c *FirestoreControllerImplements) listPendingLiveChatInboxMessages(
	ctx context.Context,
	liveChatID string,
	limit int,
) ([]LiveChatInboxDoc, error) {
	query := c.liveChatInboxCollection().
		Where("live-chat-id", "==", liveChatID).
		Where("status", "==", LiveChatInboxPending).
		OrderBy("sequence", firestore.Asc).
		Limit(limit)
	return readLiveChatInboxQuery(ctx, query)
}

func (c *FirestoreControllerImplements) listExpiredLiveChatInboxMessages(
	ctx context.Context,
	liveChatID string,
	now time.Time,
	limit int,
) ([]LiveChatInboxDoc, error) {
	query := c.liveChatInboxCollection().
		Where("live-chat-id", "==", liveChatID).
		Where("status", "==", LiveChatInboxProcessing).
		Where("lease-until", "<=", now).
		OrderBy("lease-until", firestore.Asc).
		OrderBy("sequence", firestore.Asc).
		Limit(limit)
	return readLiveChatInboxQuery(ctx, query)
}

func readLiveChatInboxQuery(ctx context.Context, query firestore.Query) ([]LiveChatInboxDoc, error) {
	iter := query.Documents(ctx)
	defer iter.Stop()

	var messages []LiveChatInboxDoc
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate live chat inbox query: %w", err)
		}
		var message LiveChatInboxDoc
		if err := doc.DataTo(&message); err != nil {
			return nil, fmt.Errorf("decode live chat inbox document %q: %w", doc.Ref.Path, err)
		}
		messages = append(messages, message)
	}
	return messages, nil
}

// ClaimLiveChatInboxMessage acquires or reacquires a processing lease for one
// deterministic source message. AttemptCount counts processing leases, including
// a lease reclaimed after a worker crash. When the configured maximum has
// already been consumed, the message is atomically dead-lettered instead.
func (c *FirestoreControllerImplements) ClaimLiveChatInboxMessage(
	ctx context.Context,
	liveChatID string,
	messageID string,
	workerID string,
	now time.Time,
	leaseDuration time.Duration,
	maxAttempts int,
) (LiveChatInboxDoc, error) {
	if strings.TrimSpace(workerID) == "" {
		return LiveChatInboxDoc{}, errors.New("worker id is empty")
	}
	if now.IsZero() {
		return LiveChatInboxDoc{}, errors.New("now is zero")
	}
	if leaseDuration <= 0 {
		return LiveChatInboxDoc{}, fmt.Errorf("lease duration must be positive: %s", leaseDuration)
	}
	if maxAttempts <= 0 {
		return LiveChatInboxDoc{}, fmt.Errorf("max attempts must be positive: %d", maxAttempts)
	}

	ref, err := c.liveChatInboxMessageRef(liveChatID, messageID)
	if err != nil {
		return LiveChatInboxDoc{}, err
	}

	var claimed LiveChatInboxDoc
	deadLettered := false
	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		message, err := readLiveChatInboxMessageTx(tx, ref)
		if err != nil {
			return err
		}
		if message.LiveChatID != liveChatID || message.MessageID != messageID {
			return fmt.Errorf("%w: deterministic key points to different source message", ErrLiveChatInboxCorruptState)
		}

		claimable := false
		switch message.Status {
		case LiveChatInboxPending:
			if message.LeaseOwner != "" || message.LeaseUntil != nil {
				return fmt.Errorf("%w: pending message still has a lease", ErrLiveChatInboxCorruptState)
			}
			claimable = true
		case LiveChatInboxProcessing:
			if message.LeaseOwner == "" || message.LeaseUntil == nil {
				return fmt.Errorf("%w: processing message is missing lease metadata", ErrLiveChatInboxCorruptState)
			}
			claimable = !message.LeaseUntil.After(now)
		case LiveChatInboxProcessed, LiveChatInboxDeadLettered:
			return ErrLiveChatInboxNotClaimable
		default:
			return fmt.Errorf("%w: unknown status %q", ErrLiveChatInboxCorruptState, message.Status)
		}
		if !claimable {
			return ErrLiveChatInboxNotClaimable
		}

		if message.AttemptCount >= maxAttempts {
			message.Status = LiveChatInboxDeadLettered
			message.LeaseOwner = ""
			message.LeaseUntil = nil
			if message.LastError == "" {
				message.LastError = "processing lease expired after retry limit was exhausted"
			}
			if err := c.set(ctx, tx, ref, message); err != nil {
				return fmt.Errorf("dead-letter exhausted live chat inbox message: %w", err)
			}
			claimed = message
			deadLettered = true
			return nil
		}

		leaseUntil := now.Add(leaseDuration)
		message.Status = LiveChatInboxProcessing
		message.AttemptCount++
		message.LeaseOwner = workerID
		message.LeaseUntil = &leaseUntil
		if err := c.set(ctx, tx, ref, message); err != nil {
			return fmt.Errorf("claim live chat inbox message: %w", err)
		}
		claimed = message
		return nil
	}); err != nil {
		return LiveChatInboxDoc{}, fmt.Errorf("claim live chat inbox transaction: %w", err)
	}
	if deadLettered {
		return claimed, ErrLiveChatInboxRetryExhausted
	}
	return claimed, nil
}

// CompleteLiveChatInboxMessage marks a message processed only while the caller
// still owns an unexpired lease. Once a lease expires, the old worker must not
// commit completion because another worker is allowed to reclaim the message.
func (c *FirestoreControllerImplements) CompleteLiveChatInboxMessage(
	ctx context.Context,
	liveChatID string,
	messageID string,
	workerID string,
	now time.Time,
) error {
	if strings.TrimSpace(workerID) == "" {
		return errors.New("worker id is empty")
	}
	if now.IsZero() {
		return errors.New("now is zero")
	}
	ref, err := c.liveChatInboxMessageRef(liveChatID, messageID)
	if err != nil {
		return err
	}

	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		message, err := readLiveChatInboxMessageTx(tx, ref)
		if err != nil {
			return err
		}
		if err := validateLiveChatInboxLease(message, workerID, now); err != nil {
			return err
		}
		processedAt := now
		message.Status = LiveChatInboxProcessed
		message.ProcessedAt = &processedAt
		message.LeaseOwner = ""
		message.LeaseUntil = nil
		message.LastError = ""
		if err := c.set(ctx, tx, ref, message); err != nil {
			return fmt.Errorf("complete live chat inbox message: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("complete live chat inbox transaction: %w", err)
	}
	return nil
}

// FailLiveChatInboxMessage records a processing failure while the caller still
// owns the lease. Attempts below maxAttempts return the message to Pending;
// reaching maxAttempts moves it to DeadLettered so poison messages cannot block
// later source messages forever.
func (c *FirestoreControllerImplements) FailLiveChatInboxMessage(
	ctx context.Context,
	liveChatID string,
	messageID string,
	workerID string,
	now time.Time,
	maxAttempts int,
	processingErr error,
) (LiveChatInboxStatus, error) {
	if strings.TrimSpace(workerID) == "" {
		return "", errors.New("worker id is empty")
	}
	if now.IsZero() {
		return "", errors.New("now is zero")
	}
	if maxAttempts <= 0 {
		return "", fmt.Errorf("max attempts must be positive: %d", maxAttempts)
	}
	if processingErr == nil {
		return "", errors.New("processing error is nil")
	}
	ref, err := c.liveChatInboxMessageRef(liveChatID, messageID)
	if err != nil {
		return "", err
	}

	var resultStatus LiveChatInboxStatus
	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		message, err := readLiveChatInboxMessageTx(tx, ref)
		if err != nil {
			return err
		}
		if err := validateLiveChatInboxLease(message, workerID, now); err != nil {
			return err
		}

		message.LastError = processingErr.Error()
		message.LeaseOwner = ""
		message.LeaseUntil = nil
		if message.AttemptCount >= maxAttempts {
			message.Status = LiveChatInboxDeadLettered
		} else {
			message.Status = LiveChatInboxPending
		}
		if err := c.set(ctx, tx, ref, message); err != nil {
			return fmt.Errorf("record live chat inbox failure: %w", err)
		}
		resultStatus = message.Status
		return nil
	}); err != nil {
		return "", fmt.Errorf("fail live chat inbox transaction: %w", err)
	}
	return resultStatus, nil
}

func (c *FirestoreControllerImplements) liveChatInboxMessageRef(
	liveChatID string,
	messageID string,
) (*firestore.DocumentRef, error) {
	key, err := LiveChatMessageKey(liveChatID, messageID)
	if err != nil {
		return nil, err
	}
	return c.liveChatInboxCollection().Doc(key), nil
}

func readLiveChatInboxMessageTx(
	tx *firestore.Transaction,
	ref *firestore.DocumentRef,
) (LiveChatInboxDoc, error) {
	doc, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return LiveChatInboxDoc{}, fmt.Errorf("live chat inbox document not found: %w", err)
	}
	if err != nil {
		return LiveChatInboxDoc{}, fmt.Errorf("get live chat inbox document: %w", err)
	}
	var message LiveChatInboxDoc
	if err := doc.DataTo(&message); err != nil {
		return LiveChatInboxDoc{}, fmt.Errorf("decode live chat inbox document: %w", err)
	}
	return message, nil
}

func validateLiveChatInboxLease(message LiveChatInboxDoc, workerID string, now time.Time) error {
	if message.Status != LiveChatInboxProcessing {
		return ErrLiveChatInboxLeaseLost
	}
	if message.LeaseOwner != workerID || message.LeaseUntil == nil || !message.LeaseUntil.After(now) {
		return ErrLiveChatInboxLeaseLost
	}
	return nil
}
