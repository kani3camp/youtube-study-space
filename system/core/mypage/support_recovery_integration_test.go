//go:build integration

package mypage

import (
	"context"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
)

type recoveryFixture struct {
	store          *FirestoreSupportStore
	selector       SupportRecoverySelector
	ref, challenge string
	now            time.Time
	auth           *AuthService
	minter         *fakeMinter
}

func newRecoveryFixture(t *testing.T) recoveryFixture {
	t.Helper()
	auth, store, provider, minter, now := authTestService(t)
	id, err := opaque()
	require.NoError(t, err)
	channel := "UC" + digest(id)[:22]
	provider.channels[0].ID = channel
	support := &FirestoreSupportStore{Client: store.Client, Environment: "development"}
	auth.Support = support
	selector := SupportRecoverySelector{Environment: "development", ProjectID: "demo-youtube-study-space-ci", RequestID: id, Channel: channel, Purpose: SupportDelete, AcceptedAt: now.Add(-8 * 24 * time.Hour)}
	ref, challenge, err := support.Create(context.Background(), id, channel, selector.Purpose, selector.AcceptedAt, now)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		indexes, err := store.Client.Collection("support-challenges").Where("requestRef", "==", ref).Documents(ctx).GetAll()
		require.NoError(t, err)
		for _, doc := range indexes {
			_, err := doc.Ref.Delete(ctx)
			require.NoError(t, err)
		}
		_, err = store.Client.Collection("support-challenges").Doc(digest(challenge)).Delete(ctx)
		require.NoError(t, err)
		_, err = store.Client.Collection("support-request-ids").Doc(digest("development:" + id)).Delete(ctx)
		require.NoError(t, err)
		oauth, err := store.Client.Collection("oauth-transactions").Where("support.requestRef", "==", ref).Documents(ctx).GetAll()
		require.NoError(t, err)
		for _, doc := range oauth {
			_, err := doc.Ref.Delete(ctx)
			require.NoError(t, err)
		}
		_, err = store.Client.Collection("support-requests").Doc(ref).Delete(ctx)
		require.NoError(t, err)
	})
	return recoveryFixture{store: support, selector: selector, ref: ref, challenge: challenge, now: now, auth: auth, minter: minter}
}

func (f recoveryFixture) document(t *testing.T) *firestore.DocumentSnapshot {
	t.Helper()
	doc, err := f.store.Client.Collection("support-requests").Doc(f.ref).Get(context.Background())
	require.NoError(t, err)
	return doc
}

func TestSupportRecoveryReconcilesLostCreateACKWithoutDuplicateOrDeadlineReset(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	// The Create commit is known only to the test fixture; recovery receives the
	// original selector, not a returned ref or the lost delivery challenge.
	before := f.document(t)
	view, err := f.store.InspectRequestID(ctx, f.selector, f.now)
	require.NoError(t, err)
	require.Equal(t, f.ref, view.RequestRef)
	require.Equal(t, before.UpdateTime, view.Revision)
	require.Equal(t, f.selector.AcceptedAt.Add(7*24*time.Hour), *view.DeleteBy)
	require.Equal(t, before.UpdateTime, f.document(t).UpdateTime, "inspection is read-only")
	ref, next, err := f.store.ReissueRequestID(ctx, f.selector, view.Revision, f.now.Add(25*time.Hour))
	require.NoError(t, err)
	require.Equal(t, f.ref, ref)
	_, err = f.store.Resolve(ctx, f.challenge, f.now)
	require.Error(t, err, "old challenge is invalidated")
	_, err = f.store.Resolve(ctx, next, f.now.Add(25*time.Hour))
	require.NoError(t, err)
	after := f.document(t)
	var record SupportRequest
	require.NoError(t, after.DataTo(&record))
	require.Equal(t, f.selector.AcceptedAt, record.AcceptedAt)
	require.Equal(t, *view.DeleteBy, *record.DeleteBy)
	// Model a lost reissue ACK: retry the same reviewed version, never blindly
	// issue another challenge. Even an old expiry is not permission to retry.
	_, _, err = f.store.ReissueRequestID(ctx, f.selector, view.Revision, f.now.Add(26*time.Hour))
	require.Error(t, err)
	require.Equal(t, after.UpdateTime, f.document(t).UpdateTime)
	_, err = f.store.Resolve(ctx, next, f.now.Add(26*time.Hour))
	require.NoError(t, err)
	docs, err := f.store.Client.Collection("support-requests").Where("requestId", "==", f.selector.RequestID).Documents(ctx).GetAll()
	require.NoError(t, err)
	require.Len(t, docs, 1)
}

func TestSupportRecoveryRejectsSelectorSubstitutionAndCorruptBindingsAtomically(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	view, err := f.store.InspectRequestID(ctx, f.selector, f.now)
	require.NoError(t, err)
	for name, change := range map[string]func(*SupportRecoverySelector){
		"environment":   func(s *SupportRecoverySelector) { s.Environment = "production" },
		"project":       func(s *SupportRecoverySelector) { s.ProjectID = "demo-other-project" },
		"request ID":    func(s *SupportRecoverySelector) { s.RequestID = digest("missing-recovery-id") },
		"channel":       func(s *SupportRecoverySelector) { s.Channel = "UCsynthetic0000000000002" },
		"purpose":       func(s *SupportRecoverySelector) { s.Purpose = SupportRevoke },
		"accepted time": func(s *SupportRecoverySelector) { s.AcceptedAt = s.AcceptedAt.Add(time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			s := f.selector
			change(&s)
			_, err := f.store.InspectRequestID(ctx, s, f.now)
			require.Error(t, err)
			_, _, err = f.store.ReissueRequestID(ctx, s, view.Revision, f.now)
			require.Error(t, err)
			require.Equal(t, view.Revision, f.document(t).UpdateTime)
		})
	}
	for _, kind := range []string{"missing ID index", "wrong ID ref", "wrong challenge ref", "missing challenge index", "wrong deadline", "completed"} {
		t.Run(kind, func(t *testing.T) {
			g := newRecoveryFixture(t)
			index := g.store.Client.Collection("support-request-ids").Doc(digest("development:" + g.selector.RequestID))
			old := g.store.Client.Collection("support-challenges").Doc(digest(g.challenge))
			switch kind {
			case "missing ID index":
				_, err = index.Delete(ctx)
			case "wrong ID ref":
				_, err = index.Set(ctx, supportChallengeIndex{Environment: "development", RequestRef: f.ref})
			case "wrong challenge ref":
				_, err = old.Set(ctx, supportChallengeIndex{Environment: "development", RequestRef: f.ref})
			case "missing challenge index":
				_, err = old.Delete(ctx)
			case "wrong deadline":
				_, err = g.store.Client.Collection("support-requests").Doc(g.ref).Update(ctx, []firestore.Update{{Path: "deleteBy", Value: g.now.Add(7 * 24 * time.Hour)}})
			case "completed":
				_, err = g.store.Client.Collection("support-requests").Doc(g.ref).Update(ctx, []firestore.Update{{Path: "status", Value: "completed"}})
			}
			require.NoError(t, err)
			before := g.document(t)
			_, err = g.store.InspectRequestID(ctx, g.selector, g.now)
			require.Error(t, err)
			_, _, err = g.store.ReissueRequestID(ctx, g.selector, before.UpdateTime, g.now)
			require.Error(t, err)
			require.Equal(t, before.UpdateTime, g.document(t).UpdateTime, "rejection must not alter receipt or reset deadline")
		})
	}
}

func TestSupportRecoveryConcurrentVersionHasExactlyOneWinner(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	view, err := f.store.InspectRequestID(ctx, f.selector, f.now)
	require.NoError(t, err)
	var nonce atomic.Value
	winners := concurrently(t, func() error {
		_, challenge, err := f.store.ReissueRequestID(ctx, f.selector, view.Revision, f.now.Add(time.Minute))
		if err == nil {
			nonce.Store(challenge)
		}
		return err
	})
	require.Equal(t, 1, winners)
	challenge, ok := nonce.Load().(string)
	require.True(t, ok)
	_, err = f.store.Resolve(ctx, challenge, f.now.Add(time.Minute))
	require.NoError(t, err)
	_, err = f.store.Resolve(ctx, f.challenge, f.now.Add(time.Minute))
	require.Error(t, err)
}

func prepareRecoveryProof(t *testing.T, f recoveryFixture, challenge string) (string, string) {
	t.Helper()
	ctx := context.Background()
	response, id, err := f.auth.Start(ctx, "", StartRequest{SupportChallenge: &challenge, PrivacyPolicyVersion: f.auth.Policy.Privacy, PrivacyAccepted: true, TermsVersion: f.auth.Policy.Terms, TermsAccepted: true})
	require.NoError(t, err)
	u, err := url.Parse(response.AuthorizationURL)
	require.NoError(t, err)
	require.NoError(t, f.auth.Callback(ctx, id, u.Query().Get("state"), "synthetic-code", false))
	channel, err := f.auth.Channel(ctx, id)
	require.NoError(t, err)
	return id, channel.ConfirmationRef
}

func TestSupportRecoveryAndFreshProofHaveOneAtomicWinner(t *testing.T) {
	for _, order := range []string{"recover before proof", "proof before recover", "concurrent"} {
		t.Run(order, func(t *testing.T) {
			f := newRecoveryFixture(t)
			ctx := context.Background()
			view, err := f.store.InspectRequestID(ctx, f.selector, f.now)
			require.NoError(t, err)
			id, confirmation := prepareRecoveryProof(t, f, f.challenge)
			var next string
			recoverCall := func() error {
				var err error
				_, next, err = f.store.ReissueRequestID(ctx, f.selector, view.Revision, f.now)
				return err
			}
			proofCall := func() error { _, err := f.auth.ConfirmResult(ctx, id, confirmation); return err }
			switch order {
			case "recover before proof":
				require.NoError(t, recoverCall())
				require.Error(t, proofCall())
				newID, newConfirmation := prepareRecoveryProof(t, f, next)
				proof, err := f.auth.ConfirmResult(ctx, newID, newConfirmation)
				require.NoError(t, err)
				require.Empty(t, proof.CustomToken)
			case "proof before recover":
				require.NoError(t, proofCall())
				require.Error(t, recoverCall())
			case "concurrent":
				start := make(chan struct{})
				results := make(chan error, 2)
				var wg sync.WaitGroup
				for _, call := range []func() error{recoverCall, proofCall} {
					wg.Add(1)
					go func() { defer wg.Done(); <-start; results <- call() }()
				}
				close(start)
				wg.Wait()
				close(results)
				winners := 0
				for err := range results {
					if err == nil {
						winners++
					}
				}
				require.Equal(t, 1, winners)
			}
			var record SupportRequest
			require.NoError(t, f.document(t).DataTo(&record))
			require.Equal(t, f.selector.AcceptedAt, record.AcceptedAt)
			require.Equal(t, f.selector.AcceptedAt.Add(7*24*time.Hour), *record.DeleteBy)
			require.Zero(t, f.minter.calls.Load())
			if record.Status == "verified" {
				_, err := f.store.InspectRequestID(ctx, f.selector, f.now)
				require.Error(t, err, "verified request cannot be reopened")
			}
		})
	}
}
