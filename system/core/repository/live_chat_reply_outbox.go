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

type LiveChatReplyOutboxStatus string

const (
	LiveChatReplyOutboxPending      LiveChatReplyOutboxStatus = "pending"
	LiveChatReplyOutboxDelivering   LiveChatReplyOutboxStatus = "delivering"
	LiveChatReplyOutboxDelivered    LiveChatReplyOutboxStatus = "delivered"
	LiveChatReplyOutboxDeadLettered LiveChatReplyOutboxStatus = "dead-lettered"
)

var (
	ErrLiveChatReplyIntentAlreadyExists = errors.New("live chat reply intent already exists")
	ErrLiveChatReplyNotClaimable        = errors.New("live chat reply outbox item is not claimable")
	ErrLiveChatReplyLeaseLost           = errors.New("live chat reply delivery lease is not owned or has expired")
	ErrLiveChatReplyRetryExhausted      = errors.New("live chat reply delivery retry limit exhausted")
	ErrLiveChatReplyCorruptState        = errors.New("live chat reply outbox item has inconsistent state")
)

type LiveChatReplyOutboxDoc struct {
	LiveChatID            string                    `firestore:"live-chat-id"`
	SourceMessageID       string                    `firestore:"source-message-id"`
	SourceAuthorChannelID string                    `firestore:"source-author-channel-id"`
	IntentSlot            string                    `firestore:"intent-slot"`
	SourceSequence        int64                     `firestore:"source-sequence"`
	Message               string                    `firestore:"message"`
	Status                LiveChatReplyOutboxStatus `firestore:"status"`
	AttemptCount          int                       `firestore:"attempt-count"`
	LastError             string                    `firestore:"last-error"`
	CreatedAt             time.Time                 `firestore:"created-at"`
	AvailableAt           time.Time                 `firestore:"available-at"`
	DeliveredAt           *time.Time                `firestore:"delivered-at,omitempty"`
	LeaseOwner            string                    `firestore:"lease-owner"`
	LeaseUntil            *time.Time                `firestore:"lease-until,omitempty"`
}

// LiveChatReplyOutboxKey identifies one logical reply intent. IntentSlot lets a
// future command persist more than one logical reply for the same source
// message without relying on random document IDs.
func LiveChatReplyOutboxKey(liveChatID, sourceMessageID, intentSlot string) (string, error) {
	if strings.TrimSpace(liveChatID) == "" {
		return "", errors.New("live chat id is empty")
	}
	if strings.TrimSpace(sourceMessageID) == "" {
		return "", errors.New("source message id is empty")
	}
	if strings.TrimSpace(intentSlot) == "" {
		return "", errors.New("reply intent slot is empty")
	}
	return liveChatCompositeKey("reply", liveChatID, sourceMessageID, intentSlot)
}

func (c *FirestoreControllerImplements) liveChatReplyOutboxCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(LiveChatReplyOutbox)
}

// CreateLiveChatReplyIntent must be called inside the same Firestore
// transaction as the corresponding domain effect. Requiring tx here prevents
// callers from accidentally persisting a reply intent after the domain commit.
// The deterministic key also makes duplicate intent creation observable.
func (c *FirestoreControllerImplements) CreateLiveChatReplyIntent(
	ctx context.Context,
	tx *firestore.Transaction,
	intent LiveChatReplyOutboxDoc,
) error {
	if tx == nil {
		return errors.New("live chat reply intent requires a Firestore transaction")
	}
	if err := validateNewLiveChatReplyIntent(intent); err != nil {
		return err
	}
	key, err := LiveChatReplyOutboxKey(intent.LiveChatID, intent.SourceMessageID, intent.IntentSlot)
	if err != nil {
		return err
	}
	ref := c.liveChatReplyOutboxCollection().Doc(key)
	if err := c.create(ctx, tx, ref, intent); err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return fmt.Errorf("%w: source-message=%q slot=%q", ErrLiveChatReplyIntentAlreadyExists, intent.SourceMessageID, intent.IntentSlot)
		}
		return fmt.Errorf("create live chat reply intent: %w", err)
	}
	return nil
}

func validateNewLiveChatReplyIntent(intent LiveChatReplyOutboxDoc) error {
	if strings.TrimSpace(intent.LiveChatID) == "" {
		return errors.New("live chat id is empty")
	}
	if strings.TrimSpace(intent.SourceMessageID) == "" {
		return errors.New("source message id is empty")
	}
	if strings.TrimSpace(intent.IntentSlot) == "" {
		return errors.New("reply intent slot is empty")
	}
	if intent.SourceSequence < 0 {
		return fmt.Errorf("source sequence must not be negative: %d", intent.SourceSequence)
	}
	if intent.Message == "" {
		return errors.New("reply message is empty")
	}
	if intent.CreatedAt.IsZero() {
		return errors.New("reply intent created at is zero")
	}
	if intent.AvailableAt.IsZero() {
		return errors.New("reply intent available at is zero")
	}
	if intent.Status != LiveChatReplyOutboxPending {
		return fmt.Errorf("new reply intent must be pending, got %q", intent.Status)
	}
	if intent.AttemptCount != 0 || intent.LastError != "" || intent.DeliveredAt != nil || intent.LeaseOwner != "" || intent.LeaseUntil != nil {
		return errors.New("new reply intent contains delivery state")
	}
	return nil
}

// ListClaimableLiveChatReplies returns due Pending items and Delivering items
// whose leases have expired. Listing is advisory; ClaimLiveChatReply is the
// transactional authority under concurrent delivery workers.
func (c *FirestoreControllerImplements) ListClaimableLiveChatReplies(
	ctx context.Context,
	liveChatID string,
	now time.Time,
	limit int,
) ([]LiveChatReplyOutboxDoc, error) {
	if strings.TrimSpace(liveChatID) == "" {
		return nil, errors.New("live chat id is empty")
	}
	if now.IsZero() {
		return nil, errors.New("now is zero")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive: %d", limit)
	}

	pending, err := c.listPendingLiveChatReplies(ctx, liveChatID, now, limit)
	if err != nil {
		return nil, err
	}
	expired, err := c.listExpiredLiveChatReplies(ctx, liveChatID, now, limit)
	if err != nil {
		return nil, err
	}
	items := append(pending, expired...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SourceSequence == items[j].SourceSequence {
			return items[i].IntentSlot < items[j].IntentSlot
		}
		return items[i].SourceSequence < items[j].SourceSequence
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (c *FirestoreControllerImplements) listPendingLiveChatReplies(
	ctx context.Context,
	liveChatID string,
	now time.Time,
	limit int,
) ([]LiveChatReplyOutboxDoc, error) {
	query := c.liveChatReplyOutboxCollection().
		Where("live-chat-id", "==", liveChatID).
		Where("status", "==", LiveChatReplyOutboxPending).
		Where("available-at", "<=", now).
		OrderBy("available-at", firestore.Asc).
		OrderBy("source-sequence", firestore.Asc).
		Limit(limit)
	return readLiveChatReplyOutboxQuery(ctx, query)
}

func (c *FirestoreControllerImplements) listExpiredLiveChatReplies(
	ctx context.Context,
	liveChatID string,
	now time.Time,
	limit int,
) ([]LiveChatReplyOutboxDoc, error) {
	query := c.liveChatReplyOutboxCollection().
		Where("live-chat-id", "==", liveChatID).
		Where("status", "==", LiveChatReplyOutboxDelivering).
		Where("lease-until", "<=", now).
		OrderBy("lease-until", firestore.Asc).
		OrderBy("source-sequence", firestore.Asc).
		Limit(limit)
	return readLiveChatReplyOutboxQuery(ctx, query)
}

func readLiveChatReplyOutboxQuery(ctx context.Context, query firestore.Query) ([]LiveChatReplyOutboxDoc, error) {
	iter := query.Documents(ctx)
	defer iter.Stop()

	var items []LiveChatReplyOutboxDoc
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate live chat reply outbox query: %w", err)
		}
		var item LiveChatReplyOutboxDoc
		if err := doc.DataTo(&item); err != nil {
			return nil, fmt.Errorf("decode live chat reply outbox document %q: %w", doc.Ref.Path, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (c *FirestoreControllerImplements) ClaimLiveChatReply(
	ctx context.Context,
	liveChatID, sourceMessageID, intentSlot, workerID string,
	now time.Time,
	leaseDuration time.Duration,
	maxAttempts int,
) (LiveChatReplyOutboxDoc, error) {
	if strings.TrimSpace(workerID) == "" {
		return LiveChatReplyOutboxDoc{}, errors.New("worker id is empty")
	}
	if now.IsZero() {
		return LiveChatReplyOutboxDoc{}, errors.New("now is zero")
	}
	if leaseDuration <= 0 {
		return LiveChatReplyOutboxDoc{}, fmt.Errorf("lease duration must be positive: %s", leaseDuration)
	}
	if maxAttempts <= 0 {
		return LiveChatReplyOutboxDoc{}, fmt.Errorf("max attempts must be positive: %d", maxAttempts)
	}
	ref, err := c.liveChatReplyOutboxRef(liveChatID, sourceMessageID, intentSlot)
	if err != nil {
		return LiveChatReplyOutboxDoc{}, err
	}

	var claimed LiveChatReplyOutboxDoc
	deadLettered := false
	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		item, err := readLiveChatReplyOutboxTx(tx, ref)
		if err != nil {
			return err
		}
		if item.LiveChatID != liveChatID || item.SourceMessageID != sourceMessageID || item.IntentSlot != intentSlot {
			return fmt.Errorf("%w: deterministic key points to different reply intent", ErrLiveChatReplyCorruptState)
		}

		claimable := false
		switch item.Status {
		case LiveChatReplyOutboxPending:
			if item.LeaseOwner != "" || item.LeaseUntil != nil {
				return fmt.Errorf("%w: pending reply still has a lease", ErrLiveChatReplyCorruptState)
			}
			claimable = !item.AvailableAt.After(now)
		case LiveChatReplyOutboxDelivering:
			if item.LeaseOwner == "" || item.LeaseUntil == nil {
				return fmt.Errorf("%w: delivering reply is missing lease metadata", ErrLiveChatReplyCorruptState)
			}
			claimable = !item.LeaseUntil.After(now)
		case LiveChatReplyOutboxDelivered, LiveChatReplyOutboxDeadLettered:
			return ErrLiveChatReplyNotClaimable
		default:
			return fmt.Errorf("%w: unknown status %q", ErrLiveChatReplyCorruptState, item.Status)
		}
		if !claimable {
			return ErrLiveChatReplyNotClaimable
		}

		if item.AttemptCount >= maxAttempts {
			item.Status = LiveChatReplyOutboxDeadLettered
			item.LeaseOwner = ""
			item.LeaseUntil = nil
			if item.LastError == "" {
				item.LastError = "delivery lease expired after retry limit was exhausted"
			}
			if err := c.set(ctx, tx, ref, item); err != nil {
				return fmt.Errorf("dead-letter exhausted live chat reply: %w", err)
			}
			claimed = item
			deadLettered = true
			return nil
		}

		leaseUntil := now.Add(leaseDuration)
		item.Status = LiveChatReplyOutboxDelivering
		item.AttemptCount++
		item.LeaseOwner = workerID
		item.LeaseUntil = &leaseUntil
		if err := c.set(ctx, tx, ref, item); err != nil {
			return fmt.Errorf("claim live chat reply: %w", err)
		}
		claimed = item
		return nil
	}); err != nil {
		return LiveChatReplyOutboxDoc{}, fmt.Errorf("claim live chat reply transaction: %w", err)
	}
	if deadLettered {
		return claimed, ErrLiveChatReplyRetryExhausted
	}
	return claimed, nil
}

func (c *FirestoreControllerImplements) CompleteLiveChatReply(
	ctx context.Context,
	liveChatID, sourceMessageID, intentSlot, workerID string,
	now time.Time,
) error {
	if strings.TrimSpace(workerID) == "" {
		return errors.New("worker id is empty")
	}
	if now.IsZero() {
		return errors.New("now is zero")
	}
	ref, err := c.liveChatReplyOutboxRef(liveChatID, sourceMessageID, intentSlot)
	if err != nil {
		return err
	}
	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		item, err := readLiveChatReplyOutboxTx(tx, ref)
		if err != nil {
			return err
		}
		if err := validateLiveChatReplyLease(item, workerID, now); err != nil {
			return err
		}
		deliveredAt := now
		item.Status = LiveChatReplyOutboxDelivered
		item.DeliveredAt = &deliveredAt
		item.LeaseOwner = ""
		item.LeaseUntil = nil
		item.LastError = ""
		if err := c.set(ctx, tx, ref, item); err != nil {
			return fmt.Errorf("complete live chat reply: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("complete live chat reply transaction: %w", err)
	}
	return nil
}

func (c *FirestoreControllerImplements) FailLiveChatReply(
	ctx context.Context,
	liveChatID, sourceMessageID, intentSlot, workerID string,
	now time.Time,
	maxAttempts int,
	retryDelay time.Duration,
	deliveryErr error,
) (LiveChatReplyOutboxStatus, error) {
	if strings.TrimSpace(workerID) == "" {
		return "", errors.New("worker id is empty")
	}
	if now.IsZero() {
		return "", errors.New("now is zero")
	}
	if maxAttempts <= 0 {
		return "", fmt.Errorf("max attempts must be positive: %d", maxAttempts)
	}
	if retryDelay < 0 {
		return "", fmt.Errorf("retry delay must not be negative: %s", retryDelay)
	}
	if deliveryErr == nil {
		return "", errors.New("delivery error is nil")
	}
	ref, err := c.liveChatReplyOutboxRef(liveChatID, sourceMessageID, intentSlot)
	if err != nil {
		return "", err
	}

	var result LiveChatReplyOutboxStatus
	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		item, err := readLiveChatReplyOutboxTx(tx, ref)
		if err != nil {
			return err
		}
		if err := validateLiveChatReplyLease(item, workerID, now); err != nil {
			return err
		}
		item.LastError = deliveryErr.Error()
		item.LeaseOwner = ""
		item.LeaseUntil = nil
		if item.AttemptCount >= maxAttempts {
			item.Status = LiveChatReplyOutboxDeadLettered
		} else {
			item.Status = LiveChatReplyOutboxPending
			item.AvailableAt = now.Add(retryDelay)
		}
		if err := c.set(ctx, tx, ref, item); err != nil {
			return fmt.Errorf("record live chat reply failure: %w", err)
		}
		result = item.Status
		return nil
	}); err != nil {
		return "", fmt.Errorf("fail live chat reply transaction: %w", err)
	}
	return result, nil
}

func (c *FirestoreControllerImplements) liveChatReplyOutboxRef(liveChatID, sourceMessageID, intentSlot string) (*firestore.DocumentRef, error) {
	key, err := LiveChatReplyOutboxKey(liveChatID, sourceMessageID, intentSlot)
	if err != nil {
		return nil, err
	}
	return c.liveChatReplyOutboxCollection().Doc(key), nil
}

func readLiveChatReplyOutboxTx(tx *firestore.Transaction, ref *firestore.DocumentRef) (LiveChatReplyOutboxDoc, error) {
	doc, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return LiveChatReplyOutboxDoc{}, fmt.Errorf("live chat reply outbox document not found: %w", err)
	}
	if err != nil {
		return LiveChatReplyOutboxDoc{}, fmt.Errorf("get live chat reply outbox document: %w", err)
	}
	var item LiveChatReplyOutboxDoc
	if err := doc.DataTo(&item); err != nil {
		return LiveChatReplyOutboxDoc{}, fmt.Errorf("decode live chat reply outbox document: %w", err)
	}
	return item, nil
}

func validateLiveChatReplyLease(item LiveChatReplyOutboxDoc, workerID string, now time.Time) error {
	if item.Status != LiveChatReplyOutboxDelivering {
		return ErrLiveChatReplyLeaseLost
	}
	if item.LeaseOwner != workerID || item.LeaseUntil == nil || !item.LeaseUntil.After(now) {
		return ErrLiveChatReplyLeaseLost
	}
	return nil
}
