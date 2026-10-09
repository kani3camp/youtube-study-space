//go:build integration

package mypage

import (
	"context"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPrivacyIntakeReceiptIdempotencyReplyAndIsolation(t *testing.T) {
	_, authStore, _, _, now := authTestService(t)
	store := &FirestorePrivacyIntake{Client: authStore.Client, Environment: "development", IntakeSecret: []byte("synthetic-test-only-key-with-thirty-two-bytes")}
	ctx := context.Background()
	key := "synthetic-submission-key-0001"
	var receipts [2]PrivacyIntakeReceipt
	var failures [2]error
	var wait sync.WaitGroup
	for i := range receipts {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			receipts[i], failures[i] = store.Create(ctx, "UCsynthetic-intake", key, SupportDelete, "Please remove my saved data", now)
		}(i)
	}
	wait.Wait()
	for _, err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := receipts[0]
	if first.RequestRef != receipts[1].RequestRef || first.Challenge != receipts[1].Challenge || first.AcceptedAt != receipts[1].AcceptedAt || first.DeleteBy == nil || !first.DeleteBy.Equal(now.Add(7*24*time.Hour)) {
		t.Fatal("concurrent retry changed receipt, challenge or deadline")
	}
	later, err := store.Create(ctx, "UCsynthetic-intake", key, SupportDelete, "Please remove my saved data", now.Add(time.Hour))
	if err != nil || later.RequestRef != first.RequestRef || !later.AcceptedAt.Equal(first.AcceptedAt) {
		t.Fatal("ACK-loss retry changed receipt")
	}
	if _, err := store.Create(ctx, "UCsynthetic-intake", key, SupportDelete, "changed", now); errorCode(err) != "INTAKE_KEY_CONFLICT" {
		t.Fatal("changed body reused key")
	}
	if _, err := store.Create(ctx, "UCsynthetic-intake", key, SupportDisclosure, "Please remove my saved data", now); errorCode(err) != "INTAKE_KEY_CONFLICT" {
		t.Fatal("changed purpose reused key")
	}
	distinct, err := store.Create(ctx, "UCsynthetic-intake", "synthetic-submission-key-0002", SupportDelete, "Please remove my saved data", now)
	if err != nil || distinct.RequestRef == first.RequestRef {
		t.Fatal("separate claim collapsed")
	}
	if _, err := store.Create(ctx, "UCsynthetic-intake", "synthetic-submission-key-0003", SupportDisclosure, "", now); err != nil {
		t.Fatal("purpose-only request was refused")
	}
	if _, err := store.Status(ctx, "UCsynthetic-intake", first.RequestRef); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("preproof status exposed")
	}
	if _, err := store.Status(ctx, "UCother", first.RequestRef); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("guessed reference exposed")
	}
	if err := store.SetReply(ctx, first.RequestRef, "Synthetic response", now); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("preproof reply accepted")
	}
	binding, err := (&FirestoreSupportStore{Client: authStore.Client, Environment: "development"}).Resolve(ctx, first.Challenge, now)
	if err != nil || binding.RequestRef != first.RequestRef {
		t.Fatal("receipt has no resolvable challenge")
	}
	// Synthetic proof state: full one-time OAuth transition has its own integration tests.
	doc := store.Client.Collection("support-requests").Doc(first.RequestRef)
	_, err = doc.Update(ctx, []firestore.Update{{Path: "status", Value: "verified"}, {Path: "verifiedAt", Value: now}, {Path: "proofRef", Value: distinct.RequestRef}, {Path: "challengeHash", Value: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetReply(ctx, first.RequestRef, "Synthetic response", now); err != nil {
		t.Fatal(err)
	}
	result, err := store.Status(ctx, "UCsynthetic-intake", first.RequestRef)
	if err != nil || result.Reply != "Synthetic response" || !result.AcceptedAt.Equal(now) {
		t.Fatal("verified reply or receipt missing")
	}
	other := &FirestorePrivacyIntake{Client: store.Client, Environment: "production", IntakeSecret: store.IntakeSecret}
	if _, err := other.Status(ctx, "UCsynthetic-intake", first.RequestRef); errorCode(err) != "SUPPORT_CHALLENGE_INVALID" {
		t.Fatal("cross environment reply exposed")
	}
	if _, err := store.Create(ctx, "UCsynthetic-intake", key, SupportDelete, "Please remove my saved data", now); err != nil {
		t.Fatal(err)
	}
}

func TestPrivacyIntakeFailureRollsBackReceiptAndIndexes(t *testing.T) {
	_, authStore, _, _, now := authTestService(t)
	store := &FirestorePrivacyIntake{Client: authStore.Client, Environment: "development", IntakeSecret: []byte("synthetic-test-only-key-with-thirty-two-bytes")}
	ctx := context.Background()
	uid, key := "UCsynthetic-rollback", "synthetic-rollback-key-0001"
	ref := store.keyed("ref", uid, key)
	requestID := store.keyed("request-id", uid, key)
	challengeHash := digest(store.keyed("challenge", uid, key))
	_, err := store.Client.Collection("support-challenges").Doc(challengeHash).Create(ctx, supportChallengeIndex{RequestRef: digest("occupied"), Environment: "development"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, uid, key, SupportDelete, "", now); err == nil {
		t.Fatal("collision accepted")
	}
	for _, doc := range []*firestore.DocumentRef{
		store.Client.Collection("support-requests").Doc(ref),
		store.Client.Collection("support-request-ids").Doc(digest("development:" + requestID)),
	} {
		if _, err := doc.Get(ctx); status.Code(err) != codes.NotFound {
			t.Fatal("failed transaction left partial intake data")
		}
	}
}
