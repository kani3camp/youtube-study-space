package mypage

import (
	"context"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
)

const googleOperatorChallenges = "support-operator-challenges"

// FirestoreGoogleOperatorChallengesは、外から渡すdemo専用アダプターである。
// クライアント・認証情報・認証フロー・TTLポリシー・コマンド初期化は作らない。
// 呼び出し元が信頼済みEmulatorクライアントを渡す。プロジェクトとの対応だけでは、
// 渡されたクライアントの通信処理を調査・証明しない。消費済みマーカーで、遅延Put再試行によるnonce復活を防ぐ。
// 実環境の保持・廃棄・運用権限は別の確認事項として残す。
type FirestoreGoogleOperatorChallenges struct {
	client    *firestore.Client
	projectID string
}

func NewFirestoreGoogleOperatorChallenges(client *firestore.Client, projectID string) (*FirestoreGoogleOperatorChallenges, error) {
	store := &FirestoreGoogleOperatorChallenges{client: client, projectID: projectID}
	if !store.ready() {
		return nil, ErrOperatorDenied
	}
	return store, nil
}

func (s *FirestoreGoogleOperatorChallenges) ready() bool {
	return s != nil && s.client != nil && firebaseProjectID.MatchString(s.projectID) && strings.HasPrefix(s.projectID, "demo-") &&
		strings.HasPrefix(s.client.Collection(googleOperatorChallenges).Path, "projects/"+s.projectID+"/databases/(default)/documents/")
}

func canonicalOperatorHash(value string) bool {
	return validOpaque(value) && strings.ToLower(value) == value
}

func validOperatorChallengeRecord(record GoogleOperatorChallengeRecord) bool {
	return canonicalOperatorHash(record.NonceHash) && canonicalOperatorHash(record.Binding) && !record.IssuedAt.IsZero() &&
		record.IssuedAt.Year() >= 1 && record.ExpiresAt.Year() <= 9999 && record.ExpiresAt.Sub(record.IssuedAt) == googleOperatorChallengeLifetime
}

func (s *FirestoreGoogleOperatorChallenges) Put(ctx context.Context, record GoogleOperatorChallengeRecord) error {
	if !s.ready() || ctx == nil || ctx.Err() != nil || !validOperatorChallengeRecord(record) {
		return ErrOperatorDenied
	}
	// 正確なUTC文字列でナノ秒を保持する。Firestore Timestampはマイクロ秒に丸められるため、
	// 一度限りの証拠の期限境界をずらさないようにする。
	_, err := s.client.Collection(googleOperatorChallenges).Doc(record.NonceHash).Create(ctx, map[string]any{
		"schemaVersion": int64(1), "nonceHash": record.NonceHash, "binding": record.Binding,
		"issuedAt": record.IssuedAt.UTC().Format(time.RFC3339Nano), "expiresAt": record.ExpiresAt.UTC().Format(time.RFC3339Nano), "consumed": false,
	})
	if err != nil || ctx.Err() != nil {
		return ErrOperatorDenied
	}
	return nil
}

func decodeOperatorChallenge(raw map[string]any) (GoogleOperatorChallengeRecord, bool) {
	if len(raw) != 6 || raw["schemaVersion"] != int64(1) || raw["consumed"] != false {
		return GoogleOperatorChallengeRecord{}, false
	}
	nonce, nonceOK := raw["nonceHash"].(string)
	binding, bindingOK := raw["binding"].(string)
	issued, issuedOK := raw["issuedAt"].(string)
	expires, expiresOK := raw["expiresAt"].(string)
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, issued)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, expires)
	record := GoogleOperatorChallengeRecord{NonceHash: nonce, Binding: binding, IssuedAt: issuedAt, ExpiresAt: expiresAt}
	return record, nonceOK && bindingOK && issuedOK && expiresOK && issuedErr == nil && expiresErr == nil &&
		issued == issuedAt.UTC().Format(time.RFC3339Nano) && expires == expiresAt.UTC().Format(time.RFC3339Nano) && validOperatorChallengeRecord(record)
}

func (s *FirestoreGoogleOperatorChallenges) Consume(ctx context.Context, nonceHash, binding string, clock func() time.Time) (GoogleOperatorChallengeRecord, error) {
	if !s.ready() || ctx == nil || ctx.Err() != nil || !canonicalOperatorHash(nonceHash) || !canonicalOperatorHash(binding) || clock == nil {
		return GoogleOperatorChallengeRecord{}, ErrOperatorDenied
	}
	ref := s.client.Collection(googleOperatorChallenges).Doc(nonceHash)
	var result GoogleOperatorChallengeRecord
	err := s.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
		if err != nil {
			return ErrOperatorDenied
		}
		record, valid := decodeOperatorChallenge(doc.Data())
		// 待機し得る読み取りの後で、再試行のたびに信頼済み時刻を読み直す。
		if !valid || record.NonceHash != nonceHash || record.Binding != binding || !googleOperatorChallengeLive(record, clock().UTC()) || ctx.Err() != nil {
			return ErrOperatorDenied
		}
		if err := tx.Set(ref, map[string]any{"schemaVersion": int64(1), "consumed": true}); err != nil {
			return ErrOperatorDenied
		}
		result = record
		return nil
	})
	// 取得が成功してACKだけが遅れた場合も、消費済みのままにする。
	// 期限切れ応答で許可せず、再試行のためにマーカーをリセットしない。
	if err != nil {
		return GoogleOperatorChallengeRecord{}, ErrOperatorDenied
	}
	now := clock().UTC()
	if ctx.Err() != nil || !googleOperatorChallengeLive(result, now) {
		return GoogleOperatorChallengeRecord{}, ErrOperatorDenied
	}
	return result, nil
}
