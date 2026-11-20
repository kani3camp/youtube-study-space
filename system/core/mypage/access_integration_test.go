//go:build integration

package mypage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"app.modules/core/serviceaccess"
)

type mintFunc func(context.Context, string) (string, error)

func (f mintFunc) Mint(ctx context.Context, uid string) (string, error) { return f(ctx, uid) }

type consumeHookStore struct {
	AuthStore
	before func()
}

func (s *consumeHookStore) Consume(ctx context.Context, id, confirmation, checkpoint string, policy Policy, now time.Time) (Channel, error) {
	s.before()
	return s.AuthStore.Consume(ctx, id, confirmation, checkpoint, policy, now)
}

func activateRestriction(t *testing.T, store *FirestoreAuthStore, uid string, deletion bool, now time.Time) serviceaccess.Snapshot {
	t.Helper()
	reason, code := serviceaccess.Moderation, "MODERATION"
	if deletion {
		reason, code = serviceaccess.PrivacyDeletion, "PRIVACY_DELETION"
	}
	value, err := (&serviceaccess.FirestoreStore{Client: store.Client}).Change(context.Background(), uid, serviceaccess.Change{Reason: reason, Active: true, ReasonCode: code, Reference: strings.Repeat("b", 64)}, now)
	require.NoError(t, err)
	return value
}

func TestConfirmFinalTransactionRejectsLateBlockWithoutAccountOrConsume(t *testing.T) {
	for _, deletion := range []bool{false, true} {
		s, store, _, minter, now := authTestService(t)
		uid := "UCsynthetic0000000000001"
		_, err := store.Client.Collection("web-accounts").Doc(uid).Delete(context.Background())
		require.NoError(t, err)
		id, confirmation := seedTransaction(t, s, "channel_verified", now)
		s.Store = &consumeHookStore{AuthStore: store, before: func() { activateRestriction(t, store, uid, deletion, now) }}
		token, err := s.Confirm(context.Background(), id, confirmation)
		want := "SERVICE_ACCESS_RESTRICTED"
		if deletion {
			want = "DATA_DELETION_IN_PROGRESS"
		}
		require.Empty(t, token)
		require.Equal(t, want, errorCode(err))
		require.Zero(t, minter.calls.Load())
		_, err = store.ReadVerified(context.Background(), id, now)
		require.NoError(t, err, "guard must roll back consumption")
		_, err = store.Client.Collection("web-accounts").Doc(uid).Get(context.Background())
		require.Equal(t, codes.NotFound, status.Code(err))
	}
}

func TestConfirmDropsTokenWhenRestrictionStartsDuringExternalMint(t *testing.T) {
	s, store, _, _, now := authTestService(t)
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	s.Minter = mintFunc(func(context.Context, string) (string, error) {
		activateRestriction(t, store, "UCsynthetic0000000000001", false, now)
		return "synthetic-undelivered-token", nil
	})
	token, err := s.Confirm(context.Background(), id, confirmation)
	require.Empty(t, token)
	require.Equal(t, "SERVICE_ACCESS_RESTRICTED", errorCode(err))
	_, err = store.ReadVerified(context.Background(), id, now)
	require.Equal(t, "OAUTH_TRANSACTION_CONSUMED", errorCode(err))
}

func TestSessionCompletionFinalGuardAndGenerationFence(t *testing.T) {
	s, store, _, _, now := authTestService(t)
	ctx := context.Background()
	uid := "UCsynthetic0000000000001"
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	_, err := s.Confirm(ctx, id, confirmation)
	require.NoError(t, err)
	blocked := activateRestriction(t, store, uid, false, now)
	err = store.CompleteSession(ctx, uid, "absent", s.Policy, now)
	require.Equal(t, "SERVICE_ACCESS_RESTRICTED", errorCode(err))
	control := &serviceaccess.FirestoreStore{Client: store.Client}
	unblocked, err := control.Change(ctx, uid, serviceaccess.Change{Reason: serviceaccess.Moderation}, now.Add(time.Second))
	require.NoError(t, err)
	require.NotEqual(t, blocked.Checkpoint(), unblocked.Checkpoint())
	err = store.CompleteSession(ctx, uid, "absent", s.Policy, now)
	require.Equal(t, "TEMPORARY_UNAVAILABLE", errorCode(err))
	require.NoError(t, store.CompleteSession(ctx, uid, unblocked.Checkpoint(), s.Policy, now))
}
