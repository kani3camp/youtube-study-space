package mypage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
)

// SupportRecoverySelectorは元の有人受付を表し、権限や本人証明には使わない。
// 復旧の呼び出し元は、この正確な対応を独立して承認する必要がある。
type SupportRecoverySelector struct {
	Environment, ProjectID, RequestID, Channel string
	Purpose                                    SupportPurpose
	AcceptedAt                                 time.Time
}

// SupportReceiptInspectionは本文・連絡先・証拠・チャレンジとそのハッシュを含めない。
// RevisionはFirestore UpdateTimeとする。
type SupportReceiptInspection struct {
	RequestRef string
	Revision   time.Time
	DeleteBy   *time.Time
}

func (s *FirestoreSupportStore) recoveryReady(ctx context.Context, selector SupportRecoverySelector, now time.Time) bool {
	return ctx != nil && ctx.Err() == nil && s != nil && s.Client != nil && validSupportEnvironment(s.Environment) && selector.Environment == s.Environment &&
		firebaseProjectID.MatchString(selector.ProjectID) && validRequestID(selector.RequestID) && youtubeChannelID.MatchString(selector.Channel) && validSupportPurpose(selector.Purpose) &&
		!selector.AcceptedAt.IsZero() && !now.IsZero() && !selector.AcceptedAt.After(now) &&
		strings.HasPrefix(s.Client.Collection("support-requests").Path, "projects/"+selector.ProjectID+"/databases/(default)/documents/")
}

func (s *FirestoreSupportStore) readRecovery(tx *firestore.Transaction, selector SupportRecoverySelector, now time.Time) (*firestore.DocumentSnapshot, SupportRequest, error) {
	deny := func() (*firestore.DocumentSnapshot, SupportRequest, error) {
		return nil, SupportRequest{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	index, err := tx.Get(s.Client.Collection("support-request-ids").Doc(digest(s.Environment + ":" + selector.RequestID)))
	if err != nil {
		return nil, SupportRequest{}, fmt.Errorf("read receipt recovery index: %w", err)
	}
	var binding supportChallengeIndex
	if len(index.Data()) != 2 || index.DataTo(&binding) != nil || binding.Environment != s.Environment || !validOpaque(binding.RequestRef) {
		return deny()
	}
	ref := s.Client.Collection("support-requests").Doc(binding.RequestRef)
	doc, err := tx.Get(ref)
	if err != nil {
		return nil, SupportRequest{}, fmt.Errorf("read receipt for recovery: %w", err)
	}
	var value SupportRequest
	if doc.DataTo(&value) != nil || doc.UpdateTime.IsZero() || value.Environment != selector.Environment || value.RequestID != selector.RequestID || value.TargetChannel != selector.Channel || value.Purpose != selector.Purpose || !value.AcceptedAt.Equal(selector.AcceptedAt) || value.Status != "awaiting_proof" || value.VerifiedAt != nil || value.CompletedAt != nil || value.ProofRef != "" || value.OAuthTransactionID != "" || !validOpaque(value.ChallengeHash) || value.ChallengeExpiresAt.IsZero() {
		return deny()
	}
	if value.Purpose == SupportDelete {
		if value.DeleteBy == nil || !value.DeleteBy.Equal(value.AcceptedAt.Add(7*24*time.Hour)) {
			return deny()
		}
	} else if value.DeleteBy != nil {
		return deny()
	}
	// 旧チャレンジの所有者は、受付と同じ読み取り集合内で一致する必要がある。
	old, err := tx.Get(s.Client.Collection("support-challenges").Doc(value.ChallengeHash))
	if err != nil {
		return nil, SupportRequest{}, fmt.Errorf("read recovery challenge index: %w", err)
	}
	var challenge supportChallengeIndex
	if len(old.Data()) != 2 || old.DataTo(&challenge) != nil || challenge.Environment != s.Environment || challenge.RequestRef != ref.ID {
		return deny()
	}
	if _, err := reissueSupportRecord(value, s.Environment, value.ChallengeHash, now); err != nil {
		return nil, SupportRequest{}, err
	}
	return doc, value, nil
}

// InspectRequestIDは読み取り専用。インデックスの欠落・不一致は不明のまま残す。
// 失敗を、Create再呼び出しの許可として扱わない。
func (s *FirestoreSupportStore) InspectRequestID(ctx context.Context, selector SupportRecoverySelector, now time.Time) (SupportReceiptInspection, error) {
	if !s.recoveryReady(ctx, selector, now) {
		return SupportReceiptInspection{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	var result SupportReceiptInspection
	err := s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		doc, value, err := s.readRecovery(tx, selector, now)
		if err != nil {
			return err
		}
		result = SupportReceiptInspection{RequestRef: doc.Ref.ID, Revision: doc.UpdateTime, DeleteBy: value.DeleteBy}
		return nil
	}, firestore.ReadOnly)
	if err != nil || ctx.Err() != nil {
		return SupportReceiptInspection{}, fmt.Errorf("inspect receipt recovery: %w", errors.Join(err, ctx.Err()))
	}
	return result, nil
}

// ReissueRequestIDは明示的にレビューした調査バージョンを必要とする。
// コミットやACK消失後、または並行する本人確認・再発行後に同じバージョンを使うと失敗する必要がある。
// 操作記録・再試行ループ・配送・Createへの切替は追加しない。
func (s *FirestoreSupportStore) ReissueRequestID(ctx context.Context, selector SupportRecoverySelector, expected time.Time, now time.Time) (string, string, error) {
	if !s.recoveryReady(ctx, selector, now) || expected.IsZero() {
		return "", "", apiError("SUPPORT_CHALLENGE_INVALID")
	}
	challenge, err := opaque()
	if err != nil {
		return "", "", err
	}
	var ref string
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		doc, value, err := s.readRecovery(tx, selector, now)
		if err != nil {
			return err
		}
		if !doc.UpdateTime.Equal(expected) {
			return apiError("SUPPORT_CHALLENGE_INVALID")
		}
		if err := stageSupportReissue(tx, doc.Ref, value, s.Environment, challenge, now, s.Client); err != nil {
			return err
		}
		ref = doc.Ref.ID
		return nil
	})
	if err != nil || ctx.Err() != nil {
		return "", "", fmt.Errorf("recover support receipt: %w", errors.Join(err, ctx.Err()))
	}
	return ref, challenge, nil
}
