package mypage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Only the caller's explicitly selected existing project may be inspected.
// No credentials, scheduler, IAM or API/provider is created by this adapter.
type FirestoreMetadataCleanupStore struct {
	Client    *firestore.Client
	ProjectID string
}

func (s *FirestoreMetadataCleanupStore) valid() bool {
	return s.Client != nil && s.ProjectID != "" && !strings.ContainsAny(s.ProjectID, "/\\ \r\n\t") && strings.HasPrefix(s.Client.Collection("web-accounts").Doc("probe").Path, "projects/"+s.ProjectID+"/databases/(default)/documents/")
}

func (s *FirestoreMetadataCleanupStore) Scan(ctx context.Context, cutoff time.Time, after *MetadataCleanupCursor, limit int) (MetadataCleanupPage, error) {
	if !s.valid() || cutoff.IsZero() || limit < 1 || limit > 1000 {
		return MetadataCleanupPage{}, ErrMetadataCleanupIncomplete
	}
	query := s.Client.Collection("web-accounts").Where("metadataFetchedAt", "<=", cutoff).OrderBy("metadataFetchedAt", firestore.Asc).OrderBy(firestore.DocumentID, firestore.Asc).Select("metadataFetchedAt").Limit(limit + 1)
	if after != nil {
		if !youtubeChannelID.MatchString(after.UID) || after.FetchedAt.IsZero() {
			return MetadataCleanupPage{}, ErrMetadataCleanupIncomplete
		}
		query = query.StartAfter(after.FetchedAt, s.Client.Collection("web-accounts").Doc(after.UID))
	}
	docs, err := query.Documents(ctx).GetAll()
	if err != nil {
		return MetadataCleanupPage{}, fmt.Errorf("read metadata cleanup page: %w", err)
	}
	more := len(docs) > limit
	if more {
		docs = docs[:limit]
	}
	page := MetadataCleanupPage{More: more, Candidates: make([]MetadataCleanupCandidate, 0, len(docs))}
	for _, doc := range docs {
		value, err := doc.DataAt("metadataFetchedAt")
		if err != nil {
			return MetadataCleanupPage{}, fmt.Errorf("read metadata timestamp: %w", err)
		}
		fetched, ok := value.(time.Time)
		if !ok || fetched.IsZero() || fetched.After(cutoff) || !youtubeChannelID.MatchString(doc.Ref.ID) || doc.UpdateTime.IsZero() {
			return MetadataCleanupPage{}, ErrMetadataCleanupIncomplete
		}
		page.Candidates = append(page.Candidates, MetadataCleanupCandidate{UID: doc.Ref.ID, FetchedAt: fetched, Revision: doc.UpdateTime})
	}
	return page, nil
}

func (s *FirestoreMetadataCleanupStore) Clear(ctx context.Context, candidate MetadataCleanupCandidate, cutoff, now time.Time) (string, error) {
	if !s.valid() || !youtubeChannelID.MatchString(candidate.UID) || candidate.FetchedAt.IsZero() || candidate.Revision.IsZero() || cutoff.IsZero() || now.IsZero() || candidate.FetchedAt.After(cutoff) || cutoff.After(now.Add(-MetadataCleanupAge)) {
		return "", ErrMetadataCleanupIncomplete
	}
	ref := s.Client.Collection("web-accounts").Doc(candidate.UID)
	outcome := ""
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
		if status.Code(err) == codes.NotFound {
			outcome = "missing"
			return nil
		}
		if err != nil {
			return fmt.Errorf("read cleanup account: %w", err)
		}
		if !doc.UpdateTime.Equal(candidate.Revision) {
			outcome = "changed"
			return nil
		}
		raw, err := doc.DataAt("metadataFetchedAt")
		if err != nil {
			return fmt.Errorf("read cleanup account timestamp: %w", err)
		}
		fetched, ok := raw.(time.Time)
		if !ok || !fetched.Equal(candidate.FetchedAt) || fetched.After(cutoff) {
			outcome = "changed"
			return nil
		}
		// Retention applies independently of login/policy/access state. Preserve all
		// identity, consent, lifecycle and blocked fields; never recreate a document.
		if err := tx.Update(ref, metadataClearUpdates(now)); err != nil {
			return fmt.Errorf("clear expired metadata: %w", err)
		}
		outcome = "cleared"
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("commit metadata cleanup: %w", err)
	}
	return outcome, nil
}
