package mypage

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

func (s *FirestoreAuthStore) CreateSupport(ctx context.Context, id, previous string, value OAuthTransaction, environment string, now time.Time) error {
	if value.Purpose != "support" || value.Support == nil || !validSupportEnvironment(environment) || !validOpaque(value.Support.RequestRef) {
		return apiError("SUPPORT_CHALLENGE_INVALID")
	}
	ref, err := s.transactionRef(id)
	if err != nil {
		return err
	}
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		request, err := readSupportRecord(tx, s.Client.Collection("support-requests").Doc(value.Support.RequestRef))
		if err != nil {
			return err
		}
		if err := checkSupportBinding(request, *value.Support, environment, now); err != nil {
			return err
		}
		var previousRef *firestore.DocumentRef
		if validOpaque(previous) {
			previousRef, err = s.transactionRef(previous)
			if err != nil {
				return err
			}
			old, err := s.readTransaction(tx, previousRef)
			if err != nil && errorCode(err) != "OAUTH_TRANSACTION_REQUIRED" {
				return err
			}
			if err != nil || !now.Before(old.ExpiresAt) || (old.Status != "pending" && old.Status != "channel_verified") {
				previousRef = nil
			}
		}
		if previousRef != nil {
			if err := tx.Update(previousRef, []firestore.Update{{Path: "status", Value: "failed"}}); err != nil {
				return fmt.Errorf("supersede support OAuth: %w", err)
			}
		}
		if err := tx.Create(ref, value); err != nil {
			return fmt.Errorf("create bound support OAuth: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("start bound support OAuth: %w", err)
	}
	return nil
}

func (s *FirestoreAuthStore) ConsumeSupport(ctx context.Context, id, confirmation string, policy Policy, environment, proofRef string, now time.Time) error {
	ref, err := s.transactionRef(id)
	if err != nil {
		return err
	}
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		value, err := s.readTransaction(tx, ref)
		if err != nil {
			return err
		}
		if err := checkTransaction(value, now, "channel_verified"); err != nil {
			return err
		}
		if value.Purpose != "support" || value.Support == nil || !validOpaque(value.Support.RequestRef) || value.Support.Environment != environment {
			return apiError("SUPPORT_CHALLENGE_INVALID")
		}
		if subtle.ConstantTimeCompare([]byte(value.ConfirmationRef), []byte(confirmation)) != 1 {
			return apiError("OAUTH_TRANSACTION_CHANGED")
		}
		if value.PrivacyPolicyVersion != policy.Privacy || value.TermsVersion != policy.Terms {
			return apiError("POLICY_VERSION_OUTDATED")
		}
		requestRef := s.Client.Collection("support-requests").Doc(value.Support.RequestRef)
		request, err := readSupportRecord(tx, requestRef)
		if err != nil {
			return err
		}
		next, err := verifySupportRecord(request, *value.Support, value.Channel.ID, id, proofRef, now)
		if err != nil {
			return err
		}
		if err := tx.Delete(s.Client.Collection("support-challenges").Doc(request.ChallengeHash)); err != nil {
			return fmt.Errorf("consume support challenge: %w", err)
		}
		if err := tx.Set(requestRef, next); err != nil {
			return fmt.Errorf("record fresh support proof: %w", err)
		}
		value.Status = "consumed"
		value.ConsumedAt = now
		// The support record keeps the verified target until the operator closes
		// the request. OAuth channel metadata is no longer needed after proof.
		value.Channel = Channel{}
		value.ConfirmationRef = ""
		value.StateHash = ""
		if err := tx.Set(ref, value); err != nil {
			return fmt.Errorf("consume support OAuth: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("consume bound support proof: %w", err)
	}
	return nil
}
