//go:build integration

package mypage

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
	"app.modules/core/supportdelete"
	"app.modules/internal/integrationtest"
)

func deletionFixture(t *testing.T) (*FirestoreDeletionStore, supportdelete.Start, string) {
	t.Helper()
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	now := time.Now().UTC().Truncate(time.Microsecond)
	channel := "UC" + digest(t.Name() + fmt.Sprint(now.UnixNano()))[:22]
	requestID := "synthetic-delete-" + fmt.Sprint(now.UnixNano())
	store := &FirestoreSupportStore{Client: client, Environment: "development"}
	ref, challenge, err := store.Create(ctx, requestID, channel, SupportDelete, now.Add(-4*24*time.Hour), now.Add(-4*time.Minute))
	require.NoError(t, err)
	binding, err := store.Resolve(ctx, challenge, now.Add(-3*time.Minute))
	require.NoError(t, err)
	oauthRef := digest(requestID + "oauth")
	proof := digest(requestID + "proof")
	policy := Policy{Privacy: "synthetic", Terms: "synthetic"}
	oauth := OAuthTransaction{Purpose: "support", Support: &binding, Status: "channel_verified", Channel: Channel{ID: channel}, PrivacyPolicyVersion: policy.Privacy, TermsVersion: policy.Terms, CreatedAt: now.Add(-3 * time.Minute), VerifiedAt: now.Add(-2 * time.Minute), ExpiresAt: now.Add(7 * time.Minute), ConfirmationRef: digest(requestID + "confirm")}
	auth := &FirestoreAuthStore{Client: client}
	require.NoError(t, auth.CreateSupport(ctx, oauthRef, "", oauth, "development", oauth.CreatedAt))
	require.NoError(t, auth.ConsumeSupport(ctx, oauthRef, oauth.ConfirmationRef, policy, "development", proof, now.Add(-time.Minute)))
	selector := supportdelete.Selector{Target: supportdelete.Target{Environment: "development", ProjectID: "demo-youtube-study-space-ci", ChannelID: channel}, RequestRef: ref, ExecutionRef: digest(requestID + "execution"), ProofRef: proof}
	start := supportdelete.Start{Selector: selector, Manifest: supportdelete.Manifest{Ref: digest("synthetic-complete-fleet"), Mode: "emulator"}, OwnerRef: digest(requestID + "owner"), Now: now}
	t.Cleanup(func() {
		for _, r := range []*firestore.DocumentRef{client.Collection(serviceaccess.Collection).Doc(channel), client.Collection("support-requests").Doc(ref), client.Collection("support-request-ids").Doc(digest("development:" + requestID)), client.Collection("oauth-transactions").Doc(oauthRef), client.Collection(supportDeleteClaims).Doc(proof), client.Collection(supportDeleteExecutions).Doc(selector.ExecutionRef), client.Collection("users").Doc(channel), client.Collection(supportDeleteBigQueryInventory).Doc(selector.ExecutionRef), client.Collection(supportDeleteBigQueryBinding).Doc(selector.ExecutionRef)} {
			_, err := r.Delete(ctx)
			require.NoError(t, err)
		}
	})
	return &FirestoreDeletionStore{Client: client}, start, oauthRef
}

type deletionFixtureEffects struct{}

func (deletionFixtureEffects) Apply(_ context.Context, op supportdelete.Operation) (supportdelete.Evidence, error) {
	e := op.Execution
	return supportdelete.Evidence{Selector: e.Selector, ManifestRef: e.Manifest.Ref, OperationID: op.ID, Action: op.Step.Action, Scope: op.Step.Scope, Mode: "emulator", TraceRef: digest("synthetic-evidence:" + op.ID), Generation: e.Generation, Cutoff: e.Cutoff, ObservedAt: op.ObservedAfter, Known: true, Complete: true, AllInstances: true, OldQueueRejected: true, RestoreExcluded: true}, nil
}

func advanceDeletion(t *testing.T, s *FirestoreDeletionStore, e supportdelete.Execution, cursor int, now time.Time) supportdelete.Execution {
	t.Helper()
	for e.Cursor < cursor {
		v, err := (deletionFixtureEffects{}).Apply(context.Background(), supportdelete.NewOperation(e, now))
		require.NoError(t, err)
		e, err = s.Commit(context.Background(), e, v, now)
		require.NoError(t, err)
	}
	return e
}

func TestDeletionClaimGuardAndProofAreAtomicAndSingleExecution(t *testing.T) {
	s, start, oauthRef := deletionFixture(t)
	ctx := context.Background()
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			candidate := start
			candidate.Selector.ExecutionRef = digest(fmt.Sprintf("%s:%d", start.Selector.ExecutionRef, i))
			candidate.OwnerRef = digest(fmt.Sprintf("%s:%d", start.OwnerRef, i))
			_, err := s.Acquire(ctx, candidate)
			if err == nil {
				successes.Add(1)
			}
			t.Cleanup(func() {
				_, err := s.Client.Collection(supportDeleteExecutions).Doc(candidate.Selector.ExecutionRef).Delete(ctx)
				require.NoError(t, err)
			})
		})
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	control, err := (&serviceaccess.FirestoreStore{Client: s.Client}).Read(ctx, start.Selector.Target.ChannelID)
	require.NoError(t, err)
	require.True(t, control.Control.PrivacyDeletion.Active)
	require.Equal(t, start.Selector.RequestRef, control.Control.PrivacyDeletion.RequestRef)
	claim, err := s.Client.Collection(supportDeleteClaims).Doc(start.Selector.ProofRef).Get(ctx)
	require.NoError(t, err)
	start.Selector.ExecutionRef = claim.Data()["executionRef"].(string)
	doc, err := s.Client.Collection(supportDeleteExecutions).Doc(start.Selector.ExecutionRef).Get(ctx)
	require.NoError(t, err)
	e, err := readDeletionExecution(doc)
	require.NoError(t, err)
	start.OwnerRef = e.OwnerRef
	_, err = s.Client.Collection("oauth-transactions").Doc(oauthRef).Delete(ctx)
	require.NoError(t, err)
	start.Now = start.Now.Add(24 * time.Hour)
	resumed, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	require.Equal(t, e, resumed, "same execution uses its bound proof after TTL, without resetting SLA")
	otherOwner := start
	otherOwner.OwnerRef = digest("new-owner")
	_, err = s.Acquire(ctx, otherOwner)
	require.ErrorIs(t, err, supportdelete.ErrBusy)
	recovery := supportdelete.Recovery{Selector: e.Selector, PreviousOwnerRef: e.OwnerRef, NewOwnerRef: otherOwner.OwnerRef, ManifestRef: e.Manifest.Ref, TraceRef: digest("trusted-stopped-trace"), Revision: e.Revision, Generation: e.Generation, ObservedAt: start.Now}
	require.ErrorIs(t, s.Recover(ctx, recovery, start.Now), supportdelete.ErrEvidence, "time alone never proves worker stopped")
	recovery.Stopped = true
	require.NoError(t, s.Recover(ctx, recovery, start.Now))
	_, err = s.Acquire(ctx, otherOwner)
	require.NoError(t, err)
}

func TestDeletionMissingTraceWrongTargetAndBindingLeaveNoGuard(t *testing.T) {
	for _, change := range []string{"missing-trace", "wrong-project", "wrong-channel", "wrong-proof", "wrong-purpose"} {
		t.Run(change, func(t *testing.T) {
			s, start, oauthRef := deletionFixture(t)
			ctx := context.Background()
			switch change {
			case "missing-trace":
				_, err := s.Client.Collection("oauth-transactions").Doc(oauthRef).Delete(ctx)
				require.NoError(t, err)
			case "wrong-project":
				start.Selector.Target.ProjectID = "demo-other-project"
			case "wrong-channel":
				start.Selector.Target.ChannelID = "UCsynthetic0000000000001"
			case "wrong-proof":
				start.Selector.ProofRef = digest("wrong-proof")
			case "wrong-purpose":
				_, err := s.Client.Collection("support-requests").Doc(start.Selector.RequestRef).Update(ctx, []firestore.Update{{Path: "purpose", Value: "disclosure"}})
				require.NoError(t, err)
			}
			_, err := s.Acquire(ctx, start)
			require.Error(t, err)
			_, err = s.Client.Collection(supportDeleteExecutions).Doc(start.Selector.ExecutionRef).Get(ctx)
			require.Equal(t, codes.NotFound, status.Code(err))
			_, err = s.Client.Collection(serviceaccess.Collection).Doc(start.Selector.Target.ChannelID).Get(ctx)
			require.Equal(t, codes.NotFound, status.Code(err))
		})
	}
}

func TestDeletionFreshGenerationAndRequestCASPreserveModerationAndScrubLinks(t *testing.T) {
	s, start, oauthRef := deletionFixture(t)
	ctx := context.Background()
	controls := &serviceaccess.FirestoreStore{Client: s.Client}
	_, err := controls.Change(ctx, start.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "SECURITY", Reference: digest("moderation1")}, start.Now)
	require.NoError(t, err)
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, supportdelete.InspectionStart()+2, start.Now)
	_, err = controls.Change(ctx, start.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "MODERATION", Reference: digest("moderation2")}, start.Now.Add(time.Second))
	require.NoError(t, err)
	v, _ := (deletionFixtureEffects{}).Apply(ctx, supportdelete.NewOperation(e, start.Now.Add(time.Second)))
	_, err = s.Commit(ctx, e, v, start.Now.Add(time.Second))
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	e, err = s.Refresh(ctx, e, start.Now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, supportdelete.InspectionStart(), e.Cursor)
	e = advanceDeletion(t, s, e, len(supportdelete.Steps()), start.Now.Add(time.Second))
	_, err = controls.Change(ctx, start.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.Moderation, Active: true, ReasonCode: "SECURITY", Reference: digest("moderation3")}, start.Now.Add(2*time.Second))
	require.NoError(t, err)
	require.ErrorIs(t, s.Finalize(ctx, e, start.Now.Add(2*time.Second)), supportdelete.ErrConflict)
	e, err = s.Refresh(ctx, e, start.Now.Add(2*time.Second))
	require.NoError(t, err)
	require.Equal(t, len(supportdelete.Steps()), e.Cursor)
	require.NoError(t, s.Finalize(ctx, e, start.Now.Add(2*time.Second)))
	control, err := controls.Read(ctx, start.Selector.Target.ChannelID)
	require.NoError(t, err)
	require.True(t, control.Exists)
	require.False(t, control.Control.PrivacyDeletion.Active)
	require.True(t, control.Control.Moderation.Active)
	require.Equal(t, digest("moderation3"), control.Control.Moderation.ActionRef)
	for _, r := range []*firestore.DocumentRef{s.Client.Collection("support-requests").Doc(start.Selector.RequestRef), s.Client.Collection(supportDeleteExecutions).Doc(start.Selector.ExecutionRef)} {
		doc, err := r.Get(ctx)
		require.NoError(t, err)
		require.True(t, deletionComplete(doc))
		if r.Parent.ID == supportDeleteExecutions {
			require.Len(t, doc.Data(), 5)
		} else {
			require.Len(t, doc.Data(), 4)
		}
	}
	for _, r := range []*firestore.DocumentRef{s.Client.Collection("oauth-transactions").Doc(oauthRef), s.Client.Collection(supportDeleteClaims).Doc(start.Selector.ProofRef)} {
		_, err := r.Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	_, err = s.Acquire(ctx, start)
	require.ErrorIs(t, err, supportdelete.ErrCompleted)
	wrong := start
	wrong.Selector.ProofRef = digest("other-proof")
	_, err = s.Acquire(ctx, wrong)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	wrong = start
	wrong.Selector.Target.ChannelID = "UCsynthetic0000000000001"
	_, err = s.Acquire(ctx, wrong)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
	wrong = start
	wrong.Manifest.Ref = digest("other-manifest")
	_, err = s.Acquire(ctx, wrong)
	require.ErrorIs(t, err, supportdelete.ErrConflict)
}

func TestDeletionOtherPrivacyRequestCannotBeCleared(t *testing.T) {
	s, start, _ := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, len(supportdelete.Steps()), start.Now)
	_, err = (&serviceaccess.FirestoreStore{Client: s.Client}).Change(ctx, start.Selector.Target.ChannelID, serviceaccess.Change{Reason: serviceaccess.PrivacyDeletion, Active: true, ReasonCode: "PRIVACY_DELETION", Reference: digest("other-request")}, start.Now.Add(time.Second))
	require.NoError(t, err)
	require.ErrorIs(t, s.Finalize(ctx, e, start.Now.Add(time.Second)), supportdelete.ErrConflict)
	_, err = s.Refresh(ctx, e, start.Now.Add(time.Second))
	require.ErrorIs(t, err, supportdelete.ErrConflict)
}

func TestDeletionSourceAdapterPaginationIsolationAndUnknownRestore(t *testing.T) {
	s, start, _ := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, 6, start.Now) // firestore-primary delete, after dedicated seat removal
	op := supportdelete.NewOperation(e, start.Now)
	refs := make([]*firestore.DocumentRef, 1002)
	for i := range refs {
		refs[i] = s.Client.Collection("work-segments").Doc(digest(start.Selector.ExecutionRef + fmt.Sprint(i)))
	}
	other := s.Client.Collection("work-segments").Doc(digest(start.Selector.ExecutionRef + "other"))
	refs = append(refs, other)
	for base := 0; base < len(refs); base += 400 {
		batch := s.Client.Batch()
		for _, r := range refs[base:min(base+400, len(refs))] {
			channel := start.Selector.Target.ChannelID
			if r == other {
				channel = "UCsynthetic0000000000002"
			}
			batch.Set(r, map[string]interface{}{"user-id": channel, "work-name": "synthetic private work"})
		}
		_, err := batch.Commit(ctx)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		for base := 0; base < len(refs); base += 400 {
			batch := s.Client.Batch()
			for _, r := range refs[base:min(base+400, len(refs))] {
				batch.Delete(r)
			}
			_, err := batch.Commit(ctx)
			require.NoError(t, err)
		}
	})
	adapter := &FirestoreDeletionEffects{Store: s, Clock: func() time.Time { return start.Now }}
	_, err = adapter.Apply(ctx, op)
	require.Error(t, err)
	_, err = refs[0].Get(ctx)
	require.NoError(t, err, "missing restore proof must not start deletion")
	adapter.RestoreGuard = deletionFixtureEffects{}
	v, err := adapter.Apply(ctx, op)
	require.NoError(t, err)
	require.NoError(t, v.Validate(op, start.Now))
	rows, err := s.Client.Collection("work-segments").Where("user-id", "==", start.Selector.Target.ChannelID).Documents(ctx).GetAll()
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = other.Get(ctx)
	require.NoError(t, err, "another channel must survive")
	_, err = adapter.Apply(ctx, op)
	require.NoError(t, err, "lost acknowledgement reconciles empty pages")
}

func TestDeletionCheckpointClientRulesDenyReadAndWrite(t *testing.T) {
	s, start, _ := deletionFixture(t)
	_, err := s.Acquire(context.Background(), start)
	require.NoError(t, err)
	encode := base64.RawURLEncoding.EncodeToString
	token := encode([]byte(`{"alg":"none","typ":"JWT"}`)) + "." + encode([]byte(`{"iss":"https://securetoken.google.com/demo-youtube-study-space-ci","aud":"demo-youtube-study-space-ci","sub":"synthetic-rules-user","user_id":"synthetic-rules-user","iat":1780000000,"exp":2090000000,"firebase":{"sign_in_provider":"custom"}}`)) + "."
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, collection := range []string{supportDeleteExecutions, supportDeleteClaims, supportDeleteAuthOwnership, supportDeleteBigQueryInventory, supportDeleteBigQueryBinding} {
		ref := start.Selector.ExecutionRef
		if collection == supportDeleteClaims {
			ref = start.Selector.ProofRef
		}
		endpoint := "http://" + os.Getenv("FIRESTORE_EMULATOR_HOST") + "/v1/projects/demo-youtube-study-space-ci/databases/(default)/documents/" + collection + "/" + ref
		for _, auth := range []string{"", "Bearer " + token} {
			for _, method := range []string{http.MethodGet, http.MethodPatch} {
				request, err := http.NewRequestWithContext(context.Background(), method, endpoint, strings.NewReader(`{"fields":{"cursor":{"integerValue":"29"}}}`))
				require.NoError(t, err)
				request.Header.Set("Content-Type", "application/json")
				if auth != "" {
					request.Header.Set("Authorization", auth)
				}
				response, err := client.Do(request)
				require.NoError(t, err)
				require.Equal(t, http.StatusForbidden, response.StatusCode)
				require.NoError(t, response.Body.Close())
			}
		}
	}
}

func TestDeletionSupportRelationsPreserveCurrentProofAndRejectOtherOwnedCases(t *testing.T) {
	s, start, oauthRef := deletionFixture(t)
	ctx := context.Background()
	e, err := s.Acquire(ctx, start)
	require.NoError(t, err)
	e = advanceDeletion(t, s, e, 8, start.Now)
	oldOAuth := s.Client.Collection("oauth-transactions").Doc(digest(start.Selector.ExecutionRef + "old-oauth"))
	challenge := s.Client.Collection("support-challenges").Doc(digest(start.Selector.ExecutionRef + "stale-challenge"))
	duplicateIndex := s.Client.Collection("support-request-ids").Doc(digest(start.Selector.ExecutionRef + "duplicate-index"))
	otherReceipt := s.Client.Collection("support-requests").Doc(digest(start.Selector.ExecutionRef + "other-receipt"))
	otherClaim := s.Client.Collection(supportDeleteClaims).Doc(digest(start.Selector.ExecutionRef + "other-claim"))
	for _, r := range []*firestore.DocumentRef{oldOAuth, challenge, duplicateIndex} {
		data := map[string]interface{}{"requestRef": start.Selector.RequestRef}
		if r == oldOAuth {
			data = map[string]interface{}{"support": map[string]interface{}{"requestRef": start.Selector.RequestRef}}
		}
		_, err := r.Set(ctx, data)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		for _, r := range []*firestore.DocumentRef{oldOAuth, challenge, duplicateIndex, otherReceipt, otherClaim} {
			_, err := r.Delete(ctx)
			require.NoError(t, err)
		}
	})
	adapter := &FirestoreDeletionEffects{Store: s, RestoreGuard: deletionFixtureEffects{}, Clock: func() time.Time { return start.Now }}
	op := supportdelete.NewOperation(e, start.Now)
	v, err := adapter.Apply(ctx, op)
	require.NoError(t, err)
	e, err = s.Commit(ctx, e, v, start.Now)
	require.NoError(t, err)
	_, err = oldOAuth.Get(ctx)
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = s.Client.Collection("oauth-transactions").Doc(oauthRef).Get(ctx)
	require.NoError(t, err, "current proof survives until finalization")
	_, err = otherReceipt.Set(ctx, map[string]interface{}{"targetChannel": start.Selector.Target.ChannelID})
	require.NoError(t, err)
	op = supportdelete.NewOperation(e, start.Now)
	_, err = adapter.Apply(ctx, op)
	require.Error(t, err)
	_, err = otherReceipt.Get(ctx)
	require.NoError(t, err, "another case cannot be silently completed")
	_, err = otherReceipt.Delete(ctx)
	require.NoError(t, err)
	_, err = otherClaim.Set(ctx, map[string]interface{}{"requestRef": start.Selector.RequestRef, "executionRef": digest("another-execution")})
	require.NoError(t, err)
	_, err = adapter.Apply(ctx, op)
	require.Error(t, err)
	_, err = otherClaim.Get(ctx)
	require.NoError(t, err, "another claim cannot be silently unbound")
	_, err = otherClaim.Delete(ctx)
	require.NoError(t, err)
	v, err = adapter.Apply(ctx, op)
	require.NoError(t, err)
	require.NoError(t, v.Validate(op, start.Now))
	for _, r := range []*firestore.DocumentRef{challenge, duplicateIndex} {
		_, err := r.Get(ctx)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	doc, err := s.Client.Collection("support-requests").Doc(start.Selector.RequestRef).Get(ctx)
	require.NoError(t, err)
	var receipt SupportRequest
	require.NoError(t, doc.DataTo(&receipt))
	_, err = s.Client.Collection("support-request-ids").Doc(digest(receipt.Environment + ":" + receipt.RequestID)).Get(ctx)
	require.NoError(t, err, "canonical receipt index survives until finalization")
}
