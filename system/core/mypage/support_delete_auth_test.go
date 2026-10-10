package mypage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/require"

	"app.modules/core/supportdelete"
)

var errFakeAuthUserMissing = errors.New("synthetic SDK USER_NOT_FOUND")

type fakeDeletionSDK struct {
	user          *auth.UserRecord
	getErr        error
	mutationErr   error
	postWriteErr  error
	applyMutation bool
	revokeMillis  int64
	revokes       int
	deletes       int
	onDelete      func()
}

func (f *fakeDeletionSDK) GetUser(context.Context, string) (*auth.UserRecord, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.user == nil {
		return nil, errFakeAuthUserMissing
	}
	u := *f.user
	return &u, nil
}

func (f *fakeDeletionSDK) RevokeRefreshTokens(context.Context, string) error {
	f.revokes++
	if f.applyMutation {
		f.user.TokensValidAfterMillis = f.revokeMillis
	}
	f.getErr = f.postWriteErr
	return f.mutationErr
}

func (f *fakeDeletionSDK) DeleteUser(context.Context, string) error {
	f.deletes++
	if f.onDelete != nil {
		f.onDelete()
	}
	if f.applyMutation {
		f.user = nil
	}
	f.getErr = f.postWriteErr
	return f.mutationErr
}

type fakeAuthInventory struct {
	snapshot AuthOwnershipSnapshot
	calls    int
}

func (f *fakeAuthInventory) Capture(context.Context, supportdelete.Operation) (AuthOwnershipSnapshot, error) {
	f.calls++
	return f.snapshot, nil
}

type fakeAuthPersistence struct {
	snapshot *AuthOwnershipSnapshot
	checks   int
	records  int
}

func (f *fakeAuthPersistence) LoadAuthOwnership(_ context.Context, e supportdelete.Execution) (AuthOwnershipSnapshot, error) {
	if f.snapshot == nil {
		return AuthOwnershipSnapshot{}, ErrAuthOwnershipMissing
	}
	if f.snapshot.Execution != e {
		return AuthOwnershipSnapshot{}, supportdelete.ErrConflict
	}
	return *f.snapshot, nil
}

func (f *fakeAuthPersistence) CaptureAuthOwnership(_ context.Context, snapshot AuthOwnershipSnapshot) error {
	f.snapshot = &snapshot
	return nil
}

func (f *fakeAuthPersistence) CheckAuthOwnership(_ context.Context, s AuthOwnershipSnapshot) error {
	f.checks++
	if f.snapshot == nil || f.snapshot.Execution != s.Execution {
		return supportdelete.ErrConflict
	}
	return nil
}

func (f *fakeAuthPersistence) ApplyAuthRecords(_ context.Context, op supportdelete.Operation, _ AuthOwnershipSnapshot) error {
	f.records++
	if op.Step.Scope == "legacy-mappings" && op.Step.Action == "delete" {
		f.snapshot.MappingsRemoved = true
	}
	return nil
}

type fakeAuthGuard struct {
	deny       bool
	noCallback bool
	noLock     bool
	noToken    bool
	noClock    bool
	calls      int
	lifecycle  sync.Mutex
}

func (f *fakeAuthGuard) WithGuard(ctx context.Context, op supportdelete.Operation, _ AuthOwnershipSnapshot, apply func(context.Context, AuthDeletionPermit) error) error {
	f.calls++
	if f.deny {
		return supportdelete.ErrEvidence
	}
	if f.noCallback {
		return nil
	}
	f.lifecycle.Lock()
	defer f.lifecycle.Unlock()
	e := op.Execution
	permit := AuthDeletionPermit{Evidence: supportdelete.Evidence{Selector: e.Selector, ManifestRef: e.Manifest.Ref, OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope, Mode: e.Manifest.Mode, TraceRef: digest("synthetic-auth-guard:" + op.ID), Generation: e.Generation, Cutoff: e.Cutoff, ObservedAt: op.ObservedAfter, Known: true, Complete: true, AllInstances: true, OldQueueRejected: true, RestoreExcluded: true}, IncarnationLocked: !f.noLock, RevokedTokenChecks: !f.noToken, SynchronizedClock: !f.noClock}
	return apply(ctx, permit)
}

func authAdapterFixture() (*FirebaseDeletionEffects, *fakeAuthPersistence, *fakeAuthInventory, *fakeDeletionSDK, *fakeAuthGuard, supportdelete.Operation) {
	cutoff := time.Date(2026, 10, 7, 0, 0, 0, 123000000, time.UTC)
	now := cutoff.Add(2 * time.Second)
	e := supportdelete.Execution{Selector: supportdelete.MockSelector(), Manifest: supportdelete.Manifest{Ref: digest("synthetic-auth-manifest"), Mode: "mock"}, OwnerRef: digest("synthetic-auth-owner"), Revision: 1, Generation: 1, GuardSince: cutoff, Cutoff: cutoff, AcceptedAt: cutoff.Add(-time.Hour), DeleteBy: cutoff.Add(-time.Hour).Add(7 * 24 * time.Hour), UpdatedAt: cutoff, EvidenceDigest: digest("synthetic-chain")}
	op := supportdelete.NewOperation(e, now)
	identity := AuthOwnedIdentity{UID: "synthetic-old-uid", TenantID: "synthetic-tenant", CreatedAtMillis: cutoff.Add(-time.Hour).UnixMilli(), OwnerChannels: []string{e.Selector.Target.ChannelID}}
	inventory := &fakeAuthInventory{snapshot: AuthOwnershipSnapshot{SchemaVersion: 1, Execution: e, CaptureOperationID: op.ID, TraceRef: digest("synthetic-complete-inventory"), CapturedAt: now, Complete: true, Identities: []AuthOwnedIdentity{identity}}}
	sdk := &fakeDeletionSDK{user: &auth.UserRecord{UserInfo: &auth.UserInfo{UID: identity.UID}, UserMetadata: &auth.UserMetadata{CreationTimestamp: identity.CreatedAtMillis}, TenantID: identity.TenantID}, applyMutation: true, revokeMillis: now.Unix() * 1000}
	store := &fakeAuthPersistence{}
	guard := &fakeAuthGuard{}
	adapter := &FirebaseDeletionEffects{Store: store, Inventory: inventory, Scopes: []FirebaseDeletionScope{{ProjectID: e.Selector.Target.ProjectID, TenantID: identity.TenantID, Client: sdk, UserNotFound: func(err error) bool { return errors.Is(err, errFakeAuthUserMissing) }}}, Guard: guard, Clock: func() time.Time { return now }}
	return adapter, store, inventory, sdk, guard, op
}

func fakeAuthAdvance(store *fakeAuthPersistence, op supportdelete.Operation, action, scope string) supportdelete.Operation {
	e := op.Execution
	for index, step := range supportdelete.Steps() {
		if step.Action == action && step.Scope == scope {
			e.Cursor = index
			break
		}
	}
	e.Revision++
	e.UpdatedAt = op.ObservedAfter
	if store.snapshot != nil {
		store.snapshot.Execution = e
	}
	return supportdelete.NewOperation(e, op.ObservedAfter)
}

func TestFirebaseDeletionCompleteInventoryCapturedAfterAcquireClockAdvances(t *testing.T) {
	a, store, inventory, sdk, _, op := authAdapterFixture()
	require.True(t, inventory.snapshot.CapturedAt.After(op.Execution.UpdatedAt))
	v, err := a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.NoError(t, v.Validate(op, a.Clock()))
	require.NotNil(t, store.snapshot)
	require.Equal(t, op.Execution, store.snapshot.Execution)
	require.Equal(t, 1, sdk.revokes)
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.Equal(t, 1, inventory.calls, "same operation uses the durable ownership snapshot")
	require.Equal(t, 1, sdk.revokes, "definitive revoke postcondition prevents duplicate mutation")
}

func TestFirebaseDeletionOwnershipAmbiguityFailsBeforeAnyMutation(t *testing.T) {
	for _, condition := range []string{"missing-inventory", "incomplete", "shared-owner", "wrong-owner", "wrong-uid", "wrong-tenant", "incarnation", "unknown", "permission", "missing-project-scope", "missing-tenant-scope", "duplicate-identity", "future-capture", "wrong-selector", "live"} {
		t.Run(condition, func(t *testing.T) {
			a, store, inventory, sdk, _, op := authAdapterFixture()
			switch condition {
			case "missing-inventory":
				a.Inventory = nil
			case "incomplete":
				inventory.snapshot.Complete = false
			case "shared-owner":
				inventory.snapshot.Identities[0].OwnerChannels = append(inventory.snapshot.Identities[0].OwnerChannels, "UCsynthetic0000000000002")
			case "wrong-owner":
				inventory.snapshot.Identities[0].OwnerChannels[0] = "UCsynthetic0000000000002"
			case "wrong-uid":
				sdk.user.UID = "synthetic-different-user"
			case "wrong-tenant":
				sdk.user.TenantID = "synthetic-other-tenant"
			case "incarnation":
				sdk.user.UserMetadata.CreationTimestamp++
			case "unknown":
				sdk.getErr = context.DeadlineExceeded
			case "permission":
				sdk.getErr = errors.New("synthetic permission denied, not USER_NOT_FOUND")
			case "missing-project-scope":
				a.Scopes[0].ProjectID = "demo-other-project"
			case "missing-tenant-scope":
				a.Scopes[0].TenantID = ""
			case "duplicate-identity":
				inventory.snapshot.Identities = append(inventory.snapshot.Identities, inventory.snapshot.Identities[0])
			case "future-capture":
				inventory.snapshot.CapturedAt = a.Clock().Add(time.Second)
			case "wrong-selector":
				inventory.snapshot.Execution.Selector.ProofRef = digest("wrong-proof")
			case "live":
				op.Execution.Manifest.Mode = "live"
				op = supportdelete.NewOperation(op.Execution, op.ObservedAfter)
			}
			_, err := a.Apply(context.Background(), op)
			require.Error(t, err)
			require.Nil(t, store.snapshot)
			require.Zero(t, sdk.revokes)
			require.Zero(t, sdk.deletes)
		})
	}
}

func TestFirebaseDeletionRequiresIncarnationTokenAndClockGuards(t *testing.T) {
	for _, condition := range []string{"nil", "unknown", "callback-missing", "no-incarnation-lock", "no-revoked-token-check", "no-clock-sync"} {
		t.Run(condition, func(t *testing.T) {
			a, _, _, sdk, guard, op := authAdapterFixture()
			switch condition {
			case "nil":
				a.Guard = nil
			case "unknown":
				guard.deny = true
			case "callback-missing":
				guard.noCallback = true
			case "no-incarnation-lock":
				guard.noLock = true
			case "no-revoked-token-check":
				guard.noToken = true
			case "no-clock-sync":
				guard.noClock = true
			}
			_, err := a.Apply(context.Background(), op)
			require.Error(t, err)
			require.Zero(t, sdk.revokes)
		})
	}
}

func TestFirebaseDeletionReconcilesTimeoutAndAcknowledgementLoss(t *testing.T) {
	for _, action := range []string{"revoke", "delete"} {
		for _, result := range []string{"applied-lost-ack", "applied-unreadable", "not-applied", "ack-without-postcondition"} {
			t.Run(action+"/"+result, func(t *testing.T) {
				a, store, _, sdk, _, op := authAdapterFixture()
				if action == "delete" {
					_, err := a.Apply(context.Background(), op)
					require.NoError(t, err)
					op = fakeAuthAdvance(store, op, action, "firebase-auth")
				}
				sdk.mutationErr = context.DeadlineExceeded
				switch result {
				case "applied-unreadable":
					sdk.postWriteErr = context.DeadlineExceeded
				case "not-applied":
					sdk.applyMutation = false
				case "ack-without-postcondition":
					sdk.applyMutation = false
					sdk.mutationErr = nil
				}
				v, err := a.Apply(context.Background(), op)
				if result == "applied-lost-ack" {
					require.NoError(t, err)
					require.NoError(t, v.Validate(op, a.Clock()))
				} else {
					require.Error(t, err)
					require.Error(t, v.Validate(op, a.Clock()), "unknown never advances the workflow")
				}
				sdk.getErr, sdk.postWriteErr, sdk.mutationErr = nil, nil, nil
				sdk.applyMutation = true
				_, err = a.Apply(context.Background(), op)
				require.NoError(t, err)
				if result == "applied-lost-ack" || result == "applied-unreadable" {
					if action == "revoke" {
						require.Equal(t, 1, sdk.revokes)
					} else {
						require.Equal(t, 1, sdk.deletes)
					}
				}
			})
		}
	}
}

func TestFirebaseDeletionCutoffSecondAndUIDReuseCannotBecomeSuccess(t *testing.T) {
	a, store, _, sdk, guard, op := authAdapterFixture()
	sdk.revokeMillis = op.Execution.Cutoff.Unix() * 1000
	_, err := a.Apply(context.Background(), op)
	require.ErrorIs(t, err, supportdelete.ErrEvidence, "same-second revocation does not cover all cutoff-second tokens")
	sdk.revokeMillis += 1000
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	op = fakeAuthAdvance(store, op, "delete", "firebase-auth")
	sdk.onDelete = func() {
		locked := !guard.lifecycle.TryLock()
		if !locked {
			guard.lifecycle.Unlock()
		}
		require.True(t, locked, "UID lifecycle guard stays held across mutation and readback")
	}
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	old := store.snapshot.Identities[0]
	sdk.user = &auth.UserRecord{UserInfo: &auth.UserInfo{UID: old.UID}, UserMetadata: &auth.UserMetadata{CreationTimestamp: old.CreatedAtMillis + 1}, TenantID: old.TenantID}
	_, err = a.Apply(context.Background(), op)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	require.Equal(t, 1, sdk.deletes, "retry must not delete a newly created user reusing the UID")
	op = fakeAuthAdvance(store, op, "inspect", "firebase-auth")
	_, err = a.Apply(context.Background(), op)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	require.NotNil(t, sdk.user)
}

func TestFirebaseDeletionResumeRequiresOriginalSnapshotAfterMappingRemoval(t *testing.T) {
	a, store, inventory, sdk, _, op := authAdapterFixture()
	_, err := a.Apply(context.Background(), op)
	require.NoError(t, err)
	op = fakeAuthAdvance(store, op, "delete", "legacy-mappings")
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.True(t, store.snapshot.MappingsRemoved)
	a.Inventory = nil
	op = fakeAuthAdvance(store, op, "delete", "firebase-auth")
	_, err = a.Apply(context.Background(), op)
	require.NoError(t, err)
	require.Equal(t, 1, sdk.deletes)
	require.Equal(t, 1, inventory.calls)
	store.snapshot = nil
	_, err = a.Apply(context.Background(), op)
	require.ErrorIs(t, err, supportdelete.ErrEvidence, "missing mappings/snapshot never means no legacy UID")
}
