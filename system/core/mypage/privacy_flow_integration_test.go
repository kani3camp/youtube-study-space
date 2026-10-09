//go:build integration

package mypage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/core/supportdelete"
)

// This fixture deliberately passes the same receipt through intake, the real
// one-use support OAuth transaction, status and the trusted operator boundary.
// Every identifier and credential here is synthetic and emulator-only.
type privacyFlow struct {
	client  *firestore.Client
	intake  *FirestorePrivacyIntake
	op      *FirestoreSupportOperator
	uid     string
	key     string
	receipt PrivacyIntakeReceipt
	proof   ConfirmResponse
	oauthID string
	now     time.Time
}

func newPrivacyFlow(t *testing.T, purpose SupportPurpose) privacyFlow {
	t.Helper()
	ctx := context.Background()
	auth, store, provider, minter, now := authTestService(t)
	auth.Support = &FirestoreSupportStore{Client: store.Client, Environment: "development"}
	uid := "UC" + digest(t.Name() + now.String())[:22]
	provider.channels[0].ID = uid
	intake := &FirestorePrivacyIntake{Client: store.Client, Environment: "development", IntakeSecret: []byte("synthetic-flow-intake-secret-thirty-two-bytes")}
	key := "synthetic-flow-" + string(purpose) + "-0001"
	receipt, err := intake.Create(ctx, uid, key, purpose, "Synthetic private request", now)
	require.NoError(t, err)
	// A lost intake ACK must reuse the same receipt and deadline.
	retry, err := intake.Create(ctx, uid, key, purpose, "Synthetic private request", now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, receipt, retry)
	_, err = intake.Status(ctx, uid, receipt.RequestRef)
	require.Equal(t, "SUPPORT_CHALLENGE_INVALID", errorCode(err))
	_, err = intake.Status(ctx, "UCsynthetic0000000000002", receipt.RequestRef)
	require.Equal(t, "SUPPORT_CHALLENGE_INVALID", errorCode(err))
	otherEnvironment := *intake
	otherEnvironment.Environment = "production"
	_, err = otherEnvironment.Status(ctx, uid, receipt.RequestRef)
	require.Equal(t, "SUPPORT_CHALLENGE_INVALID", errorCode(err))

	challenge := receipt.Challenge
	if purpose == SupportDelete {
		// Reissuing proof cannot move acceptedAt or the seven-day deadline.
		challenge, err = auth.Support.Reissue(ctx, receipt.RequestRef, now.Add(time.Minute))
		require.NoError(t, err)
		_, err = auth.Support.Resolve(ctx, receipt.Challenge, now.Add(time.Minute))
		require.Equal(t, "SUPPORT_CHALLENGE_INVALID", errorCode(err))
		retry, err = intake.Create(ctx, uid, key, purpose, "Synthetic private request", now.Add(2*time.Minute))
		require.NoError(t, err)
		require.Equal(t, receipt.AcceptedAt, retry.AcceptedAt)
		require.Equal(t, receipt.DeleteBy, retry.DeleteBy)
		require.Empty(t, retry.Challenge)
	}
	start, oauthID, err := auth.Start(ctx, "", StartRequest{PrivacyPolicyVersion: auth.Policy.Privacy, PrivacyAccepted: true, TermsVersion: auth.Policy.Terms, TermsAccepted: true, SupportChallenge: &challenge})
	require.NoError(t, err)
	redirect, err := url.Parse(start.AuthorizationURL)
	require.NoError(t, err)
	require.NoError(t, auth.Callback(ctx, oauthID, redirect.Query().Get("state"), "synthetic-flow-code", false))
	channel, err := auth.Channel(ctx, oauthID)
	require.NoError(t, err)
	require.Equal(t, "support", channel.Purpose)
	require.Equal(t, purpose, channel.SupportPurpose)
	proof, err := auth.ConfirmResult(ctx, oauthID, channel.ConfirmationRef)
	require.NoError(t, err)
	require.Equal(t, receipt.RequestRef, proof.SupportRequestRef)
	require.NotEqual(t, receipt.RequestRef, proof.RequestRef)
	require.Empty(t, proof.CustomToken)
	require.Zero(t, minter.calls.Load())
	_, err = auth.ConfirmResult(ctx, oauthID, channel.ConfirmationRef)
	require.Error(t, err, "fresh proof must be consumed once")
	_, err = intake.Status(ctx, uid, proof.RequestRef)
	require.Equal(t, "SUPPORT_CHALLENGE_INVALID", errorCode(err), "proof ref is not a status credential")
	return privacyFlow{client: store.Client, intake: intake, op: syntheticOperator(store.Client), uid: uid, key: key, receipt: receipt, proof: proof, oauthID: oauthID, now: now}
}

func (f privacyFlow) reply(purpose SupportPurpose) ReplyOperation {
	return ReplyOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: f.receipt.RequestRef, Purpose: purpose, OperationID: digest("flow-reply:" + string(purpose)), Action: "reply"}, ProofRef: f.proof.RequestRef, ExpectedRevision: 0, Body: "Synthetic reviewed response", At: f.now.Add(3 * time.Minute)}
}

// Action and delivery are separately acknowledged by the fake. Caller-supplied
// references cannot make either acknowledgement true.
type independentFlowAuthority struct {
	action   bool
	delivery bool
	digest   string
}

func (a *independentFlowAuthority) Authorize(_ context.Context, intent OperatorIntent) (OperatorIdentity, error) {
	return OperatorIdentity{Subject: "synthetic-verified-operator", Environment: intent.Environment, ProjectID: intent.ProjectID}, nil
}

func (a *independentFlowAuthority) VerifyEvidence(_ context.Context, _ OperatorIdentity, op CompletionOperation) (VerifiedCompletionEvidence, error) {
	if !a.action || !a.delivery || op.ActionEvidence != digest("synthetic-action") || op.DeliveryAcknowledgement != digest("synthetic-delivery") {
		return VerifiedCompletionEvidence{}, ErrOperatorDenied
	}
	return syntheticVerifiedCompletion(op, a.digest), nil
}

func assertPrivacyFlowTerminal(t *testing.T, f privacyFlow, purpose SupportPurpose, deleteFlow bool) {
	t.Helper()
	ctx := context.Background()
	snap, err := f.client.Collection("support-requests").Doc(f.receipt.RequestRef).Get(ctx)
	require.NoError(t, err)
	if deleteFlow {
		require.True(t, deletionComplete(snap))
		require.Equal(t, 4, len(snap.Data()))
		require.Equal(t, *f.receipt.DeleteBy, snap.Data()["deleteBy"])
	} else {
		require.True(t, simpleOperatorTerminal(snap))
		require.Equal(t, string(purpose), snap.Data()["purpose"])
	}
	require.Equal(t, f.receipt.AcceptedAt, snap.Data()["acceptedAt"])
	for _, key := range []string{"body", "operatorReply", "targetChannel", "proofRef", "oauthTransactionId", "challengeHash", "submissionHash", "replyDigest", "replyOperationId"} {
		require.NotContains(t, snap.Data(), key)
	}
	_, err = f.client.Collection("oauth-transactions").Doc(f.oauthID).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
	requestID := f.intake.keyed("request-id", f.uid, f.key)
	_, err = f.client.Collection("support-request-ids").Doc(digest("development:" + requestID)).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = f.intake.Status(ctx, f.uid, f.receipt.RequestRef)
	require.Equal(t, "SUPPORT_CHALLENGE_INVALID", errorCode(err))
	audits, err := f.client.Collection(supportOperatorAudit).Documents(ctx).GetAll()
	require.NoError(t, err)
	for _, audit := range audits {
		encoded := fmt.Sprint(audit.Data())
		for _, private := range []string{f.uid, f.receipt.RequestRef, f.proof.RequestRef, "Synthetic private request", "Synthetic reviewed response"} {
			require.NotContains(t, encoded, private, "terminal audit retained private flow content")
		}
	}
}

func TestPrivacyFlowIntakeProofReplyIndependentAcksAndCompletion(t *testing.T) {
	for _, purpose := range []SupportPurpose{SupportDisclosure, SupportRevoke} {
		t.Run(string(purpose), func(t *testing.T) {
			ctx := context.Background()
			f := newPrivacyFlow(t, purpose)
			reply := f.reply(purpose)
			wrong := reply
			wrong.Intent.Environment = "production"
			require.ErrorIs(t, f.op.Reply(ctx, wrong), ErrOperatorDenied)
			wrong = reply
			wrong.Intent.Purpose = SupportDelete
			require.ErrorIs(t, f.op.Reply(ctx, wrong), ErrOperatorConflict)
			wrong = reply
			wrong.ProofRef = digest("wrong-proof")
			require.ErrorIs(t, f.op.Reply(ctx, wrong), ErrOperatorConflict)
			wrong = reply
			wrong.ExpectedRevision = 1
			require.ErrorIs(t, f.op.Reply(ctx, wrong), ErrOperatorConflict)
			require.NoError(t, f.op.Reply(ctx, reply))
			require.NoError(t, f.op.Reply(ctx, reply), "lost reply ACK")
			view, err := f.intake.Status(ctx, f.uid, f.receipt.RequestRef)
			require.NoError(t, err)
			require.Equal(t, reply.Body, view.Reply)
			require.Equal(t, f.receipt.AcceptedAt, view.AcceptedAt)
			completion := CompletionOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: f.receipt.RequestRef, Purpose: purpose, OperationID: digest("flow-complete:" + string(purpose)), Action: "complete"}, ProofRef: f.proof.RequestRef, ReplyOperationID: reply.Intent.OperationID, ExpectedRevision: 1, ActionEvidence: digest("synthetic-action"), DeliveryAcknowledgement: digest("synthetic-delivery"), At: reply.At.Add(time.Second)}
			snap, err := f.client.Collection("support-requests").Doc(f.receipt.RequestRef).Get(ctx)
			require.NoError(t, err)
			ack := &independentFlowAuthority{digest: snap.Data()["replyDigest"].(string)}
			f.op.Authority = ack
			require.ErrorIs(t, f.op.Complete(ctx, completion), ErrOperatorDenied, "proposal is not action or delivery evidence")
			ack.action = true
			require.ErrorIs(t, f.op.Complete(ctx, completion), ErrOperatorDenied, "action alone cannot complete")
			ack.delivery = true
			wrongCompletion := completion
			wrongCompletion.Intent.Purpose = SupportDelete
			require.ErrorIs(t, f.op.Complete(ctx, wrongCompletion), ErrOperatorDenied)
			wrongCompletion = completion
			wrongCompletion.ExpectedRevision = 2
			require.ErrorIs(t, f.op.Complete(ctx, wrongCompletion), ErrOperatorConflict)
			require.NoError(t, f.op.Complete(ctx, completion))
			ack.action, ack.delivery = false, false
			require.NoError(t, f.op.Complete(ctx, completion), "lost completion ACK recovers from durable audit")
			assertPrivacyFlowTerminal(t, f, purpose, false)
			require.ErrorIs(t, f.op.Reply(ctx, reply), ErrOperatorConflict)
		})
	}
}

func TestPrivacyFlowDeleteClaimConflictRecoveryAndFinalize(t *testing.T) {
	ctx := context.Background()
	f := newPrivacyFlow(t, SupportDelete)
	reply := f.reply(SupportDelete)
	require.NoError(t, f.op.Reply(ctx, reply))
	view, err := f.intake.Status(ctx, f.uid, f.receipt.RequestRef)
	require.NoError(t, err)
	require.Equal(t, f.receipt.DeleteBy, view.DeleteBy)
	store := &FirestoreDeletionStore{Client: f.client}
	selector := supportdelete.Selector{Target: supportdelete.Target{Environment: "development", ProjectID: "demo-youtube-study-space-ci", ChannelID: f.uid}, RequestRef: f.receipt.RequestRef, ExecutionRef: digest("flow-delete-execution"), ProofRef: f.proof.RequestRef}
	start := supportdelete.Start{Selector: selector, Manifest: supportdelete.Manifest{Ref: digest("synthetic-flow-complete-fleet"), Mode: "emulator"}, OwnerRef: digest("flow-delete-owner"), Now: reply.At.Add(time.Second)}
	e, err := store.Acquire(ctx, start)
	require.NoError(t, err)
	require.Equal(t, f.receipt.AcceptedAt, e.AcceptedAt)
	require.Equal(t, *f.receipt.DeleteBy, e.DeleteBy)
	late := reply
	late.Intent.OperationID = digest("late-reply-after-claim")
	require.ErrorIs(t, f.op.Reply(ctx, late), ErrOperatorConflict)
	completion := CompletionOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: f.receipt.RequestRef, Purpose: SupportDelete, OperationID: digest("wrong-delete-complete"), Action: "complete"}, ProofRef: f.proof.RequestRef, ReplyOperationID: reply.Intent.OperationID, ExpectedRevision: 1, ActionEvidence: digest("synthetic-action"), DeliveryAcknowledgement: digest("synthetic-delivery"), At: start.Now}
	require.ErrorIs(t, f.op.Complete(ctx, completion), ErrOperatorDenied, "delete must use fenced Finalize")
	wrong := start
	wrong.Selector.Target.ChannelID = "UCsynthetic0000000000002"
	_, err = store.Acquire(ctx, wrong)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	wrong = start
	wrong.Selector.ProofRef = digest("wrong-proof")
	_, err = store.Acquire(ctx, wrong)
	require.ErrorIs(t, err, supportdelete.ErrConflict)

	// Compose the existing fake Auth, BigQuery and derived/vendor adapters on
	// this one execution. Other scopes use the repository's emulator fake guard.
	now := start.Now.Add(time.Second)
	authAdapter, _, sdk, _ := authEmulatorFixture(t, store, e, f.oauthID, now)
	var bqAdapter *BigQueryDeletionEffects
	var bqRemote *fakeBigQueryExecutor
	var derivedAdapter *DerivedDeletionEffects
	var vendor *derivedFake
	firestoreAdapter := &FirestoreDeletionEffects{Store: store, RestoreGuard: deletionFixtureEffects{}, Clock: func() time.Time { return now }}
	for e.Cursor < len(supportdelete.Steps()) {
		operation := supportdelete.NewOperation(e, now)
		var evidence supportdelete.Evidence
		switch operation.Step.Scope {
		case "firebase-auth":
			evidence, err = authAdapter.Apply(ctx, operation)
		case "bigquery":
			if operation.Step.Action != "delete" && operation.Step.Action != "inspect" {
				evidence, err = (deletionFixtureEffects{}).Apply(ctx, operation)
				break
			}
			if bqAdapter == nil {
				bqAdapter, _, _, bqRemote, _, _ = bigQueryFixture(e, now)
				bqAdapter.Store = store
			}
			evidence, err = bqAdapter.Apply(ctx, operation)
		case "derived-vendor":
			if operation.Step.Action != "delete" && operation.Step.Action != "inspect" {
				evidence, err = (deletionFixtureEffects{}).Apply(ctx, operation)
				break
			}
			if derivedAdapter == nil {
				derivedAdapter, _, vendor, _ = derivedFixture()
				fixture := derivedAdapter.Inventory.(derivedFixtureInventory)
				snapshot := fixture.snapshot
				snapshot.Execution = e
				snapshot.CaptureOperationID = operation.ID
				snapshot.CapturedAt = now
				for i := range snapshot.Sources {
					for j := range snapshot.Sources[i].Items {
						item := &snapshot.Sources[i].Items[j]
						for k := range item.OwnerChannels {
							if item.OwnerChannels[k] == "UCsynthetic0000000000001" {
								item.OwnerChannels[k] = f.uid
							}
						}
						vendor.objects[item.ObjectID] = &derivedObject{item: *item, exists: true, owners: append([]string(nil), item.OwnerChannels...)}
					}
				}
				snapshot.InventoryRef = derivedInventoryRef(snapshot)
				derivedAdapter.Store = store
				derivedAdapter.Inventory = derivedFixtureInventory{snapshot}
				derivedAdapter.Guard = derivedFixtureGuard{now: now}
				derivedAdapter.Clock = func() time.Time { return now }
				vendor.lostAck = true
			}
			evidence, err = derivedAdapter.Apply(ctx, operation)
		case "seat-state", "firestore-primary", "firestore-web", "firestore-oauth", "firestore-support-relations":
			evidence, err = firestoreAdapter.Apply(ctx, operation)
		default:
			evidence, err = (deletionFixtureEffects{}).Apply(ctx, operation)
		}
		require.NoError(t, err, "delete step %d: %s/%s", e.Cursor, operation.Step.Action, operation.Step.Scope)
		old := e
		e, err = store.Commit(ctx, e, evidence, now)
		require.NoError(t, err)
		if operation.Step.Action == "delete" && operation.Step.Scope == "bigquery" {
			_, err = store.Commit(ctx, old, evidence, now)
			require.ErrorIs(t, err, supportdelete.ErrConflict, "lost checkpoint ACK cannot reuse old fence")
			resumed, resumeErr := store.Acquire(ctx, start)
			require.NoError(t, resumeErr)
			require.Equal(t, e, resumed)
			newOwner := digest("flow-recovered-owner")
			recovery := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: newOwner, ManifestRef: e.Manifest.Ref, TraceRef: digest("synthetic-stopped-worker"), Revision: e.Revision, Generation: e.Generation, Stopped: true, ObservedAt: now}
			require.NoError(t, store.Recover(ctx, recovery, now))
			start.OwnerRef = newOwner
			e, err = store.Acquire(ctx, start)
			require.NoError(t, err, "fenced worker recovery must retain the execution and deadline")
			require.Equal(t, f.receipt.AcceptedAt, e.AcceptedAt)
			require.Equal(t, *f.receipt.DeleteBy, e.DeleteBy)
		}
	}
	require.Equal(t, 1, sdk.revokes)
	require.Equal(t, 1, sdk.deletes)
	require.Equal(t, 5, bqRemote.submitCalls)
	for _, rows := range bqRemote.rows {
		for _, row := range rows {
			require.NotEqual(t, f.uid, row["user_id"])
			require.NotEqual(t, f.uid, row["author_channel_id"])
		}
	}
	require.False(t, vendor.objects["synthetic-response"].exists)
	require.Equal(t, []string{"UCsynthetic0000000000002"}, vendor.objects["synthetic-message"].owners)
	other, err := (&serviceaccess.FirestoreStore{Client: f.client}).Read(ctx, "UCsynthetic0000000000002")
	require.NoError(t, err)
	require.False(t, other.Control.PrivacyDeletion.Active)
	require.NoError(t, store.Finalize(ctx, e, now))
	assertPrivacyFlowTerminal(t, f, SupportDelete, true)
	_, err = f.client.Collection(supportDeleteClaims).Doc(f.proof.RequestRef).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = store.Acquire(ctx, start)
	require.True(t, errors.Is(err, supportdelete.ErrCompleted))
}
