//go:build integration

package mypage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"app.modules/core/serviceaccess"
	"app.modules/internal/integrationtest"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeOAuth struct {
	calls    atomic.Int32
	channels []Channel
	err      error
}

func (f *fakeOAuth) AuthorizationURL(state string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state
}

func (f *fakeOAuth) Resolve(context.Context, string) ([]Channel, error) {
	f.calls.Add(1)
	return f.channels, f.err
}

type fakeMinter struct {
	calls atomic.Int32
	err   error
}

func (f *fakeMinter) Mint(context.Context, string) (string, error) {
	f.calls.Add(1)
	return "synthetic-custom-token", f.err
}

func authTestService(t *testing.T) (*AuthService, *FirestoreAuthStore, *fakeOAuth, *fakeMinter, time.Time) {
	t.Helper()
	integrationtest.RequireFirestoreEmulator(t)
	client, err := firestore.NewClient(context.Background(), "demo-youtube-study-space-ci", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	store := &FirestoreAuthStore{Client: client}
	controlRef := client.Collection(serviceaccess.Collection).Doc("UCsynthetic0000000000001")
	if _, err := controlRef.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := controlRef.Delete(context.Background()); err != nil {
			t.Error(err)
		}
	})
	provider := &fakeOAuth{channels: []Channel{{ID: "UCsynthetic0000000000001", DisplayName: "Sample Channel"}}}
	minter := &fakeMinter{}
	now := time.Now().UTC().Truncate(time.Second)
	service := &AuthService{Access: &serviceaccess.FirestoreStore{Client: client}, Store: store, Provider: provider, Minter: minter, Policy: Policy{Privacy: "test-policy", Terms: "test-terms"}, Now: func() time.Time { return now }}
	return service, store, provider, minter, now
}

func seedTransaction(t *testing.T, s *AuthService, status string, now time.Time) (string, string) {
	t.Helper()
	id, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	confirmation, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	value := OAuthTransaction{StateHash: digest("synthetic-state"), Status: status, PrivacyPolicyVersion: s.Policy.Privacy, TermsVersion: s.Policy.Terms, PrivacyConsentedAt: now, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute), ConfirmationRef: confirmation, Channel: Channel{ID: "UCsynthetic0000000000001", DisplayName: "Sample Channel"}, VerifiedAt: now}
	if err := s.Store.Create(context.Background(), id, "", value, now); err != nil {
		t.Fatal(err)
	}
	return id, confirmation
}

func concurrently(t *testing.T, fn func() error) int {
	t.Helper()
	var wg sync.WaitGroup
	var successes atomic.Int32
	start := make(chan struct{})
	for range 8 {
		wg.Go(func() {
			<-start
			if fn() == nil {
				successes.Add(1)
			}
		})
	}
	close(start)
	wg.Wait()
	return int(successes.Load())
}

func TestAuthAtomicCallbackAndConfirm(t *testing.T) {
	s, store, provider, minter, now := authTestService(t)
	ctx := context.Background()
	// 他パッケージと共有するEmulatorでも、未登録のチャンネルで検証する。
	channelRef, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	uid := "UC" + channelRef[:22]
	provider.channels[0].ID = uid
	user := store.Client.Collection("users").Doc(uid)
	if _, err := user.Get(ctx); status.Code(err) != codes.NotFound {
		t.Fatalf("expected an unregistered test channel before login, got: %v", err)
	}
	id, _ := seedTransaction(t, s, "pending", now)
	t.Cleanup(func() {
		for _, ref := range []*firestore.DocumentRef{
			store.Client.Collection("oauth-transactions").Doc(id),
			store.Client.Collection("web-accounts").Doc(uid),
			user,
		} {
			if _, err := ref.Delete(ctx); err != nil {
				t.Errorf("clean up auth fixture: %v", err)
			}
		}
	})
	if got := concurrently(t, func() error { return s.Callback(ctx, id, "synthetic-state", "synthetic-code", false) }); got != 1 {
		t.Fatalf("successful callback count=%d", got)
	}
	if provider.calls.Load() != 1 {
		t.Fatal("code exchange repeated after transaction retry/replay")
	}
	channel, err := s.Channel(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got := concurrently(t, func() error { _, err := s.Confirm(ctx, id, channel.ConfirmationRef); return err }); got != 1 {
		t.Fatalf("successful confirm count=%d", got)
	}
	if minter.calls.Load() != 1 {
		t.Fatal("token minted more than once")
	}
	account, err := store.ReadAccount(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if account.PrivacyPolicyVersion != s.Policy.Privacy || account.TermsVersion != s.Policy.Terms {
		t.Fatal("consent was not transferred")
	}
	if _, err := user.Get(ctx); err == nil {
		t.Fatal("web login created a Study Space user")
	} else if status.Code(err) != codes.NotFound {
		t.Fatalf("read Study Space user after web login: %v", err)
	}
}

func TestAuthExpiredAndCrossTabConfirm(t *testing.T) {
	s, _, _, minter, now := authTestService(t)
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	if _, err := s.Confirm(context.Background(), id, "old-tab-reference"); errorCode(err) != "OAUTH_TRANSACTION_CHANGED" {
		t.Fatal("old tab confirmed a different transaction")
	}
	s.Now = func() time.Time { return now.Add(10 * time.Minute) }
	if _, err := s.Confirm(context.Background(), id, confirmation); errorCode(err) != "OAUTH_TRANSACTION_EXPIRED" {
		t.Fatal("expiry boundary was not enforced")
	}
	if minter.calls.Load() != 0 {
		t.Fatal("invalid transaction minted token")
	}
}

func TestAuthMintFailureCannotRetry(t *testing.T) {
	s, _, _, minter, now := authTestService(t)
	minter.err = errors.New("synthetic upstream detail")
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	if _, err := s.Confirm(context.Background(), id, confirmation); errorCode(err) != "TEMPORARY_UNAVAILABLE" {
		t.Fatal("unsafe mint error")
	}
	if _, err := s.Confirm(context.Background(), id, confirmation); errorCode(err) != "OAUTH_TRANSACTION_CONSUMED" {
		t.Fatal("failed mint reopened consumed transaction")
	}
	if minter.calls.Load() != 1 {
		t.Fatal("failed mint was retried")
	}
}

func TestAuthConsumeRollsBackWhenAccountDecodeFails(t *testing.T) {
	s, store, _, minter, now := authTestService(t)
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	ref := store.Client.Collection("web-accounts").Doc("UCsynthetic0000000000001")
	if _, err := ref.Set(context.Background(), map[string]any{"firstWebLoginAt": "invalid-timestamp"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(context.Background(), id, confirmation); err == nil {
		t.Fatal("corrupt WebAccount accepted")
	}
	if _, err := s.Channel(context.Background(), id); err != nil {
		t.Fatal("failed transaction did not rollback consume")
	}
	if minter.calls.Load() != 0 {
		t.Fatal("mint before transaction commit")
	}
	if _, err := ref.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAuthSessionCompletionPreservesFirstLogin(t *testing.T) {
	s, store, _, _, now := authTestService(t)
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	if _, err := s.Confirm(context.Background(), id, confirmation); err != nil {
		t.Fatal(err)
	}
	uid := "UCsynthetic0000000000001"
	// 独立テストでこの合成uidを共有する場合がある。
	// このテストの初回完了を確定的に検証するため、初回ログインのフィールドだけを消去する。
	if _, err := store.Client.Collection("web-accounts").Doc(uid).Update(context.Background(), []firestore.Update{{Path: "firstWebLoginAt", Value: firestore.Delete}}); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteSession(context.Background(), uid, "absent", s.Policy, now); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteSession(context.Background(), uid, "absent", s.Policy, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	account, err := store.ReadAccount(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	if account.FirstWebLoginAt == nil || !account.FirstWebLoginAt.Equal(now) {
		t.Fatal("first login overwritten")
	}
	if err := store.CompleteSession(context.Background(), uid, "absent", Policy{Privacy: "changed", Terms: s.Policy.Terms}, now); errorCode(err) != "PRIVACY_RECONSENT_REQUIRED" {
		t.Fatal("outdated consent bypassed")
	}
}

func TestAuthChannelAmbiguityAndSuperseding(t *testing.T) {
	s, _, provider, minter, now := authTestService(t)
	provider.channels = append(provider.channels, Channel{ID: "UCsynthetic0000000000002", DisplayName: "Other Sample"})
	id, _ := seedTransaction(t, s, "pending", now)
	if err := s.Callback(context.Background(), id, "synthetic-state", "synthetic-code", false); errorCode(err) != "CHANNEL_AMBIGUOUS" {
		t.Fatal("ambiguous channel selected")
	}
	if minter.calls.Load() != 0 {
		t.Fatal("ambiguous channel minted token")
	}
	old, confirmation := seedTransaction(t, s, "channel_verified", now)
	_, next, err := s.Start(context.Background(), old, StartRequest{PrivacyPolicyVersion: s.Policy.Privacy, TermsVersion: s.Policy.Terms, PrivacyAccepted: true, TermsAccepted: true})
	if err != nil {
		t.Fatal(err)
	}
	if !validOpaque(next) {
		t.Fatal("new cookie is not opaque")
	}
	if _, err := s.Confirm(context.Background(), old, confirmation); err == nil {
		t.Fatal("superseded transaction remained usable")
	}
}

// ConfirmResultは読み取り済みの値をConsumeへ渡さず、消費トランザクション内で読み直した文書だけで一度限りの消費を判定する。
func TestConfirmResultRejectsTransactionChangedAfterPurposeRead(t *testing.T) {
	uid := "UCsynthetic0000000000001"
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, s *AuthService, store *FirestoreAuthStore, id, confirmation string, now time.Time)
		code   string
	}{
		{"confirmation replaced", func(t *testing.T, _ *AuthService, store *FirestoreAuthStore, id, _ string, _ time.Time) {
			replacement, err := opaque()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Client.Collection("oauth-transactions").Doc(id).Update(context.Background(), []firestore.Update{{Path: "confirmationRef", Value: replacement}}); err != nil {
				t.Fatal(err)
			}
		}, "OAUTH_TRANSACTION_CHANGED"},
		{"superseded by new start", func(t *testing.T, s *AuthService, _ *FirestoreAuthStore, id, _ string, _ time.Time) {
			if _, _, err := s.Start(context.Background(), id, StartRequest{PrivacyPolicyVersion: s.Policy.Privacy, TermsVersion: s.Policy.Terms, PrivacyAccepted: true, TermsAccepted: true}); err != nil {
				t.Fatal(err)
			}
		}, "OAUTH_TRANSACTION_PENDING"},
		{"consumed by another request", func(t *testing.T, s *AuthService, store *FirestoreAuthStore, id, confirmation string, now time.Time) {
			if _, err := store.Consume(context.Background(), id, confirmation, "absent", s.Policy, now); err != nil {
				t.Fatal(err)
			}
		}, "OAUTH_TRANSACTION_CONSUMED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, store, _, minter, now := authTestService(t)
			account := store.Client.Collection("web-accounts").Doc(uid)
			if _, err := account.Delete(context.Background()); err != nil {
				t.Fatal(err)
			}
			id, confirmation := seedTransaction(t, s, "channel_verified", now)
			s.Store = &consumeHookStore{AuthStore: store, before: func() { tc.change(t, s, store, id, confirmation, now) }}
			response, err := s.ConfirmResult(context.Background(), id, confirmation)
			if errorCode(err) != tc.code || response.CustomToken != "" {
				t.Fatalf("code=%s token=%q", errorCode(err), response.CustomToken)
			}
			if minter.calls.Load() != 0 {
				t.Fatal("changed transaction minted token")
			}
			if tc.code == "OAUTH_TRANSACTION_CONSUMED" {
				return
			}
			if _, err := account.Get(context.Background()); status.Code(err) != codes.NotFound {
				t.Fatalf("rejected consume wrote WebAccount: %v", err)
			}
		})
	}
}

func TestConfirmResultConsumesOnceAcrossConcurrentRequests(t *testing.T) {
	s, _, _, minter, now := authTestService(t)
	id, confirmation := seedTransaction(t, s, "channel_verified", now)
	if got := concurrently(t, func() error { _, err := s.ConfirmResult(context.Background(), id, confirmation); return err }); got != 1 {
		t.Fatalf("successful confirm count=%d", got)
	}
	if minter.calls.Load() != 1 {
		t.Fatal("token minted more than once")
	}
}
