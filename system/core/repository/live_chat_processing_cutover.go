package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const LiveChatProcessingCutover = "live-chat-processing-cutover"

var ErrLiveChatProcessingCutoverCorruptState = errors.New("live chat processing cutover state is inconsistent")

// LiveChatProcessingCutoverDoc freezes the first Inbox sequence that may be
// executed by the durable command worker. Messages below ProcessFromSequence
// belong to the legacy-processing era and must never be replayed by the new
// worker, even when they were captured by durable ingest for observation.
type LiveChatProcessingCutoverDoc struct {
	LiveChatID          string    `firestore:"live-chat-id"`
	ProcessFromSequence int64     `firestore:"process-from-sequence"`
	InitializedAt       time.Time `firestore:"initialized-at"`
}

func (c *FirestoreControllerImplements) liveChatProcessingCutoverCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(LiveChatProcessingCutover)
}

// EnsureLiveChatProcessingCutover initializes an immutable processing boundary.
// If durable ingest has already been running in shadow mode, the current
// StreamState.NextSequence becomes the boundary so all previously captured
// messages stay owned by the legacy processor. If no StreamState exists yet,
// durable processing starts from sequence zero.
//
// Repeated or concurrent calls return the first committed boundary; it never
// moves forward after initialization.
func (c *FirestoreControllerImplements) EnsureLiveChatProcessingCutover(
	ctx context.Context,
	liveChatID string,
	now time.Time,
) (LiveChatProcessingCutoverDoc, error) {
	if strings.TrimSpace(liveChatID) == "" {
		return LiveChatProcessingCutoverDoc{}, errors.New("live chat id is empty")
	}
	if now.IsZero() {
		return LiveChatProcessingCutoverDoc{}, errors.New("cutover initialized at is zero")
	}

	key, err := LiveChatStreamKey(liveChatID)
	if err != nil {
		return LiveChatProcessingCutoverDoc{}, err
	}
	cutoverRef := c.liveChatProcessingCutoverCollection().Doc(key)
	streamRef := c.liveChatStreamStateCollection().Doc(key)

	var result LiveChatProcessingCutoverDoc
	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(cutoverRef)
		if err == nil {
			if err := doc.DataTo(&result); err != nil {
				return fmt.Errorf("decode live chat processing cutover: %w", err)
			}
			return validateLiveChatProcessingCutover(result, liveChatID)
		}
		if status.Code(err) != codes.NotFound {
			return fmt.Errorf("read live chat processing cutover: %w", err)
		}

		streamState, streamExists, err := c.readLiveChatStreamState(tx, streamRef)
		if err != nil {
			return err
		}
		processFromSequence := int64(0)
		if streamExists {
			if streamState.LiveChatID != liveChatID || streamState.NextSequence < 0 {
				return fmt.Errorf("%w: invalid stream state at cutover", ErrLiveChatProcessingCutoverCorruptState)
			}
			processFromSequence = streamState.NextSequence
		}

		result = LiveChatProcessingCutoverDoc{
			LiveChatID:          liveChatID,
			ProcessFromSequence: processFromSequence,
			InitializedAt:       now,
		}
		if err := c.create(ctx, tx, cutoverRef, result); err != nil {
			return fmt.Errorf("create live chat processing cutover: %w", err)
		}
		return nil
	}); err != nil {
		return LiveChatProcessingCutoverDoc{}, fmt.Errorf("ensure live chat processing cutover transaction: %w", err)
	}
	return result, nil
}

func validateLiveChatProcessingCutover(cutover LiveChatProcessingCutoverDoc, liveChatID string) error {
	if cutover.LiveChatID != liveChatID {
		return fmt.Errorf("%w: cutover belongs to a different live chat", ErrLiveChatProcessingCutoverCorruptState)
	}
	if cutover.ProcessFromSequence < 0 {
		return fmt.Errorf("%w: negative process-from sequence", ErrLiveChatProcessingCutoverCorruptState)
	}
	if cutover.InitializedAt.IsZero() {
		return fmt.Errorf("%w: zero initialized-at", ErrLiveChatProcessingCutoverCorruptState)
	}
	return nil
}

// ListClaimableLiveChatInboxMessagesFromSequence is the cutover-aware worker
// query. It has the same lease semantics as ListClaimableLiveChatInboxMessages
// but never returns a message from the legacy-processing era.
func (c *FirestoreControllerImplements) ListClaimableLiveChatInboxMessagesFromSequence(
	ctx context.Context,
	liveChatID string,
	processFromSequence int64,
	now time.Time,
	limit int,
) ([]LiveChatInboxDoc, error) {
	if strings.TrimSpace(liveChatID) == "" {
		return nil, errors.New("live chat id is empty")
	}
	if processFromSequence < 0 {
		return nil, fmt.Errorf("process-from sequence must not be negative: %d", processFromSequence)
	}
	if now.IsZero() {
		return nil, errors.New("now is zero")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive: %d", limit)
	}

	pendingQuery := c.liveChatInboxCollection().
		Where("live-chat-id", "==", liveChatID).
		Where("status", "==", LiveChatInboxPending).
		Where("sequence", ">=", processFromSequence).
		OrderBy("sequence", firestore.Asc).
		Limit(limit)
	pending, err := readLiveChatInboxQuery(ctx, pendingQuery)
	if err != nil {
		return nil, fmt.Errorf("list post-cutover pending live chat inbox messages: %w", err)
	}

	expiredQuery := c.liveChatInboxCollection().
		Where("live-chat-id", "==", liveChatID).
		Where("status", "==", LiveChatInboxProcessing).
		Where("lease-until", "<=", now).
		Where("sequence", ">=", processFromSequence).
		OrderBy("lease-until", firestore.Asc).
		OrderBy("sequence", firestore.Asc).
		Limit(limit)
	expired, err := readLiveChatInboxQuery(ctx, expiredQuery)
	if err != nil {
		return nil, fmt.Errorf("list post-cutover expired live chat inbox messages: %w", err)
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
