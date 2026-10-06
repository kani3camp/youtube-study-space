package mypage

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SaveMetadata only updates an existing exact document revision. No external
// API call occurs inside the retryable transaction and no account is recreated.
func (s *FirestoreAuthStore) SaveMetadata(ctx context.Context, uid string, expected WebAccount, channel Channel, policy Policy, now time.Time) (WebAccount, error) {
	if !youtubeChannelID.MatchString(uid) || channel.ID != uid || channel.DisplayName == "" || expected.Revision.IsZero() {
		return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	ref := s.Client.Collection("web-accounts").Doc(uid)
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
		if status.Code(err) == codes.NotFound {
			return apiError("WEB_ACCOUNT_REQUIRED")
		}
		if err != nil {
			return fmt.Errorf("read metadata account: %w", err)
		}
		var current WebAccount
		if err := doc.DataTo(&current); err != nil {
			return fmt.Errorf("decode metadata account: %w", err)
		}
		if err := checkMetadataAccount(current, policy); err != nil {
			return err
		}
		if !doc.UpdateTime.Equal(expected.Revision) {
			return apiError("TEMPORARY_UNAVAILABLE")
		}
		if err := tx.Update(ref, []firestore.Update{{Path: "displayName", Value: channel.DisplayName}, {Path: "handle", Value: channel.Handle}, {Path: "avatarUrl", Value: channel.AvatarURL}, {Path: "metadataFetchedAt", Value: now}, {Path: "updatedAt", Value: now}}); err != nil {
			return fmt.Errorf("update public metadata: %w", err)
		}
		return nil
	})
	if err != nil {
		return WebAccount{}, fmt.Errorf("save public metadata: %w", err)
	}
	refreshed, err := s.ReadAccount(ctx, uid)
	if err != nil {
		return WebAccount{}, err
	}
	if err := checkMetadataAccount(refreshed, policy); err != nil {
		return WebAccount{}, err
	}
	return refreshed, nil
}

// ExpireMetadata clears expired public metadata and its fetched timestamp,
// retaining consent and account identity. It is not account deletion or an
// operator privacy-request execution method.
func (s *FirestoreAuthStore) ExpireMetadata(ctx context.Context, uid string, expected WebAccount, policy Policy, now time.Time) (WebAccount, error) {
	cutoff := now.Add(-30 * 24 * time.Hour)
	return s.clearAccountMetadata(ctx, uid, expected, policy, &cutoff, now)
}

// ClearMissingMetadata is called only for a validated successful API response
// proving the channel absent, not transport/malformed/ambiguous failures.
func (s *FirestoreAuthStore) ClearMissingMetadata(ctx context.Context, uid string, expected WebAccount, policy Policy, now time.Time) (WebAccount, error) {
	return s.clearAccountMetadata(ctx, uid, expected, policy, nil, now)
}

func (s *FirestoreAuthStore) clearAccountMetadata(ctx context.Context, uid string, expected WebAccount, policy Policy, cutoff *time.Time, now time.Time) (WebAccount, error) {
	if !youtubeChannelID.MatchString(uid) || expected.Revision.IsZero() {
		return WebAccount{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	ref := s.Client.Collection("web-accounts").Doc(uid)
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
		if status.Code(err) == codes.NotFound {
			return apiError("WEB_ACCOUNT_REQUIRED")
		}
		if err != nil {
			return fmt.Errorf("read expired metadata: %w", err)
		}
		var current WebAccount
		if err := doc.DataTo(&current); err != nil {
			return fmt.Errorf("decode expired metadata: %w", err)
		}
		if err := checkMetadataAccount(current, policy); err != nil {
			return err
		}
		if !doc.UpdateTime.Equal(expected.Revision) || (cutoff != nil && (current.MetadataFetchedAt.IsZero() || current.MetadataFetchedAt.After(*cutoff))) {
			return apiError("TEMPORARY_UNAVAILABLE")
		}
		if err := tx.Update(ref, metadataClearUpdates(now)); err != nil {
			return fmt.Errorf("expire public metadata: %w", err)
		}
		return nil
	})
	if err != nil {
		return WebAccount{}, fmt.Errorf("expire metadata: %w", err)
	}
	account, err := s.ReadAccount(ctx, uid)
	if err != nil {
		return WebAccount{}, err
	}
	if err := checkMetadataAccount(account, policy); err != nil {
		return WebAccount{}, err
	}
	return account, nil
}

func metadataClearUpdates(now time.Time) []firestore.Update {
	return []firestore.Update{{Path: "displayName", Value: firestore.Delete}, {Path: "handle", Value: firestore.Delete}, {Path: "avatarUrl", Value: firestore.Delete}, {Path: "metadataFetchedAt", Value: firestore.Delete}, {Path: "updatedAt", Value: now.UTC()}}
}
