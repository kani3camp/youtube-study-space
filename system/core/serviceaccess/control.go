// Package serviceaccessは、ユーザーデータから独立してチャンネル制限を管理する。
// MyPageが最初の利用先であり、旧Botのモデレーションは接続しない。
package serviceaccess

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"
)

const Collection = "service-access-controls"

type Reason string

const (
	Moderation      Reason = "moderation"
	PrivacyDeletion Reason = "privacyDeletion"
)

var (
	ErrUnavailable   = errors.New("service access unavailable")
	ErrRestricted    = errors.New("service access restricted")
	ErrDeletion      = errors.New("data deletion in progress")
	ErrInvalidChange = errors.New("invalid service access change")
	channelID        = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
	opaqueRef        = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type ReasonState struct {
	Active     bool      `firestore:"active"`
	Since      time.Time `firestore:"since,omitempty"`
	ReasonCode string    `firestore:"reasonCode,omitempty"`
	ActionRef  string    `firestore:"actionRef,omitempty"`
	RequestRef string    `firestore:"requestRef,omitempty"`
}

type Control struct {
	Moderation      ReasonState `firestore:"moderation"`
	PrivacyDeletion ReasonState `firestore:"privacyDeletion"`
	Generation      int64       `firestore:"generation"`
	UpdatedAt       time.Time   `firestore:"updatedAt"`
}

// Revisionで、カウンターを再利用した削除・再作成を検出する。
// レコード不存在の許可はキャッシュせず、保護対象リクエストのたびに読み直す。
type Snapshot struct {
	Control  Control
	Exists   bool
	Revision time.Time
}

type Reader interface {
	Read(context.Context, string) (Snapshot, error)
}

type Change struct {
	Reason     Reason
	Active     bool
	ReasonCode string
	Reference  string
}

type Store interface {
	Reader
	Change(context.Context, string, Change, time.Time) (Snapshot, error)
}

func ValidChannel(id string) bool { return channelID.MatchString(id) }

func validCode(code string) bool {
	switch code {
	case "MODERATION", "SECURITY", "POLICY_VIOLATION", "LEGACY_COMPATIBILITY":
		return true
	}
	return false
}

func validReason(state ReasonState, reason Reason, updated time.Time) bool {
	if !state.Active {
		return state.Since.IsZero() && state.ReasonCode == "" && state.ActionRef == "" && state.RequestRef == ""
	}
	if state.Since.IsZero() || state.Since.After(updated) {
		return false
	}
	if reason == Moderation {
		return validCode(state.ReasonCode) && opaqueRef.MatchString(state.ActionRef) && state.RequestRef == ""
	}
	return state.ReasonCode == "PRIVACY_DELETION" && opaqueRef.MatchString(state.RequestRef) && state.ActionRef == ""
}

func (s Snapshot) Validate() error {
	if !s.Exists {
		if s.Control != (Control{}) || !s.Revision.IsZero() {
			return ErrUnavailable
		}
		return nil
	}
	c := s.Control
	if c.Generation < 1 || c.UpdatedAt.IsZero() || s.Revision.IsZero() ||
		!validReason(c.Moderation, Moderation, c.UpdatedAt) || !validReason(c.PrivacyDeletion, PrivacyDeletion, c.UpdatedAt) {
		return ErrUnavailable
	}
	return nil
}

func (s Snapshot) Allowed() error {
	if err := s.Validate(); err != nil {
		return err
	}
	// 独立したブロックを解除せず、削除中の表示を優先する。
	if s.Control.PrivacyDeletion.Active {
		return ErrDeletion
	}
	if s.Control.Moderation.Active {
		return ErrRestricted
	}
	return nil
}

func (s Snapshot) Checkpoint() string {
	if !s.Exists {
		return "absent"
	}
	return fmt.Sprintf("%d:%s", s.Control.Generation, s.Revision.UTC().Format(time.RFC3339Nano))
}

// Transitionは同じ要求状態・参照に対して冪等である。
// Aでは無効状態のチェックポイントにTTLや削除処理を設けない。
// ライフサイクルと処理完了待ちの方針は、別途レビューする削除運用の作業で扱う。
func Transition(old Snapshot, change Change, now time.Time) (Control, bool, error) {
	if old.Validate() != nil || now.IsZero() {
		return Control{}, false, ErrUnavailable
	}
	if change.Reason != Moderation && change.Reason != PrivacyDeletion {
		return Control{}, false, ErrInvalidChange
	}
	if change.Active {
		if !opaqueRef.MatchString(change.Reference) ||
			(change.Reason == Moderation && !validCode(change.ReasonCode)) ||
			(change.Reason == PrivacyDeletion && change.ReasonCode != "PRIVACY_DELETION") {
			return Control{}, false, ErrInvalidChange
		}
	} else if change.Reference != "" || change.ReasonCode != "" {
		return Control{}, false, ErrInvalidChange
	}
	next := old.Control
	current := &next.Moderation
	if change.Reason == PrivacyDeletion {
		current = &next.PrivacyDeletion
	}
	wanted := ReasonState{}
	if change.Active {
		wanted = ReasonState{Active: true, Since: now.UTC().Truncate(time.Microsecond), ReasonCode: change.ReasonCode}
		if current.Active {
			wanted.Since = current.Since
		}
		if change.Reason == Moderation {
			wanted.ActionRef = change.Reference
		} else {
			wanted.RequestRef = change.Reference
		}
	}
	if *current == wanted {
		return next, false, nil
	}
	if next.Generation == math.MaxInt64 || (!next.UpdatedAt.IsZero() && now.Before(next.UpdatedAt)) {
		return Control{}, false, ErrUnavailable
	}
	*current = wanted
	next.Generation++
	next.UpdatedAt = now.UTC().Truncate(time.Microsecond)
	return next, true, nil
}
