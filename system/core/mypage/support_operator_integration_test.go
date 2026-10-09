//go:build integration

package mypage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type syntheticOperatorAuthority struct {
	deny     bool
	evidence bool
}

func (a syntheticOperatorAuthority) Authorize(_ context.Context, intent OperatorIntent) (OperatorIdentity, error) {
	if a.deny {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	return OperatorIdentity{Subject: "synthetic-verified-operator", Environment: intent.Environment, ProjectID: intent.ProjectID}, nil
}

func (a syntheticOperatorAuthority) VerifyEvidence(_ context.Context, identity OperatorIdentity, intent OperatorIntent, action, delivery string) error {
	if !a.evidence || identity.Subject != "synthetic-verified-operator" || identity.Environment != intent.Environment || identity.ProjectID != intent.ProjectID || action != digest("synthetic-action") || delivery != digest("synthetic-delivery") {
		return ErrOperatorDenied
	}
	return nil
}

func syntheticOperator(client *firestore.Client) *FirestoreSupportOperator {
	return &FirestoreSupportOperator{Client: client, Environment: "development", ProjectID: "demo-youtube-study-space-ci", AuditKey: []byte("synthetic-audit-key-with-thirty-two-bytes"), Authority: syntheticOperatorAuthority{evidence: true}}
}

func operatorFixture(t *testing.T, purpose SupportPurpose) (*FirestoreSupportOperator, ReplyOperation, string) {
	t.Helper()
	_, authStore, _, _, now := authTestService(t)
	ctx := context.Background()
	intake := &FirestorePrivacyIntake{Client: authStore.Client, Environment: "development", IntakeSecret: []byte("synthetic-intake-key-with-thirty-two-bytes")}
	receipt, err := intake.Create(ctx, "UCsynthetic0000000000001", digest("operator-request-"+t.Name()+string(purpose)), purpose, "Synthetic private request", now)
	require.NoError(t, err)
	proof := digest("operator-proof-" + t.Name() + string(purpose))
	oauth := digest("operator-oauth-" + t.Name() + string(purpose))
	_, err = authStore.Client.Collection("support-requests").Doc(receipt.RequestRef).Update(ctx, []firestore.Update{{Path: "status", Value: "verified"}, {Path: "verifiedAt", Value: now}, {Path: "proofRef", Value: proof}, {Path: "oauthTransactionId", Value: oauth}, {Path: "challengeHash", Value: ""}, {Path: "challengeExpiresAt", Value: time.Time{}}})
	require.NoError(t, err)
	_, err = authStore.Client.Collection("oauth-transactions").Doc(oauth).Create(ctx, map[string]interface{}{"synthetic": true})
	require.NoError(t, err)
	op := syntheticOperator(authStore.Client)
	reply := ReplyOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: receipt.RequestRef, Purpose: purpose, OperationID: digest("operator-reply-" + t.Name() + string(purpose)), Action: "reply"}, ProofRef: proof, ExpectedRevision: 0, Body: "Synthetic reviewed response", At: now.Add(time.Second)}
	return op, reply, oauth
}

func TestOperatorReplyAuthorityIdempotenceAndConflicts(t *testing.T) {
	op, reply, _ := operatorFixture(t, SupportDisclosure)
	ctx := context.Background()
	denied := *op
	denied.Authority = nil
	require.ErrorIs(t, denied.Reply(ctx, reply), ErrOperatorDenied)
	denied = *op
	denied.Authority = syntheticOperatorAuthority{deny: true}
	require.ErrorIs(t, denied.Reply(ctx, reply), ErrOperatorDenied)
	wrong := reply
	wrong.Intent.Environment = "production"
	require.ErrorIs(t, op.Reply(ctx, wrong), ErrOperatorDenied)
	wrong = reply
	wrong.Intent.Purpose = SupportRevoke
	require.ErrorIs(t, op.Reply(ctx, wrong), ErrOperatorConflict)
	wrong = reply
	wrong.ProofRef = digest("other-proof")
	require.ErrorIs(t, op.Reply(ctx, wrong), ErrOperatorConflict)
	var outcomes [2]error
	var wg sync.WaitGroup
	for i := range outcomes {
		wg.Add(1)
		go func(i int) { defer wg.Done(); outcomes[i] = op.Reply(ctx, reply) }(i)
	}
	wg.Wait()
	for _, err := range outcomes {
		require.NoError(t, err)
	}
	require.NoError(t, op.Reply(ctx, reply)) // lost ACK
	changed := reply
	changed.Body = "Different synthetic response"
	require.ErrorIs(t, op.Reply(ctx, changed), ErrOperatorConflict)
	changed = reply
	changed.Intent.OperationID = digest("another-operation")
	require.ErrorIs(t, op.Reply(ctx, changed), ErrOperatorConflict)
	snap, err := op.Client.Collection("support-requests").Doc(reply.Intent.RequestRef).Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "verified", snap.Data()["status"])
	require.Equal(t, int64(1), snap.Data()["operatorRevision"])
	audit, err := op.auditRef(reply.Intent).Get(ctx)
	require.NoError(t, err)
	require.NotContains(t, audit.Data(), "body")
	require.NotContains(t, audit.Data(), "targetChannel")
	require.NotContains(t, audit.Data(), "proofRef")
}

func TestOperatorCompletionRequiresVerifiedEvidenceAndScrubs(t *testing.T) {
	op, reply, oauth := operatorFixture(t, SupportRevoke)
	ctx := context.Background()
	require.NoError(t, op.Reply(ctx, reply))
	before, err := op.Client.Collection("support-requests").Doc(reply.Intent.RequestRef).Get(ctx)
	require.NoError(t, err)
	requestID := before.Data()["requestId"].(string)
	completion := CompletionOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: reply.Intent.RequestRef, Purpose: SupportRevoke, OperationID: digest("operator-complete"), Action: "complete"}, ProofRef: reply.ProofRef, ReplyOperationID: reply.Intent.OperationID, ExpectedRevision: 1, ActionEvidence: digest("synthetic-action"), DeliveryAcknowledgement: digest("synthetic-delivery"), At: reply.At.Add(time.Second)}
	missing := completion
	missing.DeliveryAcknowledgement = ""
	require.ErrorIs(t, op.Complete(ctx, missing), ErrOperatorDenied)
	missing = completion
	missing.ActionEvidence = digest("unverified-action")
	require.ErrorIs(t, op.Complete(ctx, missing), ErrOperatorDenied)
	wrong := completion
	wrong.Intent.Purpose = SupportDelete
	require.ErrorIs(t, op.Complete(ctx, wrong), ErrOperatorDenied)
	require.NoError(t, op.Complete(ctx, completion))
	require.NoError(t, op.Complete(ctx, completion)) // lost ACK
	snap, err := op.Client.Collection("support-requests").Doc(reply.Intent.RequestRef).Get(ctx)
	require.NoError(t, err)
	require.True(t, simpleOperatorTerminal(snap))
	for _, key := range []string{"body", "operatorReply", "targetChannel", "proofRef", "oauthTransactionId", "submissionHash", "replyDigest", "replyOperationId"} {
		require.NotContains(t, snap.Data(), key)
	}
	for _, ref := range []*firestore.DocumentRef{op.Client.Collection("oauth-transactions").Doc(oauth)} {
		_, err = ref.Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	_, err = op.Client.Collection("support-request-ids").Doc(digest("development:" + requestID)).Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.ErrorIs(t, op.Reply(ctx, reply), ErrOperatorConflict)
	changed := completion
	changed.DeliveryAcknowledgement = digest("other-delivery")
	require.ErrorIs(t, op.Complete(ctx, changed), ErrOperatorDenied)
}

func TestOperatorReplyBlockedByDeletionClaim(t *testing.T) {
	s, start, _ := deletionFixture(t)
	ctx := context.Background()
	op := syntheticOperator(s.Client)
	reply := ReplyOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: start.Selector.RequestRef, Purpose: SupportDelete, OperationID: digest("delete-reply"), Action: "reply"}, ProofRef: start.Selector.ProofRef, Body: "Synthetic response", At: start.Now}
	_, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	require.True(t, errors.Is(op.Reply(ctx, reply), ErrOperatorConflict))
}

func TestOperatorAuditCollisionLeavesReplyAndReceiptUntouched(t *testing.T) {
	op, reply, _ := operatorFixture(t, SupportDisclosure)
	ctx := context.Background()
	_, err := op.auditRef(reply.Intent).Create(ctx, map[string]interface{}{"occupied": true})
	require.NoError(t, err)
	require.ErrorIs(t, op.Reply(ctx, reply), ErrOperatorConflict)
	snap, err := op.Client.Collection("support-requests").Doc(reply.Intent.RequestRef).Get(ctx)
	require.NoError(t, err)
	require.Equal(t, "verified", snap.Data()["status"])
	require.NotContains(t, snap.Data(), "operatorReply")
	require.NotContains(t, snap.Data(), "replyOperationId")
}

func TestOperatorParallelChangedPayloadHasOneWinner(t *testing.T) {
	op, reply, _ := operatorFixture(t, SupportDisclosure)
	other := reply
	other.Body = "Different synthetic reviewed response"
	ctx := context.Background()
	var results [2]error
	var wg sync.WaitGroup
	for i, candidate := range []ReplyOperation{reply, other} {
		wg.Add(1)
		go func(i int, candidate ReplyOperation) { defer wg.Done(); results[i] = op.Reply(ctx, candidate) }(i, candidate)
	}
	wg.Wait()
	successes, conflicts := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrOperatorConflict):
			conflicts++
		default:
			t.Fatalf("unexpected parallel result: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	snap, err := op.Client.Collection("support-requests").Doc(reply.Intent.RequestRef).Get(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), snap.Data()["operatorRevision"])
}
