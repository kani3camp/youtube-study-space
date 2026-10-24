//go:build integration

package mypage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type syntheticOperatorAuthority struct {
	deny             bool
	evidence         bool
	denyVerification bool
	verified         VerifiedCompletionEvidence
	verifyCalls      *atomic.Int64
}

func (a syntheticOperatorAuthority) Authorize(_ context.Context, intent OperatorIntent) (OperatorIdentity, error) {
	if a.deny {
		return OperatorIdentity{}, ErrOperatorDenied
	}
	return OperatorIdentity{Subject: "synthetic-verified-operator", Environment: intent.Environment, ProjectID: intent.ProjectID}, nil
}

func (a syntheticOperatorAuthority) VerifyEvidence(_ context.Context, identity OperatorIdentity, op CompletionOperation) (VerifiedCompletionEvidence, error) {
	if a.verifyCalls != nil {
		a.verifyCalls.Add(1)
	}
	if a.denyVerification || !a.evidence || identity.Subject != "synthetic-verified-operator" || identity.Environment != op.Intent.Environment || identity.ProjectID != op.Intent.ProjectID || op.ActionEvidence != digest("synthetic-action") || op.DeliveryAcknowledgement != digest("synthetic-delivery") {
		return VerifiedCompletionEvidence{}, ErrOperatorDenied
	}
	return a.verified, nil
}

func syntheticVerifiedCompletion(op CompletionOperation, replyDigest string) VerifiedCompletionEvidence {
	return VerifiedCompletionEvidence{
		Actor: "synthetic-verified-operator", Environment: op.Intent.Environment, ProjectID: op.Intent.ProjectID,
		RequestRef: op.Intent.RequestRef, Purpose: op.Intent.Purpose, OperationID: op.Intent.OperationID,
		ProofRef: op.ProofRef, ReplyOperationID: op.ReplyOperationID, ExpectedRevision: op.ExpectedRevision,
		ReplyDigest: replyDigest, ActionEvidence: op.ActionEvidence, DeliveryAcknowledgement: op.DeliveryAcknowledgement,
	}
}

func syntheticOperator(client *firestore.Client) *FirestoreSupportOperator {
	return &FirestoreSupportOperator{Client: client, Environment: "development", ProjectID: "demo-youtube-study-space-ci", AuditKey: []byte("synthetic-audit-key-with-thirty-two-bytes"), Authority: syntheticOperatorAuthority{evidence: true}}
}

type operatorTestFixture struct {
	operator  *FirestoreSupportOperator
	reply     ReplyOperation
	oauth     string
	documents []*firestore.DocumentRef
}

// Each invocation owns its records, including invocations with the same t.Name.
// The returned operations stay fixed for idempotent retries within that fixture.
func newOperatorFixture(t *testing.T, purpose SupportPurpose) *operatorTestFixture {
	t.Helper()
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	namespace, err := opaque()
	require.NoError(t, err)
	uid := "UC" + namespace[:22]
	key := digest("operator-request:" + namespace)
	now := time.Now().UTC().Truncate(time.Second)
	intake := &FirestorePrivacyIntake{Client: client, Environment: "development", IntakeSecret: []byte("synthetic-intake-key-with-thirty-two-bytes")}
	requestRef := intake.keyed("ref", uid, key)
	requestID := intake.keyed("request-id", uid, key)
	proof := digest("operator-proof:" + namespace)
	oauth := digest("operator-oauth:" + namespace)
	op := syntheticOperator(client)
	reply := ReplyOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: requestRef, Purpose: purpose, OperationID: digest("operator-reply:" + namespace), Action: "reply"}, ProofRef: proof, ExpectedRevision: 0, Body: "Synthetic reviewed response", At: now.Add(time.Second)}
	f := &operatorTestFixture{operator: op, reply: reply, oauth: oauth, documents: []*firestore.DocumentRef{
		client.Collection("support-requests").Doc(requestRef),
		client.Collection("support-request-ids").Doc(digest("development:" + requestID)),
		client.Collection("support-challenges").Doc(digest(intake.keyed("challenge", uid, key))),
		client.Collection("oauth-transactions").Doc(oauth),
		op.auditRef(reply.Intent), // The collision test's occupied record has no binding.
	}}
	// LIFO cleanup runs before client.Close, even if fixture setup/assertions fail.
	t.Cleanup(func() { f.cleanup(t) })
	receipt, err := intake.Create(ctx, uid, key, purpose, "Synthetic private request", now)
	require.NoError(t, err)
	require.Equal(t, requestRef, receipt.RequestRef)
	_, err = client.Collection("support-requests").Doc(requestRef).Update(ctx, []firestore.Update{{Path: "status", Value: "verified"}, {Path: "verifiedAt", Value: now}, {Path: "proofRef", Value: proof}, {Path: "oauthTransactionId", Value: oauth}, {Path: "challengeHash", Value: ""}, {Path: "challengeExpiresAt", Value: time.Time{}}})
	require.NoError(t, err)
	_, err = client.Collection("oauth-transactions").Doc(oauth).Create(ctx, map[string]interface{}{"synthetic": true})
	require.NoError(t, err)
	return f
}

func (f *operatorTestFixture) cleanup(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Only audits bound to this unique request are eligible; never reset a DB or
	// delete an entire collection. Capture both reply and terminal completion.
	audits, err := f.operator.Client.Collection(supportOperatorAudit).Where("binding", "==", f.operator.mac("request", f.reply.Intent.RequestRef)).Documents(ctx).GetAll()
	if err != nil {
		t.Errorf("read fixture audits for cleanup: %v", err)
	}
	refs := append([]*firestore.DocumentRef(nil), f.documents...)
	for _, audit := range audits {
		refs = append(refs, audit.Ref)
	}
	for _, ref := range refs {
		if _, err := ref.Delete(ctx); err != nil {
			t.Errorf("delete fixture document: %v", err)
		}
	}
}

func operatorFixture(t *testing.T, purpose SupportPurpose) (*FirestoreSupportOperator, ReplyOperation, string) {
	t.Helper()
	f := newOperatorFixture(t, purpose)
	return f.operator, f.reply, f.oauth
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
	completion := CompletionOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: reply.Intent.RequestRef, Purpose: SupportRevoke, OperationID: digest("operator-complete:" + reply.Intent.OperationID), Action: "complete"}, ProofRef: reply.ProofRef, ReplyOperationID: reply.Intent.OperationID, ExpectedRevision: 1, ActionEvidence: digest("synthetic-action"), DeliveryAcknowledgement: digest("synthetic-delivery"), At: reply.At.Add(time.Second)}
	completion.ReplyDigest = before.Data()["replyDigest"].(string)
	var verifyCalls atomic.Int64
	op.Authority = syntheticOperatorAuthority{evidence: true, verified: syntheticVerifiedCompletion(completion, before.Data()["replyDigest"].(string)), verifyCalls: &verifyCalls}
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
	require.Equal(t, int64(2), verifyCalls.Load()) // invalid evidence and first successful attempt
	auditsBefore, err := op.Client.Collection(supportOperatorAudit).Documents(ctx).GetAll()
	require.NoError(t, err)
	op.Authority = syntheticOperatorAuthority{evidence: true, denyVerification: true, verifyCalls: &verifyCalls}
	require.NoError(t, op.Complete(ctx, completion)) // lost ACK after verifier failure
	require.Equal(t, int64(2), verifyCalls.Load())
	auditsAfter, err := op.Client.Collection(supportOperatorAudit).Documents(ctx).GetAll()
	require.NoError(t, err)
	require.Len(t, auditsAfter, len(auditsBefore))
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
	require.ErrorIs(t, op.Complete(ctx, changed), ErrOperatorConflict)
	changed = completion
	changed.ReplyDigest = digest("other-reply")
	require.ErrorIs(t, op.Complete(ctx, changed), ErrOperatorConflict)
	require.Equal(t, int64(2), verifyCalls.Load())
}

func TestOperatorTrustedClockIgnoresCallerAt(t *testing.T) {
	op, reply, _ := operatorFixture(t, SupportDisclosure)
	trusted := reply.At.Add(2 * time.Second)
	op.Clock = func() time.Time { return trusted }
	reply.At = reply.At.Add(-24 * time.Hour)
	require.NoError(t, op.Reply(context.Background(), reply))
	snap, err := op.Client.Collection("support-requests").Doc(reply.Intent.RequestRef).Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, trusted, snap.Data()["operatorReplyAt"])
	audit, err := op.auditRef(reply.Intent).Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, trusted, audit.Data()["at"])
}

func TestOperatorCompletionRejectsSwappedEvidenceAndReplyWithoutMutation(t *testing.T) {
	op, reply, oauth := operatorFixture(t, SupportDisclosure)
	ctx := context.Background()
	require.NoError(t, op.Reply(ctx, reply))
	ref := op.Client.Collection("support-requests").Doc(reply.Intent.RequestRef)
	before, err := ref.Get(ctx)
	require.NoError(t, err)
	indexRef := op.Client.Collection("support-request-ids").Doc(digest("development:" + before.Data()["requestId"].(string)))
	indexBefore, err := indexRef.Get(ctx)
	require.NoError(t, err)
	oauthRef := op.Client.Collection("oauth-transactions").Doc(oauth)
	oauthBefore, err := oauthRef.Get(ctx)
	require.NoError(t, err)
	completion := CompletionOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: reply.Intent.RequestRef, Purpose: SupportDisclosure, OperationID: digest("swapped-evidence-complete:" + reply.Intent.OperationID), Action: "complete"}, ProofRef: reply.ProofRef, ReplyOperationID: reply.Intent.OperationID, ExpectedRevision: 1, ActionEvidence: digest("synthetic-action"), DeliveryAcknowledgement: digest("synthetic-delivery"), At: reply.At.Add(time.Second)}
	verified := syntheticVerifiedCompletion(completion, before.Data()["replyDigest"].(string))
	auditRef := op.auditRef(completion.Intent)
	checks := map[string]func(*CompletionOperation, *VerifiedCompletionEvidence){
		"actor":       func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.Actor = "other-operator" },
		"environment": func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.Environment = "production" },
		"project":     func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.ProjectID = "demo-other-project" },
		"request":     func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.RequestRef = digest("other-request") },
		"purpose":     func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.Purpose = SupportRevoke },
		"completion operation": func(_ *CompletionOperation, v *VerifiedCompletionEvidence) {
			v.OperationID = digest("other-completion")
		},
		"proof": func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.ProofRef = digest("other-proof") },
		"reply operation": func(_ *CompletionOperation, v *VerifiedCompletionEvidence) {
			v.ReplyOperationID = digest("other-reply")
		},
		"revision":        func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.ExpectedRevision++ },
		"action evidence": func(_ *CompletionOperation, v *VerifiedCompletionEvidence) { v.ActionEvidence = digest("other-action") },
		"delivery evidence": func(_ *CompletionOperation, v *VerifiedCompletionEvidence) {
			v.DeliveryAcknowledgement = digest("other-delivery")
		},
		"submitted reply": func(op *CompletionOperation, _ *VerifiedCompletionEvidence) {
			op.ReplyOperationID = digest("other-reply")
		},
	}
	for name, change := range checks {
		t.Run(name, func(t *testing.T) {
			candidate, evidence := completion, verified
			change(&candidate, &evidence)
			op.Authority = syntheticOperatorAuthority{evidence: true, verified: evidence}
			require.ErrorIs(t, op.Complete(ctx, candidate), ErrOperatorDenied)
		})
	}
	t.Run("reply digest", func(t *testing.T) {
		evidence := verified
		evidence.ReplyDigest = digest("other-reply-body")
		op.Authority = syntheticOperatorAuthority{evidence: true, verified: evidence}
		require.ErrorIs(t, op.Complete(ctx, completion), ErrOperatorConflict)
	})
	t.Run("verified different reply", func(t *testing.T) {
		candidate, evidence := completion, verified
		candidate.ReplyOperationID = digest("other-reply")
		evidence.ReplyOperationID = candidate.ReplyOperationID
		evidence.ReplyDigest = digest("other-reply-body")
		op.Authority = syntheticOperatorAuthority{evidence: true, verified: evidence}
		require.ErrorIs(t, op.Complete(ctx, candidate), ErrOperatorConflict)
	})
	after, err := ref.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, before.Data(), after.Data())
	require.Equal(t, before.UpdateTime, after.UpdateTime)
	indexAfter, err := indexRef.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, indexBefore.Data(), indexAfter.Data())
	oauthAfter, err := oauthRef.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, oauthBefore.Data(), oauthAfter.Data())
	_, err = auditRef.Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
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

// Child cleanups must remove only their own records, before closing clients.
// No reset separates the same-name fixtures or the surviving fixture.
func TestOperatorFixturesRepeatAndCleanupWithoutAffectingOtherRecords(t *testing.T) {
	ctx := context.Background()
	survivor := newOperatorFixture(t, SupportDisclosure)
	require.NoError(t, survivor.operator.Reply(ctx, survivor.reply))
	before := map[string]*firestore.DocumentSnapshot{}
	for _, ref := range survivor.documents {
		snap, err := ref.Get(ctx)
		require.NoError(t, err)
		before[ref.Path] = snap
	}
	var removed []*firestore.DocumentRef
	t.Run("same-name-fixtures", func(t *testing.T) {
		first := newOperatorFixture(t, SupportDisclosure)
		second := newOperatorFixture(t, SupportDisclosure)
		require.NotEqual(t, first.reply.Intent.RequestRef, second.reply.Intent.RequestRef)
		require.NotEqual(t, first.oauth, second.oauth)
		require.NotEqual(t, first.reply.ProofRef, second.reply.ProofRef)
		require.NotEqual(t, first.reply.Intent.OperationID, second.reply.Intent.OperationID)
		require.NoError(t, first.operator.Reply(ctx, first.reply))
		snap, err := first.documents[0].Get(ctx)
		require.NoError(t, err)
		require.NoError(t, first.operator.Reply(ctx, first.reply)) // lost ACK uses the same identity
		retry, err := first.documents[0].Get(ctx)
		require.NoError(t, err)
		require.Equal(t, snap.UpdateTime, retry.UpdateTime)
		completion := CompletionOperation{Intent: OperatorIntent{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestRef: first.reply.Intent.RequestRef, Purpose: SupportDisclosure, OperationID: digest("complete:" + first.reply.Intent.OperationID), Action: "complete"}, ProofRef: first.reply.ProofRef, ReplyOperationID: first.reply.Intent.OperationID, ExpectedRevision: 1, ActionEvidence: digest("synthetic-action"), DeliveryAcknowledgement: digest("synthetic-delivery"), At: first.reply.At.Add(time.Second)}
		replyDigest, ok := snap.Data()["replyDigest"].(string)
		require.True(t, ok)
		completion.ReplyDigest = replyDigest
		first.operator.Authority = syntheticOperatorAuthority{evidence: true, verified: syntheticVerifiedCompletion(completion, replyDigest)}
		require.NoError(t, first.operator.Complete(ctx, completion))
		_, err = second.operator.auditRef(second.reply.Intent).Create(ctx, map[string]interface{}{"occupied": true})
		require.NoError(t, err)
		require.ErrorIs(t, second.operator.Reply(ctx, second.reply), ErrOperatorConflict)
		removed = append(removed, first.documents...)
		removed = append(removed, second.documents...)
		removed = append(removed, first.operator.auditRef(completion.Intent))
	})
	// Observe through the still-open survivor client after both child clients close.
	for _, ref := range removed {
		_, err := survivor.operator.Client.Collection(ref.Parent.ID).Doc(ref.ID).Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	for _, ref := range survivor.documents {
		after, err := ref.Get(ctx)
		require.NoError(t, err)
		require.Equal(t, before[ref.Path].Data(), after.Data())
		require.Equal(t, before[ref.Path].UpdateTime, after.UpdateTime)
	}
	require.NoError(t, survivor.operator.Reply(ctx, survivor.reply))
}
