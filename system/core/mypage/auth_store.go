package mypage

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreAuthStore struct{ Client *firestore.Client }

func validOpaque(id string) bool {
	if len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func (s *FirestoreAuthStore) transactionRef(id string) (*firestore.DocumentRef, error) {
	if !validOpaque(id) {
		return nil, apiError("OAUTH_TRANSACTION_REQUIRED")
	}
	return s.Client.Collection("oauth-transactions").Doc(id), nil
}

func (s *FirestoreAuthStore) readTransaction(tx *firestore.Transaction, ref *firestore.DocumentRef) (OAuthTransaction, error) {
	doc, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return OAuthTransaction{}, apiError("OAUTH_TRANSACTION_REQUIRED")
	}
	if err != nil {
		return OAuthTransaction{}, fmt.Errorf("read OAuth transaction: %w", err)
	}
	var value OAuthTransaction
	if err := doc.DataTo(&value); err != nil {
		return OAuthTransaction{}, fmt.Errorf("decode OAuth transaction: %w", err)
	}
	return value, nil
}

func (s *FirestoreAuthStore) Create(ctx context.Context, id, previous string, value OAuthTransaction, now time.Time) error {
	if transactionPurpose(value) != "login" || value.Support != nil {
		return apiError("SUPPORT_CHALLENGE_INVALID")
	}
	ref, err := s.transactionRef(id)
	if err != nil {
		return err
	}
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if validOpaque(previous) {
			oldRef, err := s.transactionRef(previous)
			if err != nil {
				return err
			}
			old, err := s.readTransaction(tx, oldRef)
			if err != nil && errorCode(err) != "OAUTH_TRANSACTION_REQUIRED" {
				return err
			}
			if err == nil && now.Before(old.ExpiresAt) && (old.Status == "pending" || old.Status == "channel_verified") {
				if err := tx.Update(oldRef, []firestore.Update{{Path: "status", Value: "failed"}}); err != nil {
					return fmt.Errorf("supersede OAuth transaction: %w", err)
				}
			}
		}
		if err := tx.Create(ref, value); err != nil {
			return fmt.Errorf("create OAuth transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	return nil
}

func (s *FirestoreAuthStore) change(ctx context.Context, id string, fn func(OAuthTransaction) (OAuthTransaction, error)) error {
	ref, err := s.transactionRef(id)
	if err != nil {
		return err
	}
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		old, err := s.readTransaction(tx, ref)
		if err != nil {
			return err
		}
		next, err := fn(old)
		if err != nil {
			return err
		}
		if err := tx.Set(ref, next); err != nil {
			return fmt.Errorf("update OAuth transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("change transaction: %w", err)
	}
	return nil
}

func (s *FirestoreAuthStore) Claim(ctx context.Context, id, stateHash string, now time.Time) error {
	return s.change(ctx, id, func(tx OAuthTransaction) (OAuthTransaction, error) {
		if err := checkTransaction(tx, now, "pending"); err != nil {
			return tx, err
		}
		if subtle.ConstantTimeCompare([]byte(tx.StateHash), []byte(stateHash)) != 1 {
			return tx, apiError("OAUTH_FAILED")
		}
		tx.Status = "processing"
		return tx, nil
	})
}

func (s *FirestoreAuthStore) Verify(ctx context.Context, id string, channel Channel, confirmation string, now time.Time) error {
	if !validOpaque(confirmation) || strings.Contains(channel.ID, "/") || channel.ID == "" || channel.DisplayName == "" {
		return apiError("CHANNEL_UNAVAILABLE")
	}
	return s.change(ctx, id, func(tx OAuthTransaction) (OAuthTransaction, error) {
		if err := checkTransaction(tx, now, "processing"); err != nil {
			return tx, err
		}
		tx.Status = "channel_verified"
		tx.Channel = channel
		tx.ConfirmationRef = confirmation
		tx.VerifiedAt = now
		return tx, nil
	})
}

func (s *FirestoreAuthStore) Fail(ctx context.Context, id string) error {
	return s.change(ctx, id, func(tx OAuthTransaction) (OAuthTransaction, error) {
		if tx.Status == "consumed" {
			return tx, apiError("OAUTH_TRANSACTION_CONSUMED")
		}
		tx.Status = "failed"
		return tx, nil
	})
}

func (s *FirestoreAuthStore) ReadVerified(ctx context.Context, id string, now time.Time) (OAuthTransaction, error) {
	ref, err := s.transactionRef(id)
	if err != nil {
		return OAuthTransaction{}, err
	}
	var result OAuthTransaction
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		value, err := s.readTransaction(tx, ref)
		if err != nil {
			return err
		}
		if err := checkTransaction(value, now, "channel_verified"); err != nil {
			return err
		}
		result = value
		return nil
	}, firestore.ReadOnly)
	if err != nil {
		return OAuthTransaction{}, fmt.Errorf("read verified transaction: %w", err)
	}
	return result, nil
}

func (s *FirestoreAuthStore) Consume(ctx context.Context, id, confirmation, checkpoint string, policy Policy, now time.Time) (Channel, error) {
	ref, err := s.transactionRef(id)
	if err != nil {
		return Channel{}, err
	}
	var channel Channel
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		value, err := s.readTransaction(tx, ref)
		if err != nil {
			return err
		}
		if err := checkTransaction(value, now, "channel_verified"); err != nil {
			return err
		}
		if transactionPurpose(value) != "login" || value.Support != nil {
			return apiError("SUPPORT_CHALLENGE_INVALID")
		}
		if subtle.ConstantTimeCompare([]byte(value.ConfirmationRef), []byte(confirmation)) != 1 {
			return apiError("OAUTH_TRANSACTION_CHANGED")
		}
		if value.PrivacyPolicyVersion != policy.Privacy || value.TermsVersion != policy.Terms {
			return apiError("POLICY_VERSION_OUTDATED")
		}
		if value.Channel.ID == "" || strings.Contains(value.Channel.ID, "/") {
			return apiError("CHANNEL_UNAVAILABLE")
		}
		if checkpoint == "" {
			return &accessGateError{cause: apiError("TEMPORARY_UNAVAILABLE")}
		}
		if err := s.transactionAccess(tx, value.Channel.ID, checkpoint); err != nil {
			return err
		}
		accountRef := s.Client.Collection("web-accounts").Doc(value.Channel.ID)
		doc, err := tx.Get(accountRef)
		account := WebAccount{CreatedAt: now}
		if err == nil {
			if err := doc.DataTo(&account); err != nil {
				return fmt.Errorf("decode WebAccount: %w", err)
			}
		} else if status.Code(err) != codes.NotFound {
			return fmt.Errorf("read WebAccount: %w", err)
		}
		account.PrivacyPolicyVersion = value.PrivacyPolicyVersion
		account.TermsVersion = value.TermsVersion
		account.PrivacyConsentedAt = value.PrivacyConsentedAt
		account.DisplayName = value.Channel.DisplayName
		account.Handle = value.Channel.Handle
		account.AvatarURL = value.Channel.AvatarURL
		account.MetadataFetchedAt = value.VerifiedAt
		account.UpdatedAt = now
		value.Status = "consumed"
		value.ConsumedAt = now
		if err := tx.Set(accountRef, account); err != nil {
			return fmt.Errorf("write WebAccount: %w", err)
		}
		if err := tx.Set(ref, value); err != nil {
			return fmt.Errorf("consume OAuth transaction: %w", err)
		}
		channel = value.Channel
		return nil
	})
	if err != nil {
		return Channel{}, fmt.Errorf("consume transaction: %w", err)
	}
	return channel, nil
}

func (s *FirestoreAuthStore) ReadAccount(ctx context.Context, uid string) (WebAccount, error) {
	if uid == "" || strings.Contains(uid, "/") {
		return WebAccount{}, apiError("AUTH_REQUIRED")
	}
	doc, err := s.Client.Collection("web-accounts").Doc(uid).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return WebAccount{}, apiError("WEB_ACCOUNT_REQUIRED")
	}
	if err != nil {
		return WebAccount{}, fmt.Errorf("read WebAccount: %w", err)
	}
	var value WebAccount
	if err := doc.DataTo(&value); err != nil {
		return WebAccount{}, fmt.Errorf("decode WebAccount: %w", err)
	}
	value.Revision = doc.UpdateTime
	return value, nil
}

func (s *FirestoreAuthStore) CompleteSession(ctx context.Context, uid, checkpoint string, policy Policy, now time.Time) error {
	if uid == "" || strings.Contains(uid, "/") {
		return apiError("AUTH_REQUIRED")
	}
	if checkpoint == "" {
		return &accessGateError{cause: apiError("TEMPORARY_UNAVAILABLE")}
	}
	ref := s.Client.Collection("web-accounts").Doc(uid)
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := s.transactionAccess(tx, uid, checkpoint); err != nil {
			return err
		}
		doc, err := tx.Get(ref)
		if status.Code(err) == codes.NotFound {
			return apiError("WEB_ACCOUNT_REQUIRED")
		}
		if err != nil {
			return fmt.Errorf("read WebAccount: %w", err)
		}
		var account WebAccount
		if err := doc.DataTo(&account); err != nil {
			return fmt.Errorf("decode WebAccount: %w", err)
		}
		if err := checkPolicy(account, policy); err != nil {
			return err
		}
		if account.FirstWebLoginAt != nil {
			return nil
		}
		if err := tx.Update(ref, []firestore.Update{{Path: "firstWebLoginAt", Value: now}, {Path: "updatedAt", Value: now}}); err != nil {
			return fmt.Errorf("complete session: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("complete session transaction: %w", err)
	}
	return nil
}
