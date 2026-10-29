package mypage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const supportOperatorAudit = "support-operator-audit"

var (
	ErrOperatorDenied   = errors.New("trusted operator unavailable or denied")
	ErrOperatorConflict = errors.New("support operation conflict")
)

// OperatorAuthorityは、信頼済み運用環境から本人情報と権限を取得する必要がある。
// CLI指定の名前・申請参照・サポートチャレンジは権限にならない。
// 本番用の実装は意図的に含めない。
type OperatorAuthority interface {
	Authorize(context.Context, OperatorIntent) (OperatorIdentity, error)
	VerifyEvidence(context.Context, OperatorIdentity, CompletionOperation) (VerifiedCompletionEvidence, error)
}

type OperatorIntent struct {
	Environment string
	ProjectID   string
	RequestRef  string
	Purpose     SupportPurpose
	OperationID string
	Action      string
	// これらのフィールドは、権限確認前にFirestoreSupportOperatorが操作全体から導出する。
	// 呼び出し元の操作意図で、証拠・リビジョン・返信との対応・操作内容を置き換えられない。
	ProofRef         string
	ExpectedRevision int64
	ReplyDigest      string
	OperationDigest  string
}

type OperatorIdentity struct {
	Subject     string
	Environment string
	ProjectID   string
}

type ReplyOperation struct {
	Intent           OperatorIntent
	ProofRef         string
	ExpectedRevision int64
	Body             string
	At               time.Time
}

type CompletionOperation struct {
	Intent                  OperatorIntent
	ProofRef                string
	ReplyOperationID        string
	ExpectedRevision        int64
	ActionEvidence          string
	DeliveryAcknowledgement string
	// 将来の有人OIDC権限検証に必要である。既存の合成証跡アダプターも、
	// 保存済み返信に対してダイジェストを独立して検証する。
	ReplyDigest string
	At          time.Time
}

// VerifiedCompletionEvidenceは、信頼済み権限検証で両証跡参照と確認済み返信との対応を確認した後だけ返す。
// 運用担当者の申告だけでは、これらのフィールドを証明できない。
type VerifiedCompletionEvidence struct {
	Actor                   string
	Environment             string
	ProjectID               string
	RequestRef              string
	Purpose                 SupportPurpose
	OperationID             string
	ProofRef                string
	ReplyOperationID        string
	ExpectedRevision        int64
	ReplyDigest             string
	ActionEvidence          string
	DeliveryAcknowledgement string
}

func (e VerifiedCompletionEvidence) matches(identity OperatorIdentity, op CompletionOperation) bool {
	return e.Actor == identity.Subject && e.Environment == op.Intent.Environment && e.ProjectID == op.Intent.ProjectID &&
		e.RequestRef == op.Intent.RequestRef && e.Purpose == op.Intent.Purpose && e.OperationID == op.Intent.OperationID &&
		e.ProofRef == op.ProofRef && e.ReplyOperationID == op.ReplyOperationID && e.ExpectedRevision == op.ExpectedRevision &&
		validOpaque(e.ReplyDigest) && (op.ReplyDigest == "" || e.ReplyDigest == op.ReplyDigest) && e.ActionEvidence == op.ActionEvidence && e.DeliveryAcknowledgement == op.DeliveryAcknowledgement
}

// FirestoreSupportOperatorは公開HTTPハンドラーから構築しない。
// 信頼済み権限検証とサーバー専用監査鍵が注入されなければ、すべての変更を拒否する。
// プロジェクトはFirestoreクライアントのパスと照合する。
type FirestoreSupportOperator struct {
	Client      *firestore.Client
	Environment string
	ProjectID   string
	AuditKey    []byte
	Authority   OperatorAuthority
	// 将来の有人権限検証は、この信頼済みサーバー時計を使う必要がある。
	// 呼び出し元のAtは既存の合成・オフラインテストデータ用だけに残す。
	Clock func() time.Time
}

func (s *FirestoreSupportOperator) operationTime(caller time.Time) (time.Time, error) {
	if s == nil {
		return time.Time{}, ErrOperatorDenied
	}
	if s.Clock != nil {
		now := s.Clock().UTC()
		if now.IsZero() {
			return time.Time{}, ErrOperatorDenied
		}
		return now, nil
	}
	if _, human := s.Authority.(*GoogleHumanOperatorAuthority); human {
		return time.Time{}, ErrOperatorDenied
	}
	return caller, nil
}

type operatorAuditEvent struct {
	SchemaVersion int64          `firestore:"schemaVersion"`
	Environment   string         `firestore:"environment"`
	Purpose       SupportPurpose `firestore:"purpose"`
	Action        string         `firestore:"action"`
	Actor         string         `firestore:"actor"`
	Binding       string         `firestore:"binding"`
	Fingerprint   string         `firestore:"fingerprint"`
	At            time.Time      `firestore:"at"`
}

func (s *FirestoreSupportOperator) mac(parts ...string) string {
	h := hmac.New(sha256.New, s.AuditKey)
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ReplyAuthorizationIntentは、確認済み本文をチャレンジや監査へ公開せず結び付ける。
// Replyは同じ導出済み意図をAuthorizeへ渡す。
func (s *FirestoreSupportOperator) ReplyAuthorizationIntent(op ReplyOperation) (OperatorIntent, error) {
	if s == nil || len(s.AuditKey) < 32 || !validOpaque(op.ProofRef) || op.ExpectedRevision < 0 || op.At.IsZero() || op.Body == "" || !validPrivacyBody(op.Body) {
		return OperatorIntent{}, ErrOperatorDenied
	}
	intent := op.Intent
	intent.ProofRef = op.ProofRef
	intent.ExpectedRevision = op.ExpectedRevision
	intent.ReplyDigest = s.mac("reply", intent.Environment, intent.ProjectID, intent.RequestRef, string(intent.Purpose), op.ProofRef, intent.OperationID, fmt.Sprint(op.ExpectedRevision), op.Body)
	intent.OperationDigest = intent.ReplyDigest
	return intent, nil
}

// CompletionAuthorizationIntentは、申告された操作・配送参照と確認済み返信のダイジェストを結び付ける。
// 実際の完了には独立した証跡と受付トランザクションが必要であり、この意図は書き込み権限を付与しない。
func (s *FirestoreSupportOperator) CompletionAuthorizationIntent(op CompletionOperation) (OperatorIntent, error) {
	if s == nil || len(s.AuditKey) < 32 || op.Intent.Purpose == SupportDelete || !validOpaque(op.ProofRef) || !validOpaque(op.ReplyOperationID) || !validOpaque(op.ActionEvidence) || !validOpaque(op.DeliveryAcknowledgement) || op.ExpectedRevision < 1 || op.At.IsZero() || (op.ReplyDigest != "" && !validOpaque(op.ReplyDigest)) {
		return OperatorIntent{}, ErrOperatorDenied
	}
	intent := op.Intent
	intent.ProofRef = op.ProofRef
	intent.ExpectedRevision = op.ExpectedRevision
	intent.ReplyDigest = op.ReplyDigest
	parts := []string{"complete", intent.Environment, intent.ProjectID, intent.RequestRef, string(intent.Purpose), op.ProofRef, intent.OperationID, op.ReplyOperationID, fmt.Sprint(op.ExpectedRevision), op.ActionEvidence, op.DeliveryAcknowledgement}
	// 既存操作の旧フィンガープリントは変更しない。
	// 指定された返信ダイジェストは、新しい有人承認の全再試行の対応情報に含める。
	if op.ReplyDigest != "" {
		parts = append(parts, op.ReplyDigest)
	}
	intent.OperationDigest = s.mac(parts...)
	return intent, nil
}

func (s *FirestoreSupportOperator) authorize(ctx context.Context, intent OperatorIntent, action string) (OperatorIdentity, error) {
	if s == nil || s.Client == nil || s.Authority == nil || len(s.AuditKey) < 32 || !validSupportEnvironment(s.Environment) || s.ProjectID == "" ||
		!strings.HasPrefix(s.Client.Collection("support-requests").Path, "projects/"+s.ProjectID+"/databases/(default)/documents/") ||
		intent.Environment != s.Environment || intent.ProjectID != s.ProjectID || intent.Action != action || !validOpaque(intent.RequestRef) || !validOpaque(intent.OperationID) || !validSupportPurpose(intent.Purpose) {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	identity, err := s.Authority.Authorize(ctx, intent)
	if err != nil || identity.Environment != s.Environment || identity.ProjectID != s.ProjectID || len(identity.Subject) < 3 || len(identity.Subject) > 128 || strings.ContainsAny(identity.Subject, "/\\\n\r\t") {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	return identity, nil
}

func (s *FirestoreSupportOperator) audit(intent OperatorIntent, actor, fingerprint string, at time.Time) operatorAuditEvent {
	return operatorAuditEvent{
		SchemaVersion: 1, Environment: s.Environment, Purpose: intent.Purpose, Action: intent.Action, Actor: actor,
		Binding: s.mac("request", intent.RequestRef), Fingerprint: fingerprint, At: at.UTC().Truncate(time.Microsecond),
	}
}

func (s *FirestoreSupportOperator) auditRef(intent OperatorIntent) *firestore.DocumentRef {
	return s.Client.Collection(supportOperatorAudit).Doc(s.mac("operation", intent.OperationID))
}

func (s *FirestoreSupportOperator) validRecord(v SupportRequest, op OperatorIntent, proof string, at time.Time) bool {
	return v.Environment == s.Environment && v.Purpose == op.Purpose && v.Status == "verified" && v.VerifiedAt != nil && !at.Before(*v.VerifiedAt) &&
		validOpaque(v.ProofRef) && v.ProofRef == proof && validOpaque(v.OAuthTransactionID) && v.TargetChannel != "" && v.CompletedAt == nil
}

func (s *FirestoreSupportOperator) deletionClaimed(tx *firestore.Transaction, proof string) (bool, error) {
	_, err := tx.Get(s.Client.Collection(supportDeleteClaims).Doc(proof))
	if err == nil {
		return true, nil
	}
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	return false, fmt.Errorf("read deletion claim: %w", err)
}

// Replyは確認済み返信1件と、本文を含まない上限付き監査イベントを同じトランザクションで書き込む。
// 依頼された操作を完了状態にはしない。
func (s *FirestoreSupportOperator) Reply(ctx context.Context, op ReplyOperation) error {
	trustedAt, err := s.operationTime(op.At)
	if err != nil {
		return err
	}
	op.At = trustedAt
	intent, err := s.ReplyAuthorizationIntent(op)
	if err != nil {
		return err
	}
	identity, err := s.authorize(ctx, intent, "reply")
	if err != nil {
		return err
	}
	fingerprint := intent.ReplyDigest
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		ref := s.Client.Collection("support-requests").Doc(op.Intent.RequestRef)
		v, err := readSupportRecord(tx, ref)
		if err != nil {
			if errorCode(err) == "SUPPORT_CHALLENGE_INVALID" {
				return ErrOperatorConflict
			}
			return err
		}
		if !s.validRecord(v, op.Intent, op.ProofRef, op.At) {
			return ErrOperatorConflict
		}
		claimed, err := s.deletionClaimed(tx, op.ProofRef)
		if err != nil {
			return err
		}
		if claimed {
			return ErrOperatorConflict
		}
		auditRef := s.auditRef(op.Intent)
		auditSnap, err := tx.Get(auditRef)
		if err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("read reply audit: %w", err)
		}
		if err == nil {
			var event operatorAuditEvent
			if auditSnap.DataTo(&event) != nil || event.Fingerprint != fingerprint || event.Actor != identity.Subject || v.ReplyOperationID != op.Intent.OperationID || v.ReplyDigest != fingerprint || v.OperatorRevision != op.ExpectedRevision+1 {
				return ErrOperatorConflict
			}
			return nil
		}
		if v.OperatorRevision != op.ExpectedRevision || v.ReplyOperationID != "" || v.OperatorReply != "" {
			return ErrOperatorConflict
		}
		if err := tx.Update(ref, []firestore.Update{{Path: "operatorReply", Value: op.Body}, {Path: "operatorReplyAt", Value: op.At.UTC().Truncate(time.Microsecond)}, {Path: "operatorRevision", Value: op.ExpectedRevision + 1}, {Path: "replyOperationId", Value: op.Intent.OperationID}, {Path: "replyDigest", Value: fingerprint}}); err != nil {
			return fmt.Errorf("write reply: %w", err)
		}
		if err := tx.Create(auditRef, s.audit(op.Intent, identity.Subject, fingerprint, op.At)); err != nil {
			return fmt.Errorf("write reply audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("operator reply: %w", err)
	}
	return nil
}

// Completeは開示とFirebaseセッション失効だけを対象にする。証跡検証は信頼済みアダプターに委ねる。
// 文字列は参照であり、自己申告の証拠ではない。削除には独立した保護付きFinalize手順を使う。
func (s *FirestoreSupportOperator) Complete(ctx context.Context, op CompletionOperation) error {
	trustedAt, err := s.operationTime(op.At)
	if err != nil {
		return err
	}
	op.At = trustedAt
	intent, err := s.CompletionAuthorizationIntent(op)
	if err != nil {
		return err
	}
	identity, err := s.authorize(ctx, intent, "complete")
	if err != nil {
		return err
	}
	fingerprint := intent.OperationDigest
	auditRef := s.auditRef(op.Intent)
	// ACK消失からの復旧には、コミット済み監査と完了受付で足りる。
	// 証跡は消去済み、または検証器が利用不能の場合がある。
	auditSnap, err := auditRef.Get(ctx)
	if err == nil {
		var event operatorAuditEvent
		if auditSnap.DataTo(&event) != nil || !s.sameCompletionAudit(event, op, identity, fingerprint) {
			return ErrOperatorConflict
		}
		snap, err := s.Client.Collection("support-requests").Doc(op.Intent.RequestRef).Get(ctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrOperatorConflict
			}
			return fmt.Errorf("read completion receipt: %w", err)
		}
		if !simpleOperatorTerminal(snap) || snap.Data()["purpose"] != string(op.Intent.Purpose) {
			return ErrOperatorConflict
		}
		return nil
	}
	if status.Code(err) != codes.NotFound {
		return fmt.Errorf("read completion audit: %w", err)
	}
	verified, err := s.Authority.VerifyEvidence(ctx, identity, op)
	if err != nil || !verified.matches(identity, op) {
		return ErrOperatorDenied
	}
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		ref := s.Client.Collection("support-requests").Doc(op.Intent.RequestRef)
		snap, err := tx.Get(ref)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ErrOperatorConflict
			}
			return fmt.Errorf("read completion receipt: %w", err)
		}
		auditSnap, err := tx.Get(auditRef)
		if err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("read completion audit: %w", err)
		}
		if err == nil {
			var event operatorAuditEvent
			if auditSnap.DataTo(&event) != nil || !s.sameCompletionAudit(event, op, identity, fingerprint) || !simpleOperatorTerminal(snap) || snap.Data()["purpose"] != string(op.Intent.Purpose) {
				return ErrOperatorConflict
			}
			return nil
		}
		var v SupportRequest
		if snap.DataTo(&v) != nil || !s.validRecord(v, op.Intent, op.ProofRef, op.At) || v.OperatorRevision != op.ExpectedRevision || v.ReplyOperationID != op.ReplyOperationID || v.ReplyDigest != verified.ReplyDigest || v.OperatorReply == "" || v.OperatorReplyAt == nil {
			return ErrOperatorConflict
		}
		claimed, err := s.deletionClaimed(tx, op.ProofRef)
		if err != nil {
			return err
		}
		if claimed {
			return ErrOperatorConflict
		}
		// 置換で、本文・返信・チャンネル・証拠・全受付フィールドを除去する。
		terminal := map[string]interface{}{"schemaVersion": int64(1), "completedAt": op.At.UTC().Truncate(time.Microsecond), "purpose": v.Purpose, "acceptedAt": v.AcceptedAt}
		if err := tx.Delete(s.Client.Collection("support-request-ids").Doc(digest(v.Environment + ":" + v.RequestID))); err != nil {
			return fmt.Errorf("delete request index: %w", err)
		}
		if err := tx.Delete(s.Client.Collection("oauth-transactions").Doc(v.OAuthTransactionID)); err != nil {
			return fmt.Errorf("delete support OAuth: %w", err)
		}
		if err := tx.Set(ref, terminal); err != nil {
			return fmt.Errorf("write terminal request: %w", err)
		}
		if err := tx.Create(auditRef, s.audit(op.Intent, identity.Subject, fingerprint, op.At)); err != nil {
			return fmt.Errorf("write completion audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("operator completion: %w", err)
	}
	return nil
}

func (s *FirestoreSupportOperator) sameCompletionAudit(event operatorAuditEvent, op CompletionOperation, identity OperatorIdentity, fingerprint string) bool {
	return event.SchemaVersion == 1 && event.Environment == op.Intent.Environment && event.Purpose == op.Intent.Purpose &&
		event.Action == "complete" && event.Actor == identity.Subject && event.Binding == s.mac("request", op.Intent.RequestRef) && event.Fingerprint == fingerprint
}

func simpleOperatorTerminal(snap *firestore.DocumentSnapshot) bool {
	if snap == nil || len(snap.Data()) != 4 {
		return false
	}
	v := snap.Data()
	_, version := v["schemaVersion"].(int64)
	_, completed := v["completedAt"].(time.Time)
	_, purpose := v["purpose"].(string)
	_, accepted := v["acceptedAt"].(time.Time)
	return version && completed && purpose && accepted
}
