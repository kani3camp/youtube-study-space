package mypage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"firebase.google.com/go/v4/auth"

	"app.modules/core/supportdelete"
)

// FirebaseDeletionClient is satisfied by the pinned Admin SDK's project and
// tenant clients. No constructor, credential discovery or live wiring is here.
type FirebaseDeletionClient interface {
	GetUser(context.Context, string) (*auth.UserRecord, error)
	RevokeRefreshTokens(context.Context, string) error
	DeleteUser(context.Context, string) error
}

var (
	_                       FirebaseDeletionClient = (*auth.Client)(nil)
	_                       FirebaseDeletionClient = (*auth.TenantClient)(nil)
	ErrAuthOwnershipMissing                        = errors.New("deletion ownership snapshot missing")
)

// FirebaseDeletionScope pins a client to an explicitly inventoried project and
// tenant. Empty TenantID is allowed only when the inventory explicitly says so.
// UserNotFound is an injected fake SDK classifier; nil uses auth.IsUserNotFound.
type FirebaseDeletionScope struct {
	ProjectID    string
	TenantID     string
	Client       FirebaseDeletionClient
	UserNotFound func(error) bool
}

type AuthOwnedIdentity struct {
	UID             string   `firestore:"uid"`
	TenantID        string   `firestore:"tenantID"`
	CreatedAtMillis int64    `firestore:"createdAtMillis"`
	OwnerChannels   []string `firestore:"ownerChannels"`
}

// Fingerprint is the digest of the entire synthetic document, not a lone UID
// field. Attribution and completeness come from an independent inventory.
type AuthOwnedDocument struct {
	Collection    string   `firestore:"collection"`
	ID            string   `firestore:"id"`
	Fingerprint   string   `firestore:"fingerprint"`
	OwnerChannels []string `firestore:"ownerChannels"`
}

// AuthOwnershipSnapshot is server-only temporary execution state. The store
// moves its full execution fence with each checkpoint and deletes it in the
// terminal transaction. It must never outlive a successful finalization.
type AuthOwnershipSnapshot struct {
	SchemaVersion      int64                   `firestore:"schemaVersion"`
	Execution          supportdelete.Execution `firestore:"execution"`
	CaptureOperationID string                  `firestore:"captureOperationID"`
	TraceRef           string                  `firestore:"traceRef"`
	CapturedAt         time.Time               `firestore:"capturedAt"`
	Complete           bool                    `firestore:"complete"`
	Identities         []AuthOwnedIdentity     `firestore:"identities"`
	Mappings           []AuthOwnedDocument     `firestore:"mappings"`
	Related            []AuthOwnedDocument     `firestore:"related"`
	MappingsRemoved    bool                    `firestore:"mappingsRemoved"`
}

type AuthOwnershipInventory interface {
	Capture(context.Context, supportdelete.Operation) (AuthOwnershipSnapshot, error)
}

type AuthDeletionPersistence interface {
	LoadAuthOwnership(context.Context, supportdelete.Execution) (AuthOwnershipSnapshot, error)
	CaptureAuthOwnership(context.Context, AuthOwnershipSnapshot) error
	CheckAuthOwnership(context.Context, AuthOwnershipSnapshot) error
	ApplyAuthRecords(context.Context, supportdelete.Operation, AuthOwnershipSnapshot) error
}

// The external guard holds exclusion of UID recreation/import and ownership
// changes across the entire callback. GetUser + DeleteUser has no SDK CAS.
// RevokedTokenChecks covers every old-token consumer independently; a revoke
// acknowledgement does not invalidate already issued ID tokens by itself.
type AuthDeletionPermit struct {
	Evidence           supportdelete.Evidence
	IncarnationLocked  bool
	RevokedTokenChecks bool
	SynchronizedClock  bool
}

type AuthDeletionGuard interface {
	WithGuard(context.Context, supportdelete.Operation, AuthOwnershipSnapshot, func(context.Context, AuthDeletionPermit) error) error
}

// FirebaseDeletionEffects is deliberately synthetic-only. Trusted completeness,
// incarnation exclusion and token-consumer evidence must be injected even for
// fixtures. It cannot make a live deletion workflow available.
type FirebaseDeletionEffects struct {
	Store     AuthDeletionPersistence
	Inventory AuthOwnershipInventory
	Scopes    []FirebaseDeletionScope
	Guard     AuthDeletionGuard
	Clock     func() time.Time
}

func validAuthDocumentID(id string) bool {
	return id != "" && len(id) <= 1500 && id != "." && id != ".." && !strings.Contains(id, "/")
}

func (s AuthOwnershipSnapshot) validate() error {
	e := s.Execution
	if s.SchemaVersion != 1 || e.Validate() != nil || (e.Manifest.Mode != "mock" && e.Manifest.Mode != "emulator") || !s.Complete || !supportdelete.ValidRef(s.TraceRef) || s.CapturedAt.Before(e.Cutoff) || len(s.Identities) > 100 || len(s.Mappings)+len(s.Related) > 100 || s.CaptureOperationID != supportdelete.NewOperation(withAuthCursorZero(e), e.Cutoff).ID {
		return supportdelete.ErrEvidence
	}
	seen := map[string]bool{}
	for _, i := range s.Identities {
		key := i.TenantID + ":" + i.UID
		if !validAuthDocumentID(i.UID) || len(i.UID) > 128 || strings.Contains(i.TenantID, "/") || len(i.TenantID) > 128 || i.CreatedAtMillis <= 0 || i.CreatedAtMillis > e.Cutoff.UnixMilli() || len(i.OwnerChannels) != 1 || i.OwnerChannels[0] != e.Selector.Target.ChannelID || seen[key] {
			return supportdelete.ErrEvidence
		}
		seen[key] = true
	}
	seen = map[string]bool{}
	for _, group := range [][]AuthOwnedDocument{s.Mappings, s.Related} {
		for _, d := range group {
			key := d.Collection + "/" + d.ID
			if !validAuthDocumentID(d.ID) || !supportdelete.ValidRef(d.Fingerprint) || len(d.OwnerChannels) != 1 || d.OwnerChannels[0] != e.Selector.Target.ChannelID || seen[key] {
				return supportdelete.ErrEvidence
			}
			seen[key] = true
		}
	}
	for _, d := range s.Mappings {
		if d.Collection != "mypage-youtube-channel-owners" && d.Collection != "mypage-users" {
			return supportdelete.ErrEvidence
		}
		if d.Collection == "mypage-youtube-channel-owners" && d.ID != e.Selector.Target.ChannelID {
			return supportdelete.ErrEvidence
		}
		if d.Collection == "mypage-users" && !s.hasUID(d.ID) {
			return supportdelete.ErrEvidence
		}
	}
	for _, d := range s.Related {
		switch d.Collection {
		case "support-requests", "oauth-transactions", "support-challenges", "support-request-ids":
		default:
			return supportdelete.ErrEvidence
		}
	}
	return nil
}

func withAuthCursorZero(e supportdelete.Execution) supportdelete.Execution {
	e.Cursor = 0
	return e
}

func (s AuthOwnershipSnapshot) hasUID(uid string) bool {
	for _, i := range s.Identities {
		if i.UID == uid {
			return true
		}
	}
	return false
}

func (a *FirebaseDeletionEffects) scope(e supportdelete.Execution, i AuthOwnedIdentity) (FirebaseDeletionScope, error) {
	var result FirebaseDeletionScope
	found := false
	for _, scope := range a.Scopes {
		if scope.ProjectID == e.Selector.Target.ProjectID && scope.TenantID == i.TenantID {
			if found || scope.Client == nil {
				return result, supportdelete.ErrUnavailable
			}
			result, found = scope, true
		}
	}
	if !found {
		return result, supportdelete.ErrUnavailable
	}
	return result, nil
}

func inspectOwnedAuth(ctx context.Context, scope FirebaseDeletionScope, i AuthOwnedIdentity) (*auth.UserRecord, error) {
	u, err := scope.Client.GetUser(ctx, i.UID)
	if err != nil {
		notFound := auth.IsUserNotFound
		if scope.UserNotFound != nil {
			notFound = scope.UserNotFound
		}
		if notFound(err) {
			return nil, nil
		}
		return nil, supportdelete.ErrUnavailable
	}
	if u == nil || u.UserInfo == nil || u.UserMetadata == nil || u.UID != i.UID || u.TenantID != i.TenantID || u.UserMetadata.CreationTimestamp != i.CreatedAtMillis {
		return nil, supportdelete.ErrConflict
	}
	return u, nil
}

func (a *FirebaseDeletionEffects) ownership(ctx context.Context, op supportdelete.Operation) (AuthOwnershipSnapshot, error) {
	s, err := a.Store.LoadAuthOwnership(ctx, op.Execution)
	if errors.Is(err, ErrAuthOwnershipMissing) {
		if op.Execution.Cursor != 0 || a.Inventory == nil {
			return s, supportdelete.ErrEvidence
		}
		s, err = a.Inventory.Capture(ctx, op)
		if err != nil || s.Execution != op.Execution || s.MappingsRemoved || s.validate() != nil || s.CapturedAt.Before(op.ObservedAfter) || s.CapturedAt.After(a.Clock()) {
			return s, supportdelete.ErrEvidence
		}
		for _, i := range s.Identities {
			scope, scopeErr := a.scope(op.Execution, i)
			if scopeErr != nil {
				return s, scopeErr
			}
			if _, inspectErr := inspectOwnedAuth(ctx, scope, i); inspectErr != nil {
				return s, inspectErr
			}
		}
		if err = a.Store.CaptureAuthOwnership(ctx, s); err != nil {
			return s, fmt.Errorf("capture deletion ownership: %w", err)
		}
		// Firestore stores timestamps at microsecond precision. Use the durable
		// representation for subsequent exact comparisons, including first apply.
		s, err = a.Store.LoadAuthOwnership(ctx, op.Execution)
		if err != nil {
			return s, fmt.Errorf("reload captured deletion ownership: %w", err)
		}
	} else if err != nil {
		return s, fmt.Errorf("load deletion ownership: %w", err)
	}
	if s.Execution != op.Execution || s.validate() != nil {
		return s, supportdelete.ErrConflict
	}
	return s, nil
}

func (a *FirebaseDeletionEffects) Apply(ctx context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	if a == nil || a.Store == nil || a.Guard == nil || a.Clock == nil || op.Execution.Validate() != nil || op.Execution.Manifest.Mode == "live" || op.Execution.Cursor >= len(supportdelete.Steps()) || op != supportdelete.NewOperation(op.Execution, op.ObservedAfter) || op.ObservedAfter.Before(op.Execution.UpdatedAt) || op.ObservedAfter.After(a.Clock()) {
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	switch op.Step.Scope {
	case "firebase-auth", "legacy-mappings", "firestore-oauth", "firestore-support-relations":
	default:
		return supportdelete.Evidence{}, supportdelete.ErrUnavailable
	}
	s, err := a.ownership(ctx, op)
	if err != nil {
		return supportdelete.Evidence{}, err
	}
	var evidence supportdelete.Evidence
	called := false
	err = a.Guard.WithGuard(ctx, op, s, func(guardCtx context.Context, permit AuthDeletionPermit) error {
		if called || !permit.IncarnationLocked || !permit.RevokedTokenChecks || !permit.SynchronizedClock || permit.Evidence.Validate(op, a.Clock()) != nil || !permit.Evidence.RestoreExcluded || !permit.Evidence.AllInstances || !permit.Evidence.OldQueueRejected {
			return supportdelete.ErrEvidence
		}
		called = true
		if err := a.Store.CheckAuthOwnership(guardCtx, s); err != nil {
			return fmt.Errorf("check deletion ownership before effect: %w", err)
		}
		if op.Step.Scope != "firebase-auth" {
			if err := a.Store.ApplyAuthRecords(guardCtx, op, s); err != nil {
				return fmt.Errorf("apply deletion records: %w", err)
			}
		} else {
			for _, i := range s.Identities {
				scope, err := a.scope(op.Execution, i)
				if err != nil {
					return err
				}
				if err := reconcileOwnedAuth(guardCtx, scope, i, op); err != nil {
					return err
				}
			}
		}
		if err := a.Store.CheckAuthOwnership(guardCtx, s); err != nil {
			return fmt.Errorf("check deletion ownership after effect: %w", err)
		}
		evidence = permit.Evidence
		evidence.ObservedAt = a.Clock()
		return nil
	})
	if err != nil || !called || evidence.Validate(op, a.Clock()) != nil {
		if err != nil {
			return supportdelete.Evidence{}, fmt.Errorf("guard deletion effect: %w", err)
		}
		return supportdelete.Evidence{}, supportdelete.ErrEvidence
	}
	return evidence, nil
}

func reconcileOwnedAuth(ctx context.Context, scope FirebaseDeletionScope, i AuthOwnedIdentity, op supportdelete.Operation) error {
	u, err := inspectOwnedAuth(ctx, scope, i)
	if err != nil || u == nil {
		return err
	}
	switch op.Step.Action {
	case "inspect":
		return supportdelete.ErrEvidence
	case "revoke":
		// The SDK stores UTC seconds. Require the first whole second strictly
		// after cutoff, including tokens issued in the cutoff's second.
		threshold := (op.Execution.Cutoff.Unix() + 1) * 1000
		if u.TokensValidAfterMillis >= threshold {
			return nil
		}
		// Acknowledgement is not evidence. Even errors/timeouts/lost acks are
		// reconciled only by a definitive read of the same incarnation.
		mutationErr := scope.Client.RevokeRefreshTokens(ctx, i.UID)
		u, err = inspectOwnedAuth(ctx, scope, i)
		if err != nil {
			return err
		}
		if u == nil || u.TokensValidAfterMillis >= threshold {
			return nil
		}
		if mutationErr != nil {
			return supportdelete.ErrUnavailable
		}
		return supportdelete.ErrEvidence
	case "delete":
		mutationErr := scope.Client.DeleteUser(ctx, i.UID)
		u, err = inspectOwnedAuth(ctx, scope, i)
		if err != nil {
			return err
		}
		if u == nil {
			return nil
		}
		if mutationErr != nil {
			return supportdelete.ErrUnavailable
		}
		return supportdelete.ErrEvidence
	default:
		return supportdelete.ErrUnavailable
	}
}
