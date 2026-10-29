package mypage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IntakeSecretは環境ごとのサーバー専用シークレットである。
// 冪等な再試行を受け付ける間は、インスタンスや再起動を跨いで同じ値を維持する必要がある。
// 未設定なら受付を無効にし、Viteのバンドルには含めない。
type FirestorePrivacyIntake struct {
	Client       *firestore.Client
	Environment  string
	IntakeSecret []byte
}

type PrivacyIntakeReceipt struct {
	RequestRef string         `json:"requestRef"`
	Challenge  string         `json:"supportChallenge,omitempty"`
	Purpose    SupportPurpose `json:"purpose"`
	Status     string         `json:"status"`
	AcceptedAt time.Time      `json:"acceptedAt"`
	DeleteBy   *time.Time     `json:"deleteBy"`
}

type PrivacyRequestStatus struct {
	RequestRef string         `json:"requestRef"`
	Purpose    SupportPurpose `json:"purpose"`
	Status     string         `json:"status"`
	AcceptedAt time.Time      `json:"acceptedAt"`
	DeleteBy   *time.Time     `json:"deleteBy"`
	VerifiedAt *time.Time     `json:"verifiedAt"`
	Reply      string         `json:"reply,omitempty"`
	ReplyAt    *time.Time     `json:"replyAt,omitempty"`
}

func validPrivacyBody(body string) bool {
	if body != strings.TrimSpace(body) || len(body) > 2000 || !utf8.ValidString(body) {
		return false
	}
	for _, r := range body {
		if unicode.IsControl(r) && r != '\n' {
			return false
		}
	}
	return true
}

func validSubmissionKey(key string) bool {
	if len(key) < 16 || len(key) > 128 {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func (s *FirestorePrivacyIntake) ready() bool {
	return s != nil && s.Client != nil && validSupportEnvironment(s.Environment) && len(s.IntakeSecret) >= 32
}

func (s *FirestorePrivacyIntake) keyed(label, uid, key string) string {
	mac := hmac.New(sha256.New, s.IntakeSecret)
	_, _ = mac.Write([]byte(label + "\x00" + s.Environment + "\x00" + uid + "\x00" + key))
	return hex.EncodeToString(mac.Sum(nil))
}

// Createは受付とチャレンジをアトミックに記録する。同じ本人・キー・内容の再試行では、
// ACK消失後も同じ推測不能なチャレンジを返す。対象はログイン済みuidとし、ブラウザ指定チャンネルは受け入れない。
func (s *FirestorePrivacyIntake) Create(ctx context.Context, uid, key string, purpose SupportPurpose, body string, now time.Time) (PrivacyIntakeReceipt, error) {
	if !s.ready() {
		return PrivacyIntakeReceipt{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	if uid == "" || strings.Contains(uid, "/") || !validSubmissionKey(key) || !validSupportPurpose(purpose) || !validPrivacyBody(body) {
		return PrivacyIntakeReceipt{}, apiError("INVALID_REQUEST")
	}
	ref := s.keyed("ref", uid, key)
	challenge := s.keyed("challenge", uid, key)
	requestID := s.keyed("request-id", uid, key)
	fingerprint := digest(string(purpose) + "\x00" + body)
	doc := s.Client.Collection("support-requests").Doc(ref)
	var value SupportRequest
	err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snapshot, err := tx.Get(doc)
		if err == nil {
			if err := snapshot.DataTo(&value); err != nil {
				return fmt.Errorf("decode intake receipt: %w", err)
			}
			if value.Environment != s.Environment || value.TargetChannel != uid || value.RequestID != requestID || value.SubmissionHash != fingerprint || value.Purpose != purpose {
				return apiError("INTAKE_KEY_CONFLICT")
			}
			return nil
		}
		if status.Code(err) != codes.NotFound {
			return fmt.Errorf("read intake receipt: %w", err)
		}
		value, err = newSupportRecord(requestID, s.Environment, uid, purpose, digest(challenge), now, now)
		if err != nil {
			return err
		}
		value.SubmissionHash = fingerprint
		value.Body = body
		if err := tx.Create(doc, value); err != nil {
			return fmt.Errorf("create intake receipt: %w", err)
		}
		if err := tx.Create(s.Client.Collection("support-challenges").Doc(value.ChallengeHash), supportChallengeIndex{RequestRef: ref, Environment: s.Environment}); err != nil {
			return fmt.Errorf("create intake challenge: %w", err)
		}
		if err := tx.Create(s.Client.Collection("support-request-ids").Doc(digest(s.Environment+":"+requestID)), supportChallengeIndex{RequestRef: ref, Environment: s.Environment}); err != nil {
			return fmt.Errorf("create intake ID: %w", err)
		}
		return nil
	})
	if err != nil {
		return PrivacyIntakeReceipt{}, fmt.Errorf("privacy intake: %w", err)
	}
	receipt := PrivacyIntakeReceipt{RequestRef: ref, Purpose: value.Purpose, Status: value.Status, AcceptedAt: value.AcceptedAt, DeleteBy: value.DeleteBy}
	if value.Status == "awaiting_proof" && value.ChallengeHash == digest(challenge) && now.Before(value.ChallengeExpiresAt) {
		receipt.Challenge = challenge
	}
	return receipt, nil
}

// Statusには、現在のFirebase本人情報と、新たなサポートOAuthの本人確認完了の両方を必要とする。
// 申請参照だけをBearer認証情報にしない。
func (s *FirestorePrivacyIntake) Status(ctx context.Context, uid, ref string) (PrivacyRequestStatus, error) {
	if !s.ready() {
		return PrivacyRequestStatus{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	if uid == "" || strings.Contains(uid, "/") || !validOpaque(ref) {
		return PrivacyRequestStatus{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	snap, err := s.Client.Collection("support-requests").Doc(ref).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return PrivacyRequestStatus{}, apiError("SUPPORT_CHALLENGE_INVALID")
		}
		return PrivacyRequestStatus{}, fmt.Errorf("read privacy status: %w", err)
	}
	var value SupportRequest
	if err := snap.DataTo(&value); err != nil {
		return PrivacyRequestStatus{}, fmt.Errorf("decode privacy status: %w", err)
	}
	if value.Environment != s.Environment || value.TargetChannel != uid || value.VerifiedAt == nil || value.Status != "verified" || value.ProofRef == "" {
		return PrivacyRequestStatus{}, apiError("SUPPORT_CHALLENGE_INVALID")
	}
	return PrivacyRequestStatus{RequestRef: ref, Purpose: value.Purpose, Status: value.Status, AcceptedAt: value.AcceptedAt, DeleteBy: value.DeleteBy, VerifiedAt: value.VerifiedAt, Reply: value.OperatorReply, ReplyAt: value.OperatorReplyAt}, nil
}
