package mypage

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FirestoreSupportStoreは運用担当者・サーバー用ライブラリである。公開管理エンドポイントはない。
// プロジェクト・認証情報・本番操作の確認は呼び出し元が担う。
type FirestoreSupportStore struct {
	Client      *firestore.Client
	Environment string
}

type supportChallengeIndex struct {
	RequestRef  string `firestore:"requestRef"`
	Environment string `firestore:"environment"`
}

func (s *FirestoreSupportStore) recordRef(ref string) (*firestore.DocumentRef, error) {
	if !validSupportEnvironment(s.Environment) || !validOpaque(ref) {
		return nil, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	return s.Client.Collection("support-requests").Doc(ref), nil
}

func readSupportRecord(tx *firestore.Transaction, ref *firestore.DocumentRef) (SupportRequest, error) {
	doc, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound {
		return SupportRequest{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	if err != nil {
		return SupportRequest{}, fmt.Errorf("read support record: %w", err)
	}
	var value SupportRequest
	if err := doc.DataTo(&value); err != nil {
		return SupportRequest{}, fmt.Errorf("decode support record: %w", err)
	}
	return value, nil
}

// Createは一度限りの配送チャレンジを返す。呼び出し元はログに出さず、
// CI成果物・シェル履歴・公開IssueやPRにも載せない。
func (s *FirestoreSupportStore) Create(ctx context.Context, requestID, channel string, purpose SupportPurpose, acceptedAt, now time.Time) (requestRef, challenge string, err error) {
	ref, err := opaque()
	if err != nil {
		return "", "", err
	}
	challenge, err = opaque()
	if err != nil {
		return "", "", err
	}
	value, err := newSupportRecord(requestID, s.Environment, channel, purpose, digest(challenge), acceptedAt, now)
	if err != nil {
		return "", "", err
	}
	record, err := s.recordRef(ref)
	if err != nil {
		return "", "", err
	}
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := tx.Create(record, value); err != nil {
			return fmt.Errorf("create support request: %w", err)
		}
		if err := tx.Create(s.Client.Collection("support-challenges").Doc(value.ChallengeHash), supportChallengeIndex{RequestRef: ref, Environment: s.Environment}); err != nil {
			return fmt.Errorf("create support challenge: %w", err)
		}
		// 外部から渡す1つの申請IDは、正確に1つの受付を識別する。
		// チャレンジ再発行で別受付を作らず、SLAもリセットしない。
		if err := tx.Create(s.Client.Collection("support-request-ids").Doc(digest(s.Environment+":"+requestID)), supportChallengeIndex{RequestRef: ref, Environment: s.Environment}); err != nil {
			return fmt.Errorf("create support receipt index: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", "", fmt.Errorf("create support record: %w", err)
	}
	return ref, challenge, nil
}

func (s *FirestoreSupportStore) Resolve(ctx context.Context, challenge string, now time.Time) (SupportBinding, error) {
	if !validSupportEnvironment(s.Environment) || !validOpaque(challenge) {
		return SupportBinding{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	var binding SupportBinding
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(s.Client.Collection("support-challenges").Doc(digest(challenge)))
		if status.Code(err) == codes.NotFound {
			return apiError("SUPPORT_CHALLENGE_INVALID")
		}
		if err != nil {
			return fmt.Errorf("read support challenge: %w", err)
		}
		var index supportChallengeIndex
		if err := doc.DataTo(&index); err != nil {
			return fmt.Errorf("decode support challenge: %w", err)
		}
		if index.Environment != s.Environment {
			return apiError("SUPPORT_CHALLENGE_INVALID")
		}
		ref, err := s.recordRef(index.RequestRef)
		if err != nil {
			return err
		}
		value, err := readSupportRecord(tx, ref)
		if err != nil {
			return err
		}
		if err := checkSupport(value, s.Environment, digest(challenge), now); err != nil {
			return err
		}
		binding = SupportBinding{RequestRef: index.RequestRef, RequestID: value.RequestID, Environment: value.Environment, Purpose: value.Purpose, ChallengeHash: value.ChallengeHash}
		return nil
	}, firestore.ReadOnly)
	if err != nil {
		return SupportBinding{}, fmt.Errorf("resolve support challenge: %w", err)
	}
	return binding, nil
}

func (s *FirestoreSupportStore) Reissue(ctx context.Context, requestRef string, now time.Time) (string, error) {
	ref, err := s.recordRef(requestRef)
	if err != nil {
		return "", err
	}
	challenge, err := opaque()
	if err != nil {
		return "", err
	}
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		value, err := readSupportRecord(tx, ref)
		if err != nil {
			return err
		}
		return stageSupportReissue(tx, ref, value, s.Environment, challenge, now, s.Client)
	})
	if err != nil {
		return "", fmt.Errorf("reissue support challenge: %w", err)
	}
	return challenge, nil
}

func checkSupportBinding(value SupportRequest, binding SupportBinding, environment string, now time.Time) error {
	if binding.Environment != environment || binding.RequestID != value.RequestID || binding.Purpose != value.Purpose {
		return apiError("SUPPORT_CHALLENGE_INVALID")
	}
	return checkSupport(value, environment, binding.ChallengeHash, now)
}

func (s *FirestoreSupportStore) CheckBinding(ctx context.Context, binding SupportBinding, channel string, now time.Time) error {
	ref, err := s.recordRef(binding.RequestRef)
	if err != nil {
		return err
	}
	err = s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		value, err := readSupportRecord(tx, ref)
		if err != nil {
			return err
		}
		if err := checkSupportBinding(value, binding, s.Environment, now); err != nil {
			return err
		}
		if value.TargetChannel != channel {
			return apiError("SUPPORT_CHANNEL_MISMATCH")
		}
		return nil
	}, firestore.ReadOnly)
	if err != nil {
		return fmt.Errorf("check bound support request: %w", err)
	}
	return nil
}

func stageSupportReissue(tx *firestore.Transaction, ref *firestore.DocumentRef, value SupportRequest, environment, challenge string, now time.Time, client *firestore.Client) error {
	next, err := reissueSupportRecord(value, environment, digest(challenge), now)
	if err != nil {
		return err
	}
	if !validOpaque(value.ChallengeHash) {
		return apiError("SUPPORT_CHALLENGE_INVALID")
	}
	if err := tx.Delete(client.Collection("support-challenges").Doc(value.ChallengeHash)); err != nil {
		return fmt.Errorf("remove previous support challenge: %w", err)
	}
	if err := tx.Create(client.Collection("support-challenges").Doc(next.ChallengeHash), supportChallengeIndex{RequestRef: ref.ID, Environment: environment}); err != nil {
		return fmt.Errorf("create replacement support challenge: %w", err)
	}
	if err := tx.Set(ref, next); err != nil {
		return fmt.Errorf("reissue support record: %w", err)
	}
	return nil
}
