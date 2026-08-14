package repository

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	LiveChatInboxPending LiveChatInboxStatus = "pending"

	// One new message creates one Inbox document and one deterministic History
	// document. Keep one write for StreamState below Firestore's 500-write limit.
	MaxAtomicLiveChatIngestMessages = (FirestoreWritesLimitPerRequest - 1) / 2
)

var (
	ErrLiveChatStreamCursorConflict = errors.New("live chat stream cursor conflict")
	ErrLiveChatIngestCorruptState   = errors.New("live chat ingest durable state is inconsistent")
)

type LiveChatInboxStatus string

type LiveChatInboxDoc struct {
	LiveChatID            string              `firestore:"live-chat-id"`
	MessageID             string              `firestore:"message-id"`
	Sequence              int64               `firestore:"sequence"`
	AuthorChannelID       string              `firestore:"author-channel-id"`
	AuthorDisplayName     string              `firestore:"author-display-name"`
	AuthorProfileImageURL string              `firestore:"author-profile-image-url"`
	AuthorIsChatModerator bool                `firestore:"author-is-chat-moderator"`
	MessageText           string              `firestore:"message-text"`
	Type                  string              `firestore:"type"`
	PublishedAt           time.Time           `firestore:"published-at"`
	Status                LiveChatInboxStatus `firestore:"status"`
	AttemptCount          int                 `firestore:"attempt-count"`
	LastError             string              `firestore:"last-error"`
	IngestedAt            time.Time           `firestore:"ingested-at"`
	ProcessedAt           *time.Time          `firestore:"processed-at,omitempty"`
	LeaseOwner            string              `firestore:"lease-owner"`
	LeaseUntil            *time.Time          `firestore:"lease-until,omitempty"`
}

type LiveChatStreamStateDoc struct {
	LiveChatID           string    `firestore:"live-chat-id"`
	NextPageToken        string    `firestore:"next-page-token"`
	NextSequence         int64     `firestore:"next-sequence"`
	UpdatedAt            time.Time `firestore:"updated-at"`
	LastFetchSucceededAt time.Time `firestore:"last-fetch-succeeded-at"`
}

// LiveChatMessageKey returns a deterministic Firestore-safe document ID for a
// source YouTube message. The length-delimited hash avoids delimiter ambiguity
// and keeps raw LiveChat/Message IDs out of document paths.
func LiveChatMessageKey(liveChatID string, messageID string) (string, error) {
	if strings.TrimSpace(liveChatID) == "" {
		return "", errors.New("live chat id is empty")
	}
	if strings.TrimSpace(messageID) == "" {
		return "", errors.New("live chat message id is empty")
	}
	return liveChatCompositeKey("message", liveChatID, messageID), nil
}

// LiveChatStreamKey returns a deterministic Firestore-safe document ID for one
// live chat's ingestion cursor/state.
func LiveChatStreamKey(liveChatID string) (string, error) {
	if strings.TrimSpace(liveChatID) == "" {
		return "", errors.New("live chat id is empty")
	}
	return liveChatCompositeKey("stream", liveChatID), nil
}

func liveChatCompositeKey(kind string, parts ...string) string {
	h := sha256.New()
	writeHashPart(h, kind)
	for _, part := range parts {
		writeHashPart(h, part)
	}
	return hex.EncodeToString(h.Sum(nil))
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writeHashPart(w hashWriter, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = w.Write(length[:])
	_, _ = w.Write([]byte(value))
}

func (c *FirestoreControllerImplements) liveChatInboxCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(LiveChatInbox)
}

func (c *FirestoreControllerImplements) liveChatStreamStateCollection() *firestore.CollectionRef {
	return c.firestoreClient.Collection(LiveChatStreamState)
}

// IngestLiveChatPage atomically persists new source messages to both Inbox and
// deterministic History documents, then advances StreamState. Replaying the
// same page is idempotent. Once StreamState exists, expectedPageToken must match
// the persisted cursor so a stale poller cannot move the cursor backward.
//
// This method is intentionally not wired into youtube-bot yet. The caller must
// limit fetched pages so two writes per new message plus StreamState fit within
// Firestore's transaction write limit.
func (c *FirestoreControllerImplements) IngestLiveChatPage(
	ctx context.Context,
	liveChatID string,
	expectedPageToken string,
	nextPageToken string,
	messages []LiveChatHistoryDoc,
	ingestedAt time.Time,
) error {
	if strings.TrimSpace(liveChatID) == "" {
		return errors.New("live chat id is empty")
	}
	if ingestedAt.IsZero() {
		return errors.New("ingested at is zero")
	}

	uniqueMessages, err := validateAndDeduplicateLiveChatMessages(liveChatID, messages)
	if err != nil {
		return err
	}
	if len(uniqueMessages) > MaxAtomicLiveChatIngestMessages {
		return fmt.Errorf(
			"live chat page has %d unique messages; maximum atomic ingest size is %d",
			len(uniqueMessages),
			MaxAtomicLiveChatIngestMessages,
		)
	}

	streamKey, err := LiveChatStreamKey(liveChatID)
	if err != nil {
		return err
	}
	streamRef := c.liveChatStreamStateCollection().Doc(streamKey)

	if err := c.firestoreClient.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		streamState, streamExists, err := c.readLiveChatStreamState(tx, streamRef)
		if err != nil {
			return err
		}
		if streamExists {
			if streamState.LiveChatID != liveChatID {
				return fmt.Errorf("%w: stream key maps to different live chat id", ErrLiveChatIngestCorruptState)
			}
			if streamState.NextPageToken != expectedPageToken {
				return fmt.Errorf(
					"%w: persisted=%q expected=%q",
					ErrLiveChatStreamCursorConflict,
					streamState.NextPageToken,
					expectedPageToken,
				)
			}
		} else {
			streamState = LiveChatStreamStateDoc{
				LiveChatID:   liveChatID,
				NextSequence: 0,
			}
		}

		type messageWrite struct {
			key     string
			message LiveChatHistoryDoc
		}
		newMessages := make([]messageWrite, 0, len(uniqueMessages))

		// Firestore transactions require reads before writes. Verify both durable
		// copies for every key before staging any Create/Set operations.
		for _, message := range uniqueMessages {
			key, keyErr := LiveChatMessageKey(liveChatID, message.ID)
			if keyErr != nil {
				return keyErr
			}
			inboxRef := c.liveChatInboxCollection().Doc(key)
			historyRef := c.liveChatHistoryCollection().Doc(key)

			inboxExists, err := transactionDocumentExists(tx, inboxRef)
			if err != nil {
				return fmt.Errorf("read live chat inbox document: %w", err)
			}
			historyExists, err := transactionDocumentExists(tx, historyRef)
			if err != nil {
				return fmt.Errorf("read deterministic live chat history document: %w", err)
			}
			if inboxExists != historyExists {
				return fmt.Errorf(
					"%w: inbox/history existence differs for message %q",
					ErrLiveChatIngestCorruptState,
					message.ID,
				)
			}
			if inboxExists {
				continue
			}
			newMessages = append(newMessages, messageWrite{key: key, message: message})
		}

		nextSequence := streamState.NextSequence
		for _, item := range newMessages {
			inbox := liveChatInboxFromHistory(item.message, nextSequence, ingestedAt)
			if err := c.create(ctx, tx, c.liveChatInboxCollection().Doc(item.key), inbox); err != nil {
				return fmt.Errorf("create live chat inbox document: %w", err)
			}
			if err := c.create(ctx, tx, c.liveChatHistoryCollection().Doc(item.key), item.message); err != nil {
				return fmt.Errorf("create deterministic live chat history document: %w", err)
			}
			nextSequence++
		}

		newStreamState := LiveChatStreamStateDoc{
			LiveChatID:           liveChatID,
			NextPageToken:        nextPageToken,
			NextSequence:         nextSequence,
			UpdatedAt:            ingestedAt,
			LastFetchSucceededAt: ingestedAt,
		}
		if err := c.set(ctx, tx, streamRef, newStreamState); err != nil {
			return fmt.Errorf("write live chat stream state: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("ingest live chat page transaction: %w", err)
	}
	return nil
}

func validateAndDeduplicateLiveChatMessages(
	liveChatID string,
	messages []LiveChatHistoryDoc,
) ([]LiveChatHistoryDoc, error) {
	unique := make([]LiveChatHistoryDoc, 0, len(messages))
	seen := make(map[string]LiveChatHistoryDoc, len(messages))
	for i, message := range messages {
		if strings.TrimSpace(message.ID) == "" {
			return nil, fmt.Errorf("live chat message at index %d has empty id", i)
		}
		if message.LiveChatID != liveChatID {
			return nil, fmt.Errorf(
				"live chat message %q belongs to %q, expected %q",
				message.ID,
				message.LiveChatID,
				liveChatID,
			)
		}
		key, err := LiveChatMessageKey(liveChatID, message.ID)
		if err != nil {
			return nil, err
		}
		if previous, ok := seen[key]; ok {
			if previous != message {
				return nil, fmt.Errorf("duplicate message id %q has conflicting payload", message.ID)
			}
			continue
		}
		seen[key] = message
		unique = append(unique, message)
	}
	return unique, nil
}

func (c *FirestoreControllerImplements) readLiveChatStreamState(
	tx *firestore.Transaction,
	ref *firestore.DocumentRef,
) (LiveChatStreamStateDoc, bool, error) {
	doc, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return LiveChatStreamStateDoc{}, false, nil
	}
	if err != nil {
		return LiveChatStreamStateDoc{}, false, fmt.Errorf("get live chat stream state: %w", err)
	}
	var state LiveChatStreamStateDoc
	if err := doc.DataTo(&state); err != nil {
		return LiveChatStreamStateDoc{}, false, fmt.Errorf("decode live chat stream state: %w", err)
	}
	return state, true, nil
}

func transactionDocumentExists(tx *firestore.Transaction, ref *firestore.DocumentRef) (bool, error) {
	_, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func liveChatInboxFromHistory(
	message LiveChatHistoryDoc,
	sequence int64,
	ingestedAt time.Time,
) LiveChatInboxDoc {
	return LiveChatInboxDoc{
		LiveChatID:            message.LiveChatID,
		MessageID:             message.ID,
		Sequence:              sequence,
		AuthorChannelID:       message.AuthorChannelID,
		AuthorDisplayName:     message.AuthorDisplayName,
		AuthorProfileImageURL: message.AuthorProfileImageURL,
		AuthorIsChatModerator: message.AuthorIsChatModerator,
		MessageText:           message.MessageText,
		Type:                  message.Type,
		PublishedAt:           message.PublishedAt,
		Status:                LiveChatInboxPending,
		AttemptCount:          0,
		LastError:             "",
		IngestedAt:            ingestedAt,
		ProcessedAt:           nil,
		LeaseOwner:            "",
		LeaseUntil:            nil,
	}
}
