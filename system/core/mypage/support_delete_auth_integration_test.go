//go:build integration

package mypage

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/core/supportdelete"
)

func authEmulatorFixture(t *testing.T, s *FirestoreDeletionStore, e supportdelete.Execution, oauthRef string, now time.Time) (*FirebaseDeletionEffects, *fakeAuthInventory, *fakeDeletionSDK, []*firestore.DocumentRef) {
	t.Helper()
	ctx := context.Background()
	uid := "synthetic-old-" + digest(e.Selector.ExecutionRef)[:24]
	owner := s.Client.Collection("mypage-youtube-channel-owners").Doc(e.Selector.Target.ChannelID)
	legacy := s.Client.Collection("mypage-users").Doc(uid)
	_, err := owner.Set(ctx, map[string]interface{}{"firebase-uid": uid, "synthetic": true})
	require.NoError(t, err)
	_, err = legacy.Set(ctx, map[string]interface{}{"synthetic-private": "legacy record without firebase-uid field"})
	require.NoError(t, err)
	currentDoc, err := s.Client.Collection("support-requests").Doc(e.Selector.RequestRef).Get(ctx)
	require.NoError(t, err)
	var current SupportRequest
	require.NoError(t, currentDoc.DataTo(&current))
	currentIndex := s.Client.Collection("support-request-ids").Doc(digest(current.Environment + ":" + current.RequestID))
	refs := []*firestore.DocumentRef{owner, legacy, currentDoc.Ref, s.Client.Collection("oauth-transactions").Doc(oauthRef), currentIndex}
	op := supportdelete.NewOperation(e, now)
	identity := AuthOwnedIdentity{UID: uid, TenantID: "synthetic-tenant", CreatedAtMillis: e.Cutoff.Add(-time.Hour).UnixMilli(), OwnerChannels: []string{e.Selector.Target.ChannelID}}
	snapshot := AuthOwnershipSnapshot{SchemaVersion: 1, Execution: e, CaptureOperationID: op.ID, TraceRef: digest("synthetic-exhaustive-ownership:" + e.Selector.ExecutionRef), CapturedAt: now, Complete: true, Identities: []AuthOwnedIdentity{identity}}
	for _, ref := range refs {
		doc, err := ref.Get(ctx)
		require.NoError(t, err)
		fingerprint, err := AuthOwnershipFingerprint(doc.Data())
		require.NoError(t, err)
		d := AuthOwnedDocument{Collection: ref.Parent.ID, ID: ref.ID, Fingerprint: fingerprint, OwnerChannels: []string{e.Selector.Target.ChannelID}}
		if ref == owner || ref == legacy {
			snapshot.Mappings = append(snapshot.Mappings, d)
		} else {
			snapshot.Related = append(snapshot.Related, d)
		}
	}
	inventory := &fakeAuthInventory{snapshot: snapshot}
	sdk := &fakeDeletionSDK{user: &auth.UserRecord{UserInfo: &auth.UserInfo{UID: uid}, UserMetadata: &auth.UserMetadata{CreationTimestamp: identity.CreatedAtMillis}, TenantID: identity.TenantID}, applyMutation: true, revokeMillis: now.Unix() * 1000}
	adapter := &FirebaseDeletionEffects{Store: s, Inventory: inventory, Guard: &fakeAuthGuard{}, Clock: func() time.Time { return now }, Scopes: []FirebaseDeletionScope{{ProjectID: e.Selector.Target.ProjectID, TenantID: identity.TenantID, Client: sdk, UserNotFound: func(err error) bool { return errors.Is(err, errFakeAuthUserMissing) }}}}
	t.Cleanup(func() {
		for _, ref := range []*firestore.DocumentRef{owner, legacy, s.Client.Collection(supportDeleteAuthOwnership).Doc(e.Selector.ExecutionRef)} {
			_, err := ref.Delete(ctx)
			require.NoError(t, err)
		}
	})
	return adapter, inventory, sdk, refs
}

func inventoryAppendDocument(t *testing.T, inventory *fakeAuthInventory, ref *firestore.DocumentRef) {
	t.Helper()
	doc, err := ref.Get(context.Background())
	require.NoError(t, err)
	fingerprint, err := AuthOwnershipFingerprint(doc.Data())
	require.NoError(t, err)
	inventory.snapshot.Related = append(inventory.snapshot.Related, AuthOwnedDocument{Collection: ref.Parent.ID, ID: ref.ID, Fingerprint: fingerprint, OwnerChannels: []string{inventory.snapshot.Execution.Selector.Target.ChannelID}})
}

func TestFirebaseDeletionEmulatorDurableSnapshotMappingRemovalAndLostAckResume(t *testing.T) {
	s, start, oauthRef := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	now := start.Now.Add(2*time.Second + 123*time.Nanosecond)
	a, inventory, sdk, refs := authEmulatorFixture(t, s, e, oauthRef, now)
	op := supportdelete.NewOperation(e, now)
	v, err := a.Apply(ctx, op)
	require.NoError(t, err)
	e, err = s.Commit(ctx, e, v, now)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, 10, now)
	op = supportdelete.NewOperation(e, now)
	v, err = a.Apply(ctx, op)
	require.NoError(t, err)
	for _, ref := range refs[:2] {
		_, err := ref.Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	_, err = a.Apply(ctx, op)
	require.NoError(t, err, "mapping delete acknowledgement loss is reconciled by the committed snapshot")
	e, err = s.Commit(ctx, e, v, now)
	require.NoError(t, err)
	a.Inventory = nil
	e = advanceDeletion(t, s, e, 15, now)
	op = supportdelete.NewOperation(e, now)
	sdk.mutationErr, sdk.postWriteErr = context.DeadlineExceeded, context.DeadlineExceeded
	v, err = a.Apply(ctx, op)
	require.ErrorIs(t, err, supportdelete.ErrUnavailable)
	_, err = s.Commit(ctx, e, v, now)
	require.ErrorIs(t, err, supportdelete.ErrEvidence)
	sdk.getErr, sdk.postWriteErr, sdk.mutationErr = nil, nil, nil
	v, err = a.Apply(ctx, op)
	require.NoError(t, err)
	require.Equal(t, 1, sdk.deletes)
	require.Equal(t, 1, inventory.calls)
	e, err = s.Commit(ctx, e, v, now)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, 22, now)
	_, err = a.Apply(ctx, supportdelete.NewOperation(e, now))
	require.NoError(t, err, "Auth absence uses old identity even after legacy mapping removal")
	_, err = s.Client.Collection(supportDeleteAuthOwnership).Doc(e.Selector.ExecutionRef).Delete(ctx)
	require.NoError(t, err)
	_, err = a.Apply(ctx, supportdelete.NewOperation(e, now))
	require.ErrorIs(t, err, supportdelete.ErrEvidence, "missing mappings never manufacture an empty inventory")
}

func TestFirebaseDeletionEmulatorReverseSharedAndAmbiguousOwnershipRollback(t *testing.T) {
	for _, condition := range []string{"reverse-shared-owner", "omitted-legacy-account", "mapping-drift", "shared-related-owner", "new-incarnation"} {
		t.Run(condition, func(t *testing.T) {
			s, start, oauthRef := deletionFixture(t)
			ctx := context.Background()
			e, err := s.Acquire(ctx, start)
			require.NoError(t, err)
			now := start.Now.Add(2 * time.Second)
			a, inventory, sdk, refs := authEmulatorFixture(t, s, e, oauthRef, now)
			switch condition {
			case "reverse-shared-owner":
				other := s.Client.Collection("mypage-youtube-channel-owners").Doc("UC" + digest(start.Selector.ExecutionRef + "other")[:22])
				_, err := other.Set(ctx, map[string]interface{}{"firebase-uid": inventory.snapshot.Identities[0].UID})
				require.NoError(t, err)
				t.Cleanup(func() { _, err := other.Delete(ctx); require.NoError(t, err) })
			case "omitted-legacy-account":
				inventory.snapshot.Mappings = inventory.snapshot.Mappings[:1]
			case "mapping-drift":
				_, err := refs[0].Update(ctx, []firestore.Update{{Path: "firebase-uid", Value: "synthetic-new-owner-uid"}})
				require.NoError(t, err)
			case "shared-related-owner":
				inventory.snapshot.Related[0].OwnerChannels = append(inventory.snapshot.Related[0].OwnerChannels, "UCsynthetic0000000000002")
			case "new-incarnation":
				sdk.user.UserMetadata.CreationTimestamp++
			}
			_, err = a.Apply(ctx, supportdelete.NewOperation(e, now))
			require.Error(t, err)
			_, err = s.Client.Collection(supportDeleteAuthOwnership).Doc(e.Selector.ExecutionRef).Get(ctx)
			require.Equal(t, codes.NotFound, status.Code(err), "failed inventory capture rolls back temporary state")
			for _, ref := range refs {
				_, err := ref.Get(ctx)
				require.NoError(t, err, "no attributable record may be deleted on ambiguity")
			}
			require.Zero(t, sdk.revokes)
			require.Zero(t, sdk.deletes)
		})
	}
}

func TestFirebaseDeletionEmulatorRelatedClosurePreservesCurrentAndOtherChannel(t *testing.T) {
	s, start, oauthRef := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	now := start.Now.Add(2 * time.Second)
	a, inventory, _, currentRefs := authEmulatorFixture(t, s, e, oauthRef, now)
	oldRequest := digest(e.Selector.ExecutionRef + ":old-receipt")
	oldOAuth := digest(e.Selector.ExecutionRef + ":old-oauth")
	oldReceipt := s.Client.Collection("support-requests").Doc(oldRequest)
	oldRefs := []*firestore.DocumentRef{oldReceipt, s.Client.Collection("oauth-transactions").Doc(oldOAuth), s.Client.Collection("support-challenges").Doc(digest(oldRequest + "challenge")), s.Client.Collection("support-request-ids").Doc(digest(oldRequest + "index"))}
	_, err = oldReceipt.Set(ctx, SupportRequest{RequestID: "synthetic-old-unowned", Environment: "development", Purpose: SupportDelete, Status: "verified", AcceptedAt: e.AcceptedAt, TargetChannel: e.Selector.Target.ChannelID, OAuthTransactionID: oldOAuth, ProofRef: digest(oldRequest + "unclaimed-proof")})
	require.NoError(t, err)
	_, err = oldRefs[1].Set(ctx, map[string]interface{}{"support": map[string]interface{}{"requestRef": oldRequest}, "channel": map[string]interface{}{"channelId": e.Selector.Target.ChannelID}})
	require.NoError(t, err)
	for _, ref := range oldRefs[2:] {
		_, err := ref.Set(ctx, map[string]interface{}{"requestRef": oldRequest, "environment": "development"})
		require.NoError(t, err)
	}
	for _, ref := range oldRefs {
		inventoryAppendDocument(t, inventory, ref)
	}
	other := s.Client.Collection("support-requests").Doc(digest(oldRequest + "unrelated-channel"))
	_, err = other.Set(ctx, map[string]interface{}{"targetChannel": "UCsynthetic0000000000002", "synthetic-private": true})
	require.NoError(t, err)
	otherExecution := s.Client.Collection(supportDeleteExecutions).Doc(digest(oldRequest + "active-execution"))
	otherClaim := s.Client.Collection(supportDeleteClaims).Doc(digest(oldRequest + "unclaimed-proof"))
	t.Cleanup(func() {
		for _, ref := range append(oldRefs, other, otherExecution, otherClaim) {
			_, err := ref.Delete(ctx)
			require.NoError(t, err)
		}
	})
	v, err := a.Apply(ctx, supportdelete.NewOperation(e, now))
	require.NoError(t, err)
	e, err = s.Commit(ctx, e, v, now)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, 9, now)
	op := supportdelete.NewOperation(e, now)
	for _, active := range []*firestore.DocumentRef{otherExecution, otherClaim} {
		data := map[string]interface{}{"requestRef": oldRequest, "executionRef": otherExecution.ID}
		if active == otherExecution {
			data = map[string]interface{}{"selector": map[string]interface{}{"target": map[string]interface{}{"channelID": e.Selector.Target.ChannelID}}}
		}
		_, err := active.Set(ctx, data)
		require.NoError(t, err)
		_, err = a.Apply(ctx, op)
		require.Error(t, err)
		for _, ref := range append(oldRefs, active) {
			_, err := ref.Get(ctx)
			require.NoError(t, err, "active owned case blocks the entire related deletion transaction")
		}
		_, err = active.Delete(ctx)
		require.NoError(t, err)
	}
	_, err = a.Apply(ctx, op)
	require.NoError(t, err)
	_, err = a.Apply(ctx, op)
	require.NoError(t, err, "same operation reconciles already absent related records")
	for _, ref := range oldRefs {
		_, err := ref.Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	for _, ref := range append(currentRefs, other, s.Client.Collection(supportDeleteClaims).Doc(e.Selector.ProofRef)) {
		_, err := ref.Get(ctx)
		require.NoError(t, err, "current receipt/proof and unrelated channel survive")
	}
}

func TestFirebaseDeletionEmulatorSnapshotFencesRecoveryGenerationAndTerminalCleanup(t *testing.T) {
	s, start, oauthRef := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	now := start.Now.Add(2 * time.Second)
	a, _, _, _ := authEmulatorFixture(t, s, e, oauthRef, now)
	v, err := a.Apply(ctx, supportdelete.NewOperation(e, now))
	require.NoError(t, err)
	old := e
	e, err = s.Commit(ctx, e, v, now)
	require.NoError(t, err)
	_, err = s.LoadAuthOwnership(ctx, old)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	controls := &serviceaccess.FirestoreStore{Client: s.Client}
	_, err = controls.Change(ctx, e.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "SECURITY", Reference: digest("synthetic-auth-moderation")}, now)
	require.NoError(t, err)
	e, err = s.Refresh(ctx, e, now)
	require.NoError(t, err)
	snapshot, err := s.LoadAuthOwnership(ctx, e)
	require.NoError(t, err)
	require.Equal(t, e, snapshot.Execution)
	proof := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: digest(e.OwnerRef + ":new"), ManifestRef: e.Manifest.Ref, TraceRef: digest("synthetic-stopped-auth-worker"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: now}
	require.NoError(t, s.Recover(ctx, proof, now))
	start.OwnerRef, start.Now = proof.NewOwnerRef, now
	e, err = s.Acquire(ctx, start)
	require.NoError(t, err)
	snapshot, err = s.LoadAuthOwnership(ctx, e)
	require.NoError(t, err)
	require.Equal(t, e, snapshot.Execution)
	e = advanceDeletion(t, s, e, len(supportdelete.Steps()), now)
	ref := s.Client.Collection("support-requests").Doc(e.Selector.RequestRef)
	_, err = ref.Update(ctx, []firestore.Update{{Path: "proofRef", Value: digest("synthetic-drifted-proof")}})
	require.NoError(t, err)
	require.ErrorIs(t, s.Finalize(ctx, e, now), supportdelete.ErrConflict)
	_, err = s.LoadAuthOwnership(ctx, e)
	require.NoError(t, err, "failed terminal transaction retains snapshot for same-execution retry")
	_, err = ref.Update(ctx, []firestore.Update{{Path: "proofRef", Value: e.Selector.ProofRef}})
	require.NoError(t, err)
	require.NoError(t, s.Finalize(ctx, e, now))
	_, err = s.Client.Collection(supportDeleteAuthOwnership).Doc(e.Selector.ExecutionRef).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err), "raw ownership snapshot is removed atomically with terminal receipt")
	doc, err := ref.Get(ctx)
	require.NoError(t, err)
	require.True(t, deletionComplete(doc))
	require.Len(t, doc.Data(), 4)
	control, err := controls.Read(ctx, e.Selector.Target.ChannelID)
	require.NoError(t, err)
	require.True(t, control.Control.Moderation.Active)
	require.False(t, control.Control.PrivacyDeletion.Active)
}
