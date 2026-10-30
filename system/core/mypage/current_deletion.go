package mypage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/supportdelete"
)

var ErrDeletionGate = errors.New("owner deletion runbook gate required")

const currentAuthTargets = "support-delete-current-auth-targets"

// OwnerDeletionGatesはprivate CLIを実行する所有者が実環境で照合した記録。
// SDKで検証した不存在と手動確認を混同しない。unknownは完了にしない。
// Gateは合成データや将来vendorをモデル化せず、実Runbookの未自動化部分だけを表す。
type OwnerDeletionGates struct {
	Selector          supportdelete.Selector        `json:"selector"`
	Generation        int64                         `json:"generation"`
	Cutoff            time.Time                     `json:"cutoff"`
	ExpiresAt         time.Time                     `json:"expiresAt"`
	WritersStoppedAt  time.Time                     `json:"writersStoppedAt"`
	CachePurgedAt     time.Time                     `json:"cachePurgedAt"`
	RestoreExcludedAt time.Time                     `json:"restoreExcludedAt"`
	WritersResumedAt  time.Time                     `json:"writersResumedAt"`
	Manual            map[string]DeletionGateRecord `json:"manual"`
}
type DeletionGateRecord struct {
	Status      string    `json:"status"` // completed or absent; pending/unknown never authorizes completion.
	EvidenceRef string    `json:"evidenceRef"`
	ObservedAt  time.Time `json:"observedAt"`
}

func (g OwnerDeletionGates) check(e supportdelete.Execution, now time.Time) error {
	if g.Selector != e.Selector || g.Generation != e.Generation || !g.Cutoff.Equal(e.Cutoff) || !now.Before(g.ExpiresAt) || g.ExpiresAt.After(now.Add(time.Hour)) {
		return ErrDeletionGate
	}
	// Reject early writer resumption before any effect can advance delete/inspect.
	// A later timestamp must not turn inspections made while writers ran into completion evidence.
	if e.Cursor < supportdelete.ResumeStart() && !g.WritersResumedAt.IsZero() {
		return ErrDeletionGate
	}
	for _, at := range []time.Time{g.WritersStoppedAt, g.CachePurgedAt, g.RestoreExcludedAt} {
		if at.Before(e.Cutoff) || at.After(now) {
			return ErrDeletionGate
		}
	}
	return nil
}

// CurrentBigQueryDataは現在の固定3テーブルだけを操作する実SDK境界。
type FirebaseDeletionClient interface {
	GetUser(context.Context, string) (*auth.UserRecord, error)
	RevokeRefreshTokens(context.Context, string) error
	DeleteUser(context.Context, string) error
}

type CurrentBigQueryData interface {
	ApplyChannelDeletion(context.Context, string, string, bool) error
}

// CurrentDeletionEffectsは現在のFirestore/channel-UID Auth/固定BQを直接接続する。
// Registry/Catalogやvendor SDKの合成実装は使わない。
type CurrentDeletionEffects struct {
	Store    *FirestoreDeletionStore
	Auth     FirebaseDeletionClient
	BigQuery CurrentBigQueryData
	Gates    OwnerDeletionGates
	Clock    func() time.Time
}

func (a *CurrentDeletionEffects) gateEvidence(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	if ctx.Err() != nil || a.Gates.check(op.Execution, a.Clock()) != nil {
		return supportdelete.Evidence{}, ErrDeletionGate
	}
	e := op.Execution
	now := a.Clock()
	return supportdelete.Evidence{Selector: e.Selector, ManifestRef: e.Manifest.Ref, OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope, Mode: e.Manifest.Mode, TraceRef: digest("owner-runbook:" + op.ID + ":" + a.Gates.RestoreExcludedAt.String()), Generation: e.Generation, Cutoff: e.Cutoff, ObservedAt: now, Known: true, Complete: true, AllInstances: true, OldQueueRejected: true, RestoreExcluded: true}, nil
}

type currentGateEffects struct{ current *CurrentDeletionEffects }

func (g currentGateEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	return g.current.gateEvidence(ctx, op)
}

func (a *CurrentDeletionEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	if a == nil || a.Store == nil || a.Clock == nil || a.Auth == nil || a.BigQuery == nil || op.Execution.Validate() != nil || op.Execution.Cursor >= len(supportdelete.Steps()) || op != supportdelete.NewOperation(op.Execution, op.ObservedAfter) || a.Store.check(op.Execution.Selector) != nil {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	v, err := a.gateEvidence(ctx, op)
	if err != nil {
		return v, err
	}
	// Every side effect is preceded by a transactional owner/generation check.
	if err = a.fence(ctx, op.Execution); err != nil {
		return supportdelete.Evidence{}, err
	}
	switch op.Step.Scope {
	case "runtime-my-page", "runtime-legacy":
		if op.Step.Action == "resume" && (a.Gates.WritersResumedAt.Before(op.Execution.Cutoff) || (op.Execution.Cursor == supportdelete.ResumeStart() && a.Gates.WritersResumedAt.Before(op.Execution.UpdatedAt)) || a.Gates.WritersResumedAt.After(a.Clock())) {
			return supportdelete.Evidence{}, ErrDeletionGate
		}
	case "seat-state", "firestore-primary", "firestore-web", "firestore-oauth", "firestore-support-relations":
		return (&FirestoreDeletionEffects{Store: a.Store, RestoreGuard: currentGateEffects{a}, Clock: a.Clock}).Apply(ctx, op)
	case "firebase-auth":
		if err = a.authOperation(ctx, op); err != nil {
			return supportdelete.Evidence{}, err
		}
	case "bigquery":
		if err = a.BigQuery.ApplyChannelDeletion(ctx, op.Execution.Selector.Target.ChannelID, op.ID, op.Step.Action == "inspect"); err != nil {
			return supportdelete.Evidence{}, fmt.Errorf("delete current BigQuery data: %w", err)
		}
	case "legacy-mappings", "backups-exports", "derived-vendor", "platform-logs":
		record, ok := a.Gates.Manual[op.Step.Scope]
		if !ok || (record.Status != "completed" && record.Status != "absent") || !supportdelete.ValidRef(record.EvidenceRef) || record.ObservedAt.Before(op.Execution.Cutoff) || record.ObservedAt.After(a.Clock()) {
			return supportdelete.Evidence{}, ErrDeletionGate
		}
		v.TraceRef = record.EvidenceRef
	default:
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	if err = a.fence(ctx, op.Execution); err != nil {
		return supportdelete.Evidence{}, err
	}
	v.ObservedAt = a.Clock()
	return v, nil
}

func (a *CurrentDeletionEffects) fence(ctx context.Context, e supportdelete.Execution) error {
	err := a.Store.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		_, _, err := a.Store.readFence(tx, e, true)
		return err
	})
	return deletionStoreError(err)
}

func (a *CurrentDeletionEffects) authOperation(ctx context.Context, op supportdelete.Operation) error {
	e := op.Execution
	uid := e.Selector.Target.ChannelID
	user, err := a.Auth.GetUser(ctx, uid)
	created := int64(-1)
	if err != nil && !auth.IsUserNotFound(err) {
		return supportdelete.ErrUnavailable
	}
	if err == nil {
		if user == nil || user.UserInfo == nil || user.UID != uid || user.UserMetadata == nil || user.UserMetadata.CreationTimestamp <= 0 {
			return supportdelete.ErrEvidence
		}
		created = user.UserMetadata.CreationTimestamp
	}
	// Capture the exact Auth incarnation once, before revoke/deletion. A retry may observe absence,
	// but never adopt a recreated UID. All Auth writers/imports must remain stopped by the Runbook gate.
	ref := a.Store.Client.Collection(currentAuthTargets).Doc(e.Selector.ExecutionRef)
	err = a.Store.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		if _, _, err := a.Store.readFence(tx, e, true); err != nil {
			return err
		}
		doc, err := tx.Get(ref)
		if status.Code(err) == codes.NotFound {
			if e.Cursor != 0 {
				return supportdelete.ErrConflict
			}
			if err := tx.Create(ref, map[string]any{"createdAtMillis": created, "inspected": false}); err != nil {
				return fmt.Errorf("capture current Auth incarnation: %w", err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("read current Auth incarnation: %w", err)
		}
		old, ok := doc.Data()["createdAtMillis"].(int64)
		if !ok || len(doc.Data()) != 2 || (created != -1 && old != created) {
			return supportdelete.ErrConflict
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("persist current Auth incarnation: %w", err)
	}
	switch op.Step.Action {
	case "revoke":
		if created != -1 {
			if err = a.Auth.RevokeRefreshTokens(ctx, uid); err != nil && !auth.IsUserNotFound(err) {
				return supportdelete.ErrUnavailable
			}
		}
	case "delete":
		if created != -1 {
			if err = a.Auth.DeleteUser(ctx, uid); err != nil && !auth.IsUserNotFound(err) {
				return supportdelete.ErrUnavailable
			}
		}
	case "inspect":
		if created != -1 {
			return supportdelete.ErrEvidence
		}
		err = a.Store.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
			if _, _, err := a.Store.readFence(tx, e, true); err != nil {
				return err
			}
			doc, err := tx.Get(ref)
			if err != nil {
				return fmt.Errorf("read current Auth verification: %w", err)
			}
			return tx.Update(ref, []firestore.Update{{Path: "inspected", Value: true}}, firestore.LastUpdateTime(doc.UpdateTime))
		})
		return deletionStoreError(err)
	default:
		return supportdelete.ErrInvalid
	}
	return nil
}

// Finalize reads this before any writes and scrubs the Auth incarnation snapshot atomically.
func (s *FirestoreDeletionStore) prepareCurrentAuthTarget(tx *firestore.Transaction, e supportdelete.Execution) (func() error, error) {
	ref := s.Client.Collection(currentAuthTargets).Doc(e.Selector.ExecutionRef)
	doc, err := tx.Get(ref)
	if status.Code(err) == codes.NotFound && e.Manifest.Mode != "live" {
		return func() error { return nil }, nil
	}
	if err != nil {
		return nil, supportdelete.ErrEvidence
	}
	_, valid := doc.Data()["createdAtMillis"].(int64)
	if !valid || len(doc.Data()) != 2 || doc.Data()["inspected"] != true {
		return nil, supportdelete.ErrEvidence
	}
	return func() error { return tx.Delete(ref) }, nil
}
