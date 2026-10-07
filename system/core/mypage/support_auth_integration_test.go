//go:build integration

package mypage

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func supportAuthFixture(t *testing.T) (*AuthService, *FirestoreAuthStore, *fakeMinter, string, string, string, time.Time) {
	t.Helper()
	s, store, provider, minter, now := authTestService(t)
	requestID, err := opaque()
	if err != nil {
		t.Fatal(err)
	}
	provider.channels[0].ID = "UC" + requestID[:22]
	s.Support = &FirestoreSupportStore{Client: store.Client, Environment: "development"}
	ref, challenge, err := s.Support.Create(context.Background(), requestID, provider.channels[0].ID, SupportDelete, now, now)
	if err != nil {
		t.Fatal(err)
	}
	start, id, err := s.Start(context.Background(), "", StartRequest{PrivacyPolicyVersion: s.Policy.Privacy, PrivacyAccepted: true, TermsVersion: s.Policy.Terms, TermsAccepted: true, SupportChallenge: &challenge})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(start.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Callback(context.Background(), id, parsed.Query().Get("state"), "synthetic-code", false); err != nil {
		t.Fatal(err)
	}
	return s, store, minter, ref, challenge, id, now
}

func TestSupportConfirmThroughHTTPNeverMintsOrCreatesWebAccount(t *testing.T) {
	s, store, minter, requestRef, challenge, id, now := supportAuthFixture(t)
	ctx := context.Background()
	value, err := store.ReadVerified(ctx, id, now)
	if err != nil {
		t.Fatal(err)
	}
	activateRestriction(t, store, value.Channel.ID, true, now)
	channel, err := s.Channel(ctx, id)
	if err != nil || channel.Purpose != "support" || channel.SupportPurpose != SupportDelete {
		t.Fatal("support purpose not bound to confirmation UI")
	}
	h := &HTTPHandler{Auth: s, Verifier: &boundaryVerifier{}, PublicOrigin: "https://mypage.example.test"}
	r := httptest.NewRequest("POST", "/api/auth/youtube/confirm", strings.NewReader(`{"confirmationRef":"`+channel.ConfirmationRef+`"}`))
	r.Host = "mypage.example.test"
	r.Header.Set("Origin", h.PublicOrigin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Firebase-AppCheck", "synthetic-app")
	r.Header.Set("Cookie", "__session="+id)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("support confirm status=%d", w.Code)
	}
	var response ConfirmResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Purpose != "support" || !validOpaque(response.RequestRef) || response.CustomToken != "" || minter.calls.Load() != 0 || strings.Contains(w.Body.String(), "customToken") {
		t.Fatal("support proof was used as normal login")
	}
	doc, err := store.Client.Collection("support-requests").Doc(requestRef).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var verified SupportRequest
	if err := doc.DataTo(&verified); err != nil {
		t.Fatal(err)
	}
	if verified.Status != "verified" || verified.ProofRef != response.RequestRef || verified.VerifiedAt == nil || !verified.VerifiedAt.Equal(now) {
		t.Fatal("proof not recorded once against original request")
	}
	for _, collection := range []string{"web-accounts", "users"} {
		if _, err := store.Client.Collection(collection).Doc(verified.TargetChannel).Get(ctx); status.Code(err) != codes.NotFound {
			t.Fatal("support proof created normal user/account")
		}
	}
	if _, err := s.Support.Resolve(ctx, challenge, now); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("support challenge remained reusable")
	}
	if _, err := s.ConfirmResult(ctx, id, channel.ConfirmationRef); err == nil {
		t.Fatal("support confirm replay accepted")
	}
	if minter.calls.Load() != 0 {
		t.Fatal("support replay minted token")
	}
}

func TestSupportConfirmIsAtomicAcrossConcurrentTransactions(t *testing.T) {
	s, store, minter, requestRef, _, id, now := supportAuthFixture(t)
	channel, err := s.Channel(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := concurrently(t, func() error { _, err := s.ConfirmResult(context.Background(), id, channel.ConfirmationRef); return err }); got != 1 {
		t.Fatalf("support proof winners=%d", got)
	}
	if minter.calls.Load() != 0 {
		t.Fatal("support minted Firebase custom token")
	}
	doc, err := store.Client.Collection("support-requests").Doc(requestRef).Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var value SupportRequest
	if err := doc.DataTo(&value); err != nil {
		t.Fatal(err)
	}
	if value.Status != "verified" || value.VerifiedAt == nil || !value.VerifiedAt.Equal(now) {
		t.Fatal("atomic support proof lost")
	}
}

func TestSupportMismatchAndReissueRollBackOAuthConsume(t *testing.T) {
	for _, scenario := range []string{"environment", "purpose", "request", "channel", "reissue"} {
		t.Run(scenario, func(t *testing.T) {
			s, store, minter, requestRef, _, id, now := supportAuthFixture(t)
			channel, err := s.Channel(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			ref := store.Client.Collection("oauth-transactions").Doc(id)
			switch scenario {
			case "environment":
				s.Support.Environment = "production"
			case "purpose":
				if _, err := ref.Update(context.Background(), []firestore.Update{{Path: "support.purpose", Value: SupportRevoke}}); err != nil {
					t.Fatal(err)
				}
			case "request":
				if _, err := ref.Update(context.Background(), []firestore.Update{{Path: "support.requestId", Value: "SYNTHETIC-OTHER"}}); err != nil {
					t.Fatal(err)
				}
			case "channel":
				if _, err := ref.Update(context.Background(), []firestore.Update{{Path: "channel.channelId", Value: "UCsynthetic-other"}}); err != nil {
					t.Fatal(err)
				}
			case "reissue":
				if _, err := s.Support.Reissue(context.Background(), requestRef, now); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.ConfirmResult(context.Background(), id, channel.ConfirmationRef); err == nil {
				t.Fatal("mismatched proof accepted")
			}
			if minter.calls.Load() != 0 {
				t.Fatal("mismatched proof minted token")
			}
			if _, err := store.ReadVerified(context.Background(), id, now); err != nil {
				t.Fatal("failed support transaction did not roll back OAuth consume")
			}
			doc, err := store.Client.Collection("support-requests").Doc(requestRef).Get(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var value SupportRequest
			if err := doc.DataTo(&value); err != nil {
				t.Fatal(err)
			}
			if value.Status != "awaiting_proof" || value.VerifiedAt != nil {
				t.Fatal("failed support proof partially modified record")
			}
		})
	}
}

func TestSupportTransactionCannotUseNormalConfirm(t *testing.T) {
	s, _, minter, _, _, id, _ := supportAuthFixture(t)
	channel, err := s.Channel(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Confirm(context.Background(), id, channel.ConfirmationRef); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("support transaction crossed normal mint boundary")
	}
	if minter.calls.Load() != 0 {
		t.Fatal("support transaction minted normal token")
	}
}

func TestSupportTwoFreshOAuthTransactionsCannotProveTheSameRequestTwice(t *testing.T) {
	s, _, minter, _, challenge, firstID, _ := supportAuthFixture(t)
	ctx := context.Background()
	firstChannel, err := s.Channel(ctx, firstID)
	if err != nil {
		t.Fatal(err)
	}
	start, secondID, err := s.Start(ctx, "", StartRequest{PrivacyPolicyVersion: s.Policy.Privacy, PrivacyAccepted: true, TermsVersion: s.Policy.Terms, TermsAccepted: true, SupportChallenge: &challenge})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(start.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Callback(ctx, secondID, parsed.Query().Get("state"), "synthetic-second-fresh-code", false); err != nil {
		t.Fatal(err)
	}
	secondChannel, err := s.Channel(ctx, secondID)
	if err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() { _, err := s.ConfirmResult(ctx, firstID, firstChannel.ConfirmationRef); firstDone <- err }()
	go func() { _, err := s.ConfirmResult(ctx, secondID, secondChannel.ConfirmationRef); secondDone <- err }()
	winners := 0
	for _, err := range []error{<-firstDone, <-secondDone} {
		if err == nil {
			winners++
		}
	}
	if winners != 1 || minter.calls.Load() != 0 {
		t.Fatal("one support challenge proved through multiple OAuth transactions")
	}
}
